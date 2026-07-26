package operator

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
)

type staticPlanner struct {
	result PlanResult
	err    error
}

var _ Planner = staticPlanner{}

func (p staticPlanner) Plan(context.Context, UpRequest) (PlanResult, error) {
	return p.result, p.err
}

type recordingSupervisor struct {
	t               *testing.T
	repoRoot        string
	mu              sync.Mutex
	started         []string
	attempted       []string
	stopped         []string
	stopErr         error
	failStopService string
	healthErr       error
	onStart         func()
	onHealth        func()
	processes       map[string]*exec.Cmd
}

var _ serviceSupervisor = (*recordingSupervisor)(nil)

func (s *recordingSupervisor) StartPreparedService(
	ctx context.Context,
	spec engine.ServiceSpec,
	runID string,
) (state.ServiceRecord, error) {
	s.t.Helper()
	store, err := runstate.NewStore(s.repoRoot)
	if err != nil {
		return state.ServiceRecord{}, err
	}
	environment, err := store.LoadEnvironment(ctx)
	if err != nil {
		return state.ServiceRecord{}, err
	}
	if got := environment.Services[spec.Name].CurrentRunID; got != runID {
		s.t.Fatalf("wrapper start observed current run %q, want %q", got, runID)
	}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return state.ServiceRecord{}, err
	}
	if run.Phase != runstate.RunPlanned {
		s.t.Fatalf("wrapper start observed phase %q, want planned", run.Phase)
	}
	s.mu.Lock()
	s.attempted = append(s.attempted, spec.Name)
	s.mu.Unlock()
	if s.onStart != nil {
		s.onStart()
	}
	if err := ctx.Err(); err != nil {
		return state.ServiceRecord{}, err
	}
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return state.ServiceRecord{}, err
	}
	identity, err := runstate.ReadProcessIdentity(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return state.ServiceRecord{}, err
	}
	runDir, err := store.RunDir(runID)
	if err != nil {
		_ = cmd.Process.Kill()
		return state.ServiceRecord{}, err
	}
	owner := supervise.OwnerRecord{
		Version: supervise.HandshakeVersion,
		RunID:   runID, Service: spec.Name, Wrapper: *identity, WrittenAt: time.Now().UTC(),
	}
	ready := supervise.ReadyRecord{
		Version: supervise.HandshakeVersion,
		RunID:   runID, Service: spec.Name, Wrapper: *identity, Child: *identity,
		ChildPGID: identity.PID, WrittenAt: time.Now().UTC(),
	}
	if err := supervise.WriteOwnerRecord(runDir+"/"+supervise.OwnerRecordName, owner); err != nil {
		_ = cmd.Process.Kill()
		return state.ServiceRecord{}, err
	}
	if err := supervise.WriteReadyRecord(runDir+"/"+supervise.ReadyRecordName, ready); err != nil {
		_ = cmd.Process.Kill()
		return state.ServiceRecord{}, err
	}
	if err := store.UpdateRun(ctx, runID, func(run *runstate.RunRecord) error {
		run.Phase = runstate.RunStarting
		run.Wrapper = identity
		run.Child = identity
		run.ChildPGID = identity.PID
		return nil
	}); err != nil {
		return state.ServiceRecord{}, err
	}
	s.mu.Lock()
	s.started = append(s.started, spec.Name)
	if s.processes == nil {
		s.processes = map[string]*exec.Cmd{}
	}
	s.processes[runID] = cmd
	s.mu.Unlock()
	return state.ServiceRecord{Name: spec.Name}, nil
}

func (s *recordingSupervisor) CompleteHealth(ctx context.Context, _ engine.ServiceSpec, runID string) error {
	if s.onHealth != nil {
		s.onHealth()
	}
	store, err := runstate.NewStore(s.repoRoot)
	if err != nil {
		return err
	}
	if s.healthErr != nil {
		_ = store.UpdateRun(ctx, runID, func(run *runstate.RunRecord) error {
			run.Phase = runstate.RunFailed
			run.Health = &runstate.HealthResult{
				Healthy: false, CheckedAt: time.Now().UTC(), Detail: s.healthErr.Error(),
			}
			run.LastError = &runstate.ErrorRecord{Code: CodeHealthTimeout, Message: s.healthErr.Error()}
			return nil
		})
		return s.healthErr
	}
	return store.UpdateRun(ctx, runID, func(run *runstate.RunRecord) error {
		run.Phase = runstate.RunReady
		return nil
	})
}

func (s *recordingSupervisor) StopPreparedService(ctx context.Context, runID string) error {
	if s.stopErr != nil {
		return s.stopErr
	}
	store, err := runstate.NewStore(s.repoRoot)
	if err != nil {
		return err
	}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Service == s.failStopService {
		return errors.New("injected stop failure")
	}
	s.mu.Lock()
	cmd := s.processes[runID]
	delete(s.processes, runID)
	s.mu.Unlock()
	if cmd != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_ = cmd.Wait()
	}
	if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunExited
		return nil
	}); err != nil {
		return err
	}
	s.mu.Lock()
	s.stopped = append(s.stopped, run.Service)
	s.mu.Unlock()
	return nil
}

