package operator

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
)

type Controller interface {
	Up(context.Context, UpRequest, EventSink) (OperationResult, error)
	Down(context.Context, DownRequest, EventSink) (OperationResult, error)
	Restart(context.Context, RestartRequest, EventSink) (OperationResult, error)
	Snapshot(context.Context, SnapshotRequest) (Snapshot, error)
	Logs() runlog.Reader
	Doctor(context.Context, DoctorRequest) (DoctorReport, error)
}

type serviceSupervisor interface {
	StartPreparedService(context.Context, engine.ServiceSpec, string) (state.ServiceRecord, error)
	CompleteHealth(context.Context, engine.ServiceSpec, string) error
	StopPreparedService(context.Context, string) error
}

type SupervisorFactory func(repoRoot string, timeout time.Duration) serviceSupervisor

type ControllerOptions struct {
	Planner           Planner
	SupervisorFactory SupervisorFactory
	LogReader         runlog.Reader
	Now               func() time.Time
	NewOperationID    func() (string, error)
	WrapperExe        string
}

type controller struct {
	planner           Planner
	supervisorFactory SupervisorFactory
	logReader         runlog.Reader
	now               func() time.Time
	newOperationID    func() (string, error)
}

var _ Controller = (*controller)(nil)

func NewController(options ControllerOptions) (Controller, error) {
	planner := options.Planner
	if planner == nil {
		planner = PipelinePlanner{}
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newOperationID := options.NewOperationID
	if newOperationID == nil {
		newOperationID = runstate.NewRunID
	}
	factory := options.SupervisorFactory
	if factory == nil {
		wrapperExe := options.WrapperExe
		if wrapperExe == "" {
			var err error
			wrapperExe, err = os.Executable()
			if err != nil {
				return nil, errors.Wrap(err, "resolve devctl wrapper executable")
			}
		}
		factory = func(repoRoot string, timeout time.Duration) serviceSupervisor {
			return supervise.New(supervise.Options{
				RepoRoot:        repoRoot,
				WrapperExe:      wrapperExe,
				ReadyTimeout:    timeout,
				ShutdownTimeout: timeout,
			})
		}
	}
	return &controller{
		planner:           planner,
		supervisorFactory: factory,
		logReader:         options.LogReader,
		now:               now,
		newOperationID:    newOperationID,
	}, nil
}

func (c *controller) Logs() runlog.Reader {
	return c.logReader
}

func (c *controller) Up(
	ctx context.Context,
	request UpRequest,
	sink EventSink,
) (OperationResult, error) {
	result, operationErr := c.newOperation("up")
	if operationErr != nil {
		return OperationResult{}, operationErr
	}
	sink = normalizeSink(sink)
	c.emit(ctx, sink, result.OperationID, EventOperationStarted, "", "started", "starting environment", nil)

	if err := ctx.Err(); err != nil {
		return c.finishCanceled(result, err)
	}
	store, err := runstate.NewStore(request.RepoRoot)
	if err != nil {
		return c.finishFailed(result, CodeUsage, "invalid repository root", err)
	}
	request.RepoRoot = store.RepoRoot()

	planResult, err := c.planner.Plan(ctx, request)
	if err != nil {
		return c.finishFailed(result, CodeConfigInvalid, "could not resolve launch plan", err)
	}
	services, selectionErr := selectPlannedServices(planResult.Plan.Services, request.Select)
	if selectionErr != nil {
		return c.finishWithOperatorError(result, selectionErr)
	}
	if request.Policy.DryRun {
		for _, service := range services {
			result.Outcomes = append(result.Outcomes, ServiceOutcome{
				Service: service.Name,
				After:   runstate.RunPlanned,
				Changed: false,
			})
		}
		result.Status = "succeeded"
		result.FinishedAt = c.now()
		c.emit(ctx, sink, result.OperationID, EventOperationFinished, "", result.Status, "dry-run plan complete", nil)
		return result, nil
	}

	timeout := normalizedTimeout(request.Policy.Timeout)
	locker, err := runstate.NewLocker(request.RepoRoot)
	if err != nil {
		return c.finishFailed(result, CodeStateCorrupt, "create repository lock", err)
	}
	lockErr := locker.WithExclusive(ctx, runstate.LockMetadata{
		OperationID: result.OperationID,
		Command:     []string{"devctl", "up"},
	}, func(lockContext context.Context) error {
		return c.upLocked(lockContext, store, planResult.ProfileName, services, timeout, sink, &result)
	})
	if lockErr != nil {
		if stderrors.Is(lockErr, runstate.ErrOperationBusy) {
			return c.finishFailed(result, CodeOperationBusy, "another lifecycle operation holds the repository lock", lockErr)
		}
		if stderrors.Is(lockErr, context.Canceled) || stderrors.Is(lockErr, context.DeadlineExceeded) {
			return c.finishCanceled(result, lockErr)
		}
		if operatorErr := asOperatorError(lockErr); operatorErr != nil {
			return c.finishWithOperatorError(result, operatorErr)
		}
		return c.finishFailed(result, CodePartialFailure, "up operation failed", lockErr)
	}
	return c.finishFromOutcomes(ctx, sink, result)
}

func (c *controller) upLocked(
	ctx context.Context,
	store *runstate.Store,
	profile string,
	services []engine.ServiceSpec,
	timeout time.Duration,
	sink EventSink,
	result *OperationResult,
) error {
	if _, err := reconcile(ctx, store); err != nil {
		return newError(CodeStateCorrupt, "reconcile environment before start", err)
	}
	current, err := loadEnvironmentOptional(ctx, store)
	if err != nil {
		return newError(CodeStateCorrupt, "load environment state", err)
	}
	if err := rejectRunningSelections(ctx, store, current, services); err != nil {
		return err
	}

	type preparedService struct {
		spec  engine.ServiceSpec
		runID string
	}
	prepared := make([]preparedService, 0, len(services))
	for _, service := range services {
		runID, err := runstate.NewRunID()
		if err != nil {
			return newError(CodeStateCorrupt, "generate service run ID", err)
		}
		run := runstate.RunRecord{
			RunID:   runID,
			Service: service.Name,
			Phase:   runstate.RunPlanned,
			Spec: runstate.ServiceSpecRecord{
				Name:        service.Name,
				Command:     append([]string{}, service.Command...),
				Cwd:         resolveServiceCwd(store.RepoRoot(), service.Cwd),
				Environment: state.SanitizeEnv(service.Env),
				Health:      healthRecord(service.Health),
			},
		}
		if err := store.CreateRun(ctx, run); err != nil {
			return newError(CodeStateCorrupt, "create planned service run", err)
		}
		prepared = append(prepared, preparedService{spec: service, runID: runID})
		c.emit(ctx, sink, result.OperationID, EventServicePlanned, service.Name, "planned", "service run planned", nil)
	}

	if current == nil {
		stateDocument := runstate.EnvironmentState{
			Profile:  profile,
			Services: map[string]runstate.ServiceSlot{},
		}
		for _, service := range prepared {
			stateDocument.Services[service.spec.Name] = runstate.ServiceSlot{
				Name:         service.spec.Name,
				CurrentRunID: service.runID,
				Desired:      runstate.DesiredRunning,
			}
		}
		if err := store.CreateEnvironment(ctx, stateDocument); err != nil {
			return newError(CodeStateCorrupt, "create environment index", err)
		}
	} else {
		expectedRevision := current.Revision
		if err := store.Update(ctx, expectedRevision, func(stateDocument *runstate.EnvironmentState) error {
			stateDocument.Profile = profile
			for _, service := range prepared {
				slot := stateDocument.Services[service.spec.Name]
				slot.Name = service.spec.Name
				if slot.CurrentRunID != "" {
					slot.LastRunID = slot.CurrentRunID
				}
				slot.CurrentRunID = service.runID
				slot.Desired = runstate.DesiredRunning
				stateDocument.Services[service.spec.Name] = slot
			}
			return nil
		}); err != nil {
			return newError(CodeStateCorrupt, "index planned service runs", err)
		}
	}

	supervisor := c.supervisorFactory(store.RepoRoot(), timeout)
	startErrors := make(map[string]error, len(prepared))
	for _, service := range prepared {
		c.emit(ctx, sink, result.OperationID, EventServiceStarting, service.spec.Name, "starting", "starting service wrapper", nil)
		_, startErr := supervisor.StartPreparedService(ctx, service.spec, service.runID)
		startErrors[service.runID] = startErr
		if startErr != nil && ctx.Err() != nil {
			return ctx.Err()
		}
	}

	for _, service := range prepared {
		outcome := ServiceOutcome{
			Service: service.spec.Name,
			RunID:   service.runID,
			Before:  runstate.RunPlanned,
			Changed: true,
		}
		startErr := startErrors[service.runID]
		errorCode := classifyStartError(startErr)
		errorMessage := "service start failed"
		if startErr == nil {
			healthErr := supervisor.CompleteHealth(ctx, service.spec, service.runID)
			if healthErr != nil {
				errorCode = CodeHealthTimeout
				errorMessage = "service health check failed"
				c.emit(ctx, sink, result.OperationID, EventServiceUnhealthy, service.spec.Name, "unhealthy", errorMessage, serviceError(errorCode, errorMessage, service.spec.Name, service.runID, healthErr))
				cleanupContext, cancel := context.WithTimeout(context.Background(), timeout)
				cleanupErr := supervisor.StopPreparedService(cleanupContext, service.runID)
				cancel()
				if cleanupErr == nil {
					persistErr := store.UpdateRun(context.Background(), service.runID, func(record *runstate.RunRecord) error {
						record.Phase = runstate.RunFailed
						return nil
					})
					if persistErr != nil {
						cleanupErr = persistErr
					}
				}
				startErr = stderrors.Join(healthErr, cleanupErr)
			}
		}
		run, loadErr := store.LoadRun(context.Background(), service.runID)
		if loadErr == nil {
			outcome.After = run.Phase
		}
		if startErr != nil {
			outcome.Error = serviceError(errorCode, errorMessage, service.spec.Name, service.runID, startErr)
			result.Outcomes = append(result.Outcomes, outcome)
			c.emit(ctx, sink, result.OperationID, EventServiceFailed, service.spec.Name, "failed", "service start failed", outcome.Error)
			continue
		}
		result.Outcomes = append(result.Outcomes, outcome)
		c.emit(ctx, sink, result.OperationID, EventServiceReady, service.spec.Name, "ready", "service is ready", nil)
	}
	return nil
}

func (c *controller) Down(
	ctx context.Context,
	request DownRequest,
	sink EventSink,
) (OperationResult, error) {
	result, operationErr := c.newOperation("down")
	if operationErr != nil {
		return OperationResult{}, operationErr
	}
	sink = normalizeSink(sink)
	c.emit(ctx, sink, result.OperationID, EventOperationStarted, "", "started", "stopping environment", nil)
	store, err := runstate.NewStore(request.RepoRoot)
	if err != nil {
		return c.finishFailed(result, CodeUsage, "invalid repository root", err)
	}
	request.RepoRoot = store.RepoRoot()
	locker, err := runstate.NewLocker(request.RepoRoot)
	if err != nil {
		return c.finishFailed(result, CodeStateCorrupt, "create repository lock", err)
	}
	lockErr := locker.WithExclusive(ctx, runstate.LockMetadata{
		OperationID: result.OperationID,
		Command:     []string{"devctl", "down"},
	}, func(lockContext context.Context) error {
		return c.downLocked(lockContext, store, request.Select, 30*time.Second, sink, &result)
	})
	if lockErr != nil {
		if stderrors.Is(lockErr, runstate.ErrOperationBusy) {
			return c.finishFailed(result, CodeOperationBusy, "another lifecycle operation holds the repository lock", lockErr)
		}
		if operatorErr := asOperatorError(lockErr); operatorErr != nil {
			return c.finishWithOperatorError(result, operatorErr)
		}
		return c.finishFailed(result, CodePartialFailure, "down operation failed", lockErr)
	}
	return c.finishFromOutcomes(ctx, sink, result)
}

func (c *controller) downLocked(
	ctx context.Context,
	store *runstate.Store,
	selection Selection,
	timeout time.Duration,
	sink EventSink,
	result *OperationResult,
) error {
	if _, err := reconcile(ctx, store); err != nil {
		return newError(CodeStateCorrupt, "reconcile environment before stop", err)
	}
	environment, err := loadEnvironmentOptional(ctx, store)
	if err != nil {
		return newError(CodeStateCorrupt, "load environment state", err)
	}
	if environment == nil {
		return nil
	}
	names, selectionErr := selectStateServices(environment, selection)
	if selectionErr != nil {
		return selectionErr
	}
	supervisor := c.supervisorFactory(store.RepoRoot(), timeout)
	for _, name := range names {
		latest, err := store.LoadEnvironment(ctx)
		if err != nil {
			return newError(CodeStateCorrupt, "reload environment state", err)
		}
		slot := latest.Services[name]
		outcome := ServiceOutcome{
			Service: name,
			RunID:   slot.CurrentRunID,
			Changed: slot.Desired != runstate.DesiredStopped || slot.CurrentRunID != "",
		}
		if slot.CurrentRunID == "" {
			if err := store.Update(ctx, latest.Revision, func(stateDocument *runstate.EnvironmentState) error {
				currentSlot := stateDocument.Services[name]
				currentSlot.Desired = runstate.DesiredStopped
				stateDocument.Services[name] = currentSlot
				return nil
			}); err != nil {
				return newError(CodeStateCorrupt, "persist stopped desired state", err)
			}
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		run, err := store.LoadRun(ctx, slot.CurrentRunID)
		if err != nil {
			outcome.Error = serviceError(CodeStateCorrupt, "load current run", name, slot.CurrentRunID, err)
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		outcome.Before = run.Phase
		c.emit(ctx, sink, result.OperationID, EventServiceStopping, name, "stopping", "stopping service", nil)

		stopErr := error(nil)
		if run.Wrapper == nil && (run.Phase == runstate.RunFailed || run.Phase == runstate.RunExited) {
			stopErr = nil
		} else {
			stopErr = supervisor.StopPreparedService(ctx, slot.CurrentRunID)
		}
		updatedRun, loadErr := store.LoadRun(context.Background(), slot.CurrentRunID)
		if loadErr == nil {
			outcome.After = updatedRun.Phase
			outcome.Exit = updatedRun.Exit
		}
		if stopErr != nil {
			outcome.Error = serviceError(CodeStopFailed, "service stop failed", name, slot.CurrentRunID, stopErr)
			result.Outcomes = append(result.Outcomes, outcome)
			c.emit(ctx, sink, result.OperationID, EventServiceUnknown, name, "unknown", "service stop could not prove termination", outcome.Error)
			continue
		}
		latest, err = store.LoadEnvironment(ctx)
		if err != nil {
			return newError(CodeStateCorrupt, "reload environment after stop", err)
		}
		if err := store.Update(ctx, latest.Revision, func(stateDocument *runstate.EnvironmentState) error {
			currentSlot := stateDocument.Services[name]
			currentSlot.Desired = runstate.DesiredStopped
			currentSlot.LastRunID = slot.CurrentRunID
			currentSlot.CurrentRunID = ""
			stateDocument.Services[name] = currentSlot
			return nil
		}); err != nil {
			return newError(CodeStateCorrupt, "persist stopped service slot", err)
		}
		result.Outcomes = append(result.Outcomes, outcome)
		c.emit(ctx, sink, result.OperationID, EventServiceExited, name, "exited", "service stopped", nil)
	}
	return nil
}

func (c *controller) Restart(
	ctx context.Context,
	request RestartRequest,
	sink EventSink,
) (OperationResult, error) {
	result, operationErr := c.newOperation("restart")
	if operationErr != nil {
		return OperationResult{}, operationErr
	}
	sink = normalizeSink(sink)
	c.emit(ctx, sink, result.OperationID, EventOperationStarted, "", "started", "restarting environment", nil)
	if err := ctx.Err(); err != nil {
		return c.finishCanceled(result, err)
	}

	upRequest := UpRequest(request)
	planResult, err := c.planner.Plan(ctx, upRequest)
	if err != nil {
		return c.finishFailed(result, CodeConfigInvalid, "could not resolve restart plan", err)
	}
	services, selectionErr := selectPlannedServices(planResult.Plan.Services, request.Select)
	if selectionErr != nil {
		return c.finishWithOperatorError(result, selectionErr)
	}
	if request.Policy.DryRun {
		for _, service := range services {
			result.Outcomes = append(result.Outcomes, ServiceOutcome{
				Service: service.Name,
				After:   runstate.RunPlanned,
			})
		}
		result.Status = "succeeded"
		result.FinishedAt = c.now()
		c.emit(ctx, sink, result.OperationID, EventOperationFinished, "", result.Status, "dry-run restart plan complete", nil)
		return result, nil
	}

	store, err := runstate.NewStore(request.RepoRoot)
	if err != nil {
		return c.finishFailed(result, CodeUsage, "invalid repository root", err)
	}
	locker, err := runstate.NewLocker(store.RepoRoot())
	if err != nil {
		return c.finishFailed(result, CodeStateCorrupt, "create repository lock", err)
	}
	lockErr := locker.WithExclusive(ctx, runstate.LockMetadata{
		OperationID: result.OperationID,
		Command:     []string{"devctl", "restart"},
	}, func(lockContext context.Context) error {
		timeout := normalizedTimeout(request.Policy.Timeout)
		if err := c.downLocked(lockContext, store, request.Select, timeout, sink, &result); err != nil {
			return err
		}
		for _, outcome := range result.Outcomes {
			if outcome.Error != nil {
				return newError(CodePartialFailure, "restart stopped after an unproven service termination", outcome.Error)
			}
		}
		return c.upLocked(lockContext, store, planResult.ProfileName, services, timeout, sink, &result)
	})
	if lockErr != nil {
		if stderrors.Is(lockErr, runstate.ErrOperationBusy) {
			return c.finishFailed(result, CodeOperationBusy, "another lifecycle operation holds the repository lock", lockErr)
		}
		if stderrors.Is(lockErr, context.Canceled) || stderrors.Is(lockErr, context.DeadlineExceeded) {
			return c.finishCanceled(result, lockErr)
		}
		if operatorErr := asOperatorError(lockErr); operatorErr != nil {
			return c.finishWithOperatorError(result, operatorErr)
		}
		return c.finishFailed(result, CodePartialFailure, "restart operation failed", lockErr)
	}
	return c.finishFromOutcomes(ctx, sink, result)
}

func (c *controller) Snapshot(ctx context.Context, request SnapshotRequest) (Snapshot, error) {
	store, err := runstate.NewStore(request.RepoRoot)
	if err != nil {
		return Snapshot{}, newError(CodeUsage, "invalid repository root", err)
	}
	environment, err := loadEnvironmentOptional(ctx, store)
	if err != nil {
		return Snapshot{}, newError(CodeStateCorrupt, "load environment state", err)
	}
	if environment == nil {
		return Snapshot{Exists: false, Services: []ServiceSnapshot{}}, nil
	}
	locker, err := runstate.NewLocker(store.RepoRoot())
	if err != nil {
		return Snapshot{}, newError(CodeStateCorrupt, "create snapshot reconciliation lock", err)
	}
	if err := locker.WithExclusive(ctx, runstate.LockMetadata{
		Command: []string{"devctl", "snapshot"},
	}, func(lockContext context.Context) error {
		_, reconcileErr := reconcile(lockContext, store)
		return reconcileErr
	}); err != nil {
		if stderrors.Is(err, runstate.ErrOperationBusy) {
			return Snapshot{}, newError(CodeOperationBusy, "another lifecycle operation holds the repository lock", err)
		}
		return Snapshot{}, newError(CodeStateCorrupt, "reconcile environment snapshot", err)
	}
	environment, err = store.LoadEnvironment(ctx)
	if err != nil {
		return Snapshot{}, newError(CodeStateCorrupt, "reload reconciled environment state", err)
	}
	snapshot := Snapshot{
		Exists:   true,
		Profile:  environment.Profile,
		Revision: environment.Revision,
		Services: make([]ServiceSnapshot, 0, len(environment.Services)),
	}
	names := sortedSlotNames(environment)
	for _, name := range names {
		slot := environment.Services[name]
		service := ServiceSnapshot{Service: name, Desired: slot.Desired}
		runID := slot.CurrentRunID
		if runID == "" && request.IncludeRuns {
			runID = slot.LastRunID
		}
		if runID != "" {
			run, loadErr := store.LoadRun(ctx, runID)
			if loadErr != nil {
				return Snapshot{}, newError(CodeStateCorrupt, "load snapshot run", loadErr)
			}
			service.Phase = run.Phase
			service.RunID = run.RunID
			service.Wrapper = run.Wrapper
			service.Child = run.Child
			service.Health = run.Health
			service.CreatedAt = run.CreatedAt
			service.UpdatedAt = run.UpdatedAt
			service.Exit = run.Exit
			service.LastError = run.LastError
			runDir, pathErr := store.RunDir(run.RunID)
			if pathErr == nil {
				service.StdoutPath = filepath.Join(runDir, supervise.StdoutLogName)
				service.StderrPath = filepath.Join(runDir, supervise.StderrLogName)
			}
		}
		snapshot.Services = append(snapshot.Services, service)
	}
	return snapshot, nil
}

func (c *controller) Doctor(ctx context.Context, request DoctorRequest) (DoctorReport, error) {
	return c.doctor(ctx, request)
}