func newTestController(t *testing.T, repoRoot string, planner Planner, supervisor *recordingSupervisor) Controller {
	t.Helper()
	t.Cleanup(func() {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		for _, cmd := range supervisor.processes {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
		}
	})
	controller, err := NewController(ControllerOptions{
		Planner: planner,
		SupervisorFactory: func(string, time.Duration) serviceSupervisor {
			return supervisor
		},
		NewOperationID: func() (string, error) {
			return "018f0f65-6c1a-7abc-8def-0123456789ab", nil
		},
	})
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	return controller
}

func TestUpIndexesRunBeforeStartingWrapper(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	controller := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		ProfileName: "dev",
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{Name: "web", Command: []string{"serve"}},
		}},
	}}, supervisor)

	result, err := controller.Up(context.Background(), UpRequest{RepoRoot: repoRoot}, nil)
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if result.Status != "succeeded" || len(result.Outcomes) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	snapshot, err := controller.Snapshot(context.Background(), SnapshotRequest{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snapshot.Exists || snapshot.Services[0].Phase != runstate.RunReady {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestUpStartsEveryWrapperBeforeCompletingHealth(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	supervisor.onHealth = func() {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		if len(supervisor.started) != 2 {
			t.Fatalf("health started after %d wrappers, want 2: %v", len(supervisor.started), supervisor.started)
		}
	}
	controller := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{
				Name: "api", Command: []string{"serve"},
				Health: &engine.HealthCheck{Type: "tcp", Address: "127.0.0.1:1"},
			},
			{Name: "dependency", Command: []string{"serve"}},
		}},
	}}, supervisor)

	result, err := controller.Up(context.Background(), UpRequest{RepoRoot: repoRoot}, nil)
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if result.Status != "succeeded" || len(result.Outcomes) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestUpRejectsUnknownSelectionBeforeMutation(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	controller := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{Name: "web", Command: []string{"serve"}},
		}},
	}}, supervisor)

	_, err := controller.Up(context.Background(), UpRequest{
		RepoRoot: repoRoot,
		Select:   Selection{Services: []string{"missing"}},
	}, nil)
	var operatorErr *OperatorError
	if !errors.As(err, &operatorErr) || operatorErr.Code != CodeServiceUnknown {
		t.Fatalf("error = %v, want %s", err, CodeServiceUnknown)
	}
	store, storeErr := runstate.NewStore(repoRoot)
	if storeErr != nil {
		t.Fatalf("new store: %v", storeErr)
	}
	if _, loadErr := store.LoadEnvironment(context.Background()); loadErr == nil {
		t.Fatal("unknown selection unexpectedly created environment state")
	}
}

func TestDownWithoutStateIsSuccessfulNoOp(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	controller := newTestController(t, repoRoot, staticPlanner{}, supervisor)

	result, err := controller.Down(context.Background(), DownRequest{RepoRoot: repoRoot}, nil)
	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if result.Status != "succeeded" || len(result.Outcomes) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRestartPlansBeforeStopAndUsesOneOperation(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	plan := PlanResult{
		ProfileName: "dev",
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{Name: "web", Command: []string{"serve"}},
		}},
	}
	controller := newTestController(t, repoRoot, staticPlanner{result: plan}, supervisor)
	if _, err := controller.Up(context.Background(), UpRequest{RepoRoot: repoRoot}, nil); err != nil {
		t.Fatalf("initial up: %v", err)
	}

	result, err := controller.Restart(context.Background(), RestartRequest{RepoRoot: repoRoot}, nil)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if result.Kind != "restart" || result.OperationID == "" {
		t.Fatalf("unexpected restart identity: %#v", result)
	}
	if len(result.Outcomes) != 2 {
		t.Fatalf("restart outcomes = %d, want stop and start outcomes", len(result.Outcomes))
	}
	if len(supervisor.stopped) != 1 || len(supervisor.started) != 2 {
		t.Fatalf("supervisor calls: stopped=%v started=%v", supervisor.stopped, supervisor.started)
	}
}

func TestRestartPlanningFailureDoesNotStopService(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	working := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{Name: "web", Command: []string{"serve"}},
		}},
	}}, supervisor)
	if _, err := working.Up(context.Background(), UpRequest{RepoRoot: repoRoot}, nil); err != nil {
		t.Fatalf("initial up: %v", err)
	}
	broken := newTestController(t, repoRoot, staticPlanner{err: errors.New("invalid config")}, supervisor)

	_, err := broken.Restart(context.Background(), RestartRequest{RepoRoot: repoRoot}, nil)
	if err == nil {
		t.Fatal("restart unexpectedly succeeded")
	}
	if len(supervisor.stopped) != 0 {
		t.Fatalf("planning failure stopped services: %v", supervisor.stopped)
	}
}

func TestDownReturnsAllOutcomesAndRetainsFailedCurrentRun(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	controller := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{Name: "api", Command: []string{"serve"}},
			{Name: "web", Command: []string{"serve"}},
		}},
	}}, supervisor)
	if _, err := controller.Up(context.Background(), UpRequest{RepoRoot: repoRoot}, nil); err != nil {
		t.Fatalf("up: %v", err)
	}
	before, err := controller.Snapshot(context.Background(), SnapshotRequest{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("snapshot before down: %v", err)
	}
	var failedRunID string
	for _, service := range before.Services {
		if service.Service == "web" {
			failedRunID = service.RunID
		}
	}
	supervisor.failStopService = "web"

	result, err := controller.Down(context.Background(), DownRequest{RepoRoot: repoRoot}, nil)
	if err == nil {
		t.Fatal("down unexpectedly succeeded")
	}
	if result.Status != "partial" || len(result.Outcomes) != 2 {
		t.Fatalf("unexpected partial result: %#v", result)
	}
	after, snapshotErr := controller.Snapshot(context.Background(), SnapshotRequest{RepoRoot: repoRoot})
	if snapshotErr != nil {
		t.Fatalf("snapshot after down: %v", snapshotErr)
	}
	for _, service := range after.Services {
		switch service.Service {
		case "api":
			if service.RunID != "" || service.Desired != runstate.DesiredStopped {
				t.Fatalf("successfully stopped slot not cleared: %#v", service)
			}
		case "web":
			if service.RunID != failedRunID || service.Desired != runstate.DesiredRunning {
				t.Fatalf("failed stop did not retain ownership: %#v", service)
			}
		}
	}
}

func TestUpHealthFailureStopsOwnedProcessAndRetainsFailedAttempt(t *testing.T) {
	repoRoot := t.TempDir()
	supervisor := &recordingSupervisor{
		t: t, repoRoot: repoRoot, healthErr: errors.New("health deadline"),
	}
	controller := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{
				Name: "web", Command: []string{"serve"},
				Health: &engine.HealthCheck{Type: "tcp", Address: "127.0.0.1:1"},
			},
		}},
	}}, supervisor)

	result, err := controller.Up(context.Background(), UpRequest{RepoRoot: repoRoot}, nil)
	if err == nil {
		t.Fatal("up unexpectedly succeeded")
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Error == nil ||
		result.Outcomes[0].Error.Code != CodeHealthTimeout {
		t.Fatalf("unexpected health failure result: %#v", result)
	}
	runID := result.Outcomes[0].RunID
	store, storeErr := runstate.NewStore(repoRoot)
	if storeErr != nil {
		t.Fatalf("new store: %v", storeErr)
	}
	run, loadErr := store.LoadRun(context.Background(), runID)
	if loadErr != nil {
		t.Fatalf("load run: %v", loadErr)
	}
	if run.Phase != runstate.RunFailed || run.Health == nil || run.Health.Healthy {
		t.Fatalf("unexpected failed health run: %#v", run)
	}
	wrapperStatus, inspectErr := runstate.InspectProcess(run.Wrapper)
	if inspectErr != nil {
		t.Fatalf("inspect stopped wrapper: %v", inspectErr)
	}
	if wrapperStatus != runstate.ProcessAbsent {
		t.Fatalf("health-failed wrapper status = %q, want absent", wrapperStatus)
	}
}

func TestUpCancellationStopsLaunchingAndLeavesReconcilerEvidence(t *testing.T) {
	repoRoot := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	supervisor := &recordingSupervisor{t: t, repoRoot: repoRoot}
	supervisor.onStart = cancel
	controller := newTestController(t, repoRoot, staticPlanner{result: PlanResult{
		Plan: engine.LaunchPlan{Services: []engine.ServiceSpec{
			{Name: "api", Command: []string{"serve"}},
			{Name: "web", Command: []string{"serve"}},
		}},
	}}, supervisor)

	result, err := controller.Up(ctx, UpRequest{RepoRoot: repoRoot}, nil)
	if err == nil || result.Status != "canceled" {
		t.Fatalf("unexpected canceled result: result=%#v err=%v", result, err)
	}
	if len(supervisor.attempted) != 1 || supervisor.attempted[0] != "api" {
		t.Fatalf("start attempts after cancellation: %v", supervisor.attempted)
	}
	store, storeErr := runstate.NewStore(repoRoot)
	if storeErr != nil {
		t.Fatalf("new store: %v", storeErr)
	}
	environment, loadErr := store.LoadEnvironment(context.Background())
	if loadErr != nil {
		t.Fatalf("load environment: %v", loadErr)
	}
	if len(environment.Services) != 2 {
		t.Fatalf("planned state does not cover all services: %#v", environment.Services)
	}
	report, reconcileErr := reconcile(context.Background(), store)
	if reconcileErr != nil {
		t.Fatalf("reconcile canceled starts: %v", reconcileErr)
	}
	if len(report.Actions) != 2 {
		t.Fatalf("reconciliation actions = %d, want 2: %#v", len(report.Actions), report)
	}
	environment, loadErr = store.LoadEnvironment(context.Background())
	if loadErr != nil {
		t.Fatalf("reload environment: %v", loadErr)
	}
	for name, slot := range environment.Services {
		if slot.CurrentRunID != "" || slot.LastRunID == "" {
			t.Fatalf("service %s not reconciled: %#v", name, slot)
		}
	}
}
