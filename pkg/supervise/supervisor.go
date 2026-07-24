package supervise

import (
	"context"
	stderrors "errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Options struct {
	RepoRoot        string
	ShutdownTimeout time.Duration
	ReadyTimeout    time.Duration
	WrapperExe      string
}

type Supervisor struct {
	opts Options
}

func New(opts Options) *Supervisor {
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = 3 * time.Second
	}
	if opts.ReadyTimeout <= 0 {
		opts.ReadyTimeout = 30 * time.Second
	}
	return &Supervisor{opts: opts}
}

func (s *Supervisor) Start(ctx context.Context, plan engine.LaunchPlan) (*state.State, error) {
	if s.opts.RepoRoot == "" {
		return nil, errors.New("missing RepoRoot")
	}
	if err := os.MkdirAll(state.LogsDir(s.opts.RepoRoot), 0o755); err != nil {
		return nil, errors.Wrap(err, "mkdir logs dir")
	}

	st := &state.State{
		RepoRoot:  s.opts.RepoRoot,
		CreatedAt: time.Now(),
		Services:  []state.ServiceRecord{},
	}

	for _, svc := range plan.Services {
		rec, err := s.startService(ctx, svc)
		if err != nil {
			_ = s.Stop(context.Background(), st)
			return nil, err
		}
		st.Services = append(st.Services, rec)
	}

	for index, svc := range plan.Services {
		var err error
		if st.Services[index].RunID != "" {
			err = s.CompleteHealth(ctx, svc, st.Services[index].RunID)
		} else if svc.Health != nil {
			readyCtx, cancel := context.WithTimeout(ctx, s.opts.ReadyTimeout)
			err = waitReady(readyCtx, svc)
			cancel()
		}
		if err != nil {
			_ = s.Stop(context.Background(), st)
			return nil, err
		}
	}

	return st, nil
}

func (s *Supervisor) Stop(ctx context.Context, st *state.State) error {
	if st == nil {
		return nil
	}
	var lastErr error
	for _, svc := range st.Services {
		if svc.PID <= 0 {
			continue
		}
		if err := terminatePIDGroup(ctx, svc.PID, s.opts.ShutdownTimeout); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// StopService stops a single named service, clears its PID in the state,
// and saves the updated state file. The service record is kept so it can be
// restarted later.
func (s *Supervisor) StopService(ctx context.Context, st *state.State, name string) error {
	if st == nil {
		return errors.New("state is nil")
	}
	var svc *state.ServiceRecord
	for i := range st.Services {
		if st.Services[i].Name == name {
			svc = &st.Services[i]
			break
		}
	}
	if svc == nil {
		return errors.Errorf("service %q not found in state", name)
	}

	if svc.PID > 0 && state.ProcessAlive(svc.PID) {
		if err := terminatePIDGroup(ctx, svc.PID, s.opts.ShutdownTimeout); err != nil {
			return errors.Wrapf(err, "failed to stop service %q", name)
		}
	}

	svc.PID = 0
	svc.ExitInfo = ""
	return state.Save(s.opts.RepoRoot, st)
}

// StartService starts a single named service from a freshly resolved ServiceSpec.
// It creates new log files, starts the process, waits for health checks,
// and updates the state file.
func (s *Supervisor) StartService(ctx context.Context, st *state.State, spec engine.ServiceSpec) error {
	if st == nil {
		return errors.New("state is nil")
	}
	name := spec.Name
	if name == "" {
		return errors.New("service spec missing name")
	}
	var rec *state.ServiceRecord
	for i := range st.Services {
		if st.Services[i].Name == name {
			rec = &st.Services[i]
			break
		}
	}
	if rec == nil {
		return errors.Errorf("service %q not found in state", name)
	}
	if rec.PID > 0 && state.ProcessAlive(rec.PID) {
		return errors.Errorf("service %q is already running (pid %d)", name, rec.PID)
	}

	newRec, err := s.startService(ctx, spec)
	if err != nil {
		return errors.Wrapf(err, "failed to start service %q", name)
	}

	var healthErr error
	if newRec.RunID != "" {
		healthErr = s.CompleteHealth(ctx, spec, newRec.RunID)
	} else if spec.Health != nil {
		readyCtx, cancel := context.WithTimeout(ctx, s.opts.ReadyTimeout)
		healthErr = waitReady(readyCtx, spec)
		cancel()
	}
	if healthErr != nil {
		_ = terminatePIDGroup(context.Background(), newRec.PID, s.opts.ShutdownTimeout)
		return errors.Wrapf(healthErr, "service %q health check failed", name)
	}

	rec.PID = newRec.PID
	rec.RunID = newRec.RunID
	rec.WrapperStartToken = newRec.WrapperStartToken
	rec.ChildPID = newRec.ChildPID
	rec.ChildStartToken = newRec.ChildStartToken
	rec.ChildPGID = newRec.ChildPGID
	rec.Command = newRec.Command
	rec.Cwd = newRec.Cwd
	rec.Env = newRec.Env
	rec.StdoutLog = newRec.StdoutLog
	rec.StderrLog = newRec.StderrLog
	rec.ExitInfo = ""
	rec.StartedAt = newRec.StartedAt
	rec.HealthType = newRec.HealthType
	rec.HealthAddress = newRec.HealthAddress
	rec.HealthURL = newRec.HealthURL
	rec.Spec = newRec.Spec

	return state.Save(s.opts.RepoRoot, st)
}

// RestartService stops and then starts a single named service from a freshly resolved ServiceSpec.
func (s *Supervisor) RestartService(ctx context.Context, st *state.State, spec engine.ServiceSpec) error {
	if err := s.StopService(ctx, st, spec.Name); err != nil {
		return errors.Wrap(err, "stop failed")
	}
	if err := s.StartService(ctx, st, spec); err != nil {
		return errors.Wrap(err, "start failed")
	}
	return nil
}

func (s *Supervisor) startService(ctx context.Context, svc engine.ServiceSpec) (state.ServiceRecord, error) {
	if svc.Name == "" {
		return state.ServiceRecord{}, errors.New("service name is required")
	}
	if len(svc.Command) == 0 {
		return state.ServiceRecord{}, errors.Errorf("service %q missing command", svc.Name)
	}

	cwd := s.opts.RepoRoot
	if svc.Cwd != "" {
		if filepath.IsAbs(svc.Cwd) {
			cwd = svc.Cwd
		} else {
			cwd = filepath.Join(s.opts.RepoRoot, svc.Cwd)
		}
	}

	ts := time.Now().Format("20060102-150405")
	stdoutPath := filepath.Join(state.LogsDir(s.opts.RepoRoot), svc.Name+"-"+ts+".stdout.log")
	stderrPath := filepath.Join(state.LogsDir(s.opts.RepoRoot), svc.Name+"-"+ts+".stderr.log")

	if s.opts.WrapperExe == "" {
		stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return state.ServiceRecord{}, errors.Wrap(err, "open stdout log")
		}
		defer func() { _ = stdoutFile.Close() }()

		stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return state.ServiceRecord{}, errors.Wrap(err, "open stderr log")
		}
		defer func() { _ = stderrFile.Close() }()

		// #nosec G204 -- command is configured in the repo spec.
		cmd := exec.CommandContext(ctx, svc.Command[0], svc.Command[1:]...)
		cmd.Dir = cwd
		cmd.Env = mergeEnv(os.Environ(), svc.Env)
		cmd.Stdout = stdoutFile
		cmd.Stderr = stderrFile
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := cmd.Start(); err != nil {
			return state.ServiceRecord{}, errors.Wrap(err, "start service")
		}

		pid := cmd.Process.Pid
		startedAt := time.Now()
		log.Info().Str("service", svc.Name).Int("pid", pid).Msg("service started")
		go func() { _ = cmd.Wait() }()

		rec := state.ServiceRecord{
			Name:      svc.Name,
			PID:       pid,
			Command:   svc.Command,
			Cwd:       cwd,
			Env:       state.SanitizeEnv(svc.Env),
			StdoutLog: stdoutPath,
			StderrLog: stderrPath,
			StartedAt: startedAt,
			Spec:      specRecordFromServiceSpec(svc, cwd),
		}
		if svc.Health != nil {
			rec.HealthType = svc.Health.Type
			rec.HealthAddress = svc.Health.Address
			rec.HealthURL = svc.Health.URL
		}
		return rec, nil
	}

	store, err := runstate.NewStore(s.opts.RepoRoot)
	if err != nil {
		return state.ServiceRecord{}, err
	}
	runID, err := runstate.NewRunID()
	if err != nil {
		return state.ServiceRecord{}, err
	}
	run := runstate.RunRecord{
		RunID:   runID,
		Service: svc.Name,
		Phase:   runstate.RunPlanned,
		Spec: runstate.ServiceSpecRecord{
			Name:        svc.Name,
			Command:     append([]string{}, svc.Command...),
			Cwd:         cwd,
			Environment: state.SanitizeEnv(svc.Env),
			Health:      runstateHealthRecord(svc),
		},
	}
	if err := store.CreateRun(ctx, run); err != nil {
		return state.ServiceRecord{}, errors.Wrap(err, "create planned run")
	}
	return s.StartPreparedService(ctx, svc, runID)
}

// StartPreparedService starts a wrapper for a run record that the controller
// created before this call. It never creates or selects an environment slot.
func (s *Supervisor) StartPreparedService(
	ctx context.Context,
	svc engine.ServiceSpec,
	runID string,
) (state.ServiceRecord, error) {
	if s.opts.WrapperExe == "" {
		return state.ServiceRecord{}, errors.New("prepared service start requires WrapperExe")
	}
	if svc.Name == "" {
		return state.ServiceRecord{}, errors.New("service name is required")
	}
	if len(svc.Command) == 0 {
		return state.ServiceRecord{}, errors.Errorf("service %q missing command", svc.Name)
	}
	cwd := s.opts.RepoRoot
	if svc.Cwd != "" {
		if filepath.IsAbs(svc.Cwd) {
			cwd = svc.Cwd
		} else {
			cwd = filepath.Join(s.opts.RepoRoot, svc.Cwd)
		}
	}
	store, err := runstate.NewStore(s.opts.RepoRoot)
	if err != nil {
		return state.ServiceRecord{}, err
	}
	prepared, err := store.LoadRun(ctx, runID)
	if err != nil {
		return state.ServiceRecord{}, errors.Wrap(err, "load prepared run")
	}
	if prepared.Service != svc.Name || prepared.Spec.Name != svc.Name {
		return state.ServiceRecord{}, errors.Errorf(
			"prepared run %q belongs to service %q, not %q",
			runID,
			prepared.Service,
			svc.Name,
		)
	}
	if prepared.Phase != runstate.RunPlanned {
		return state.ServiceRecord{}, errors.Errorf(
			"prepared run %q has phase %q, expected %q",
			runID,
			prepared.Phase,
			runstate.RunPlanned,
		)
	}

	request, err := NewWrapperRequest(store, runID, svc.Name, cwd, svc.Command, svc.Env)
	if err != nil {
		markRunFailed(store, runID, "WRAPPER_REQUEST_INVALID", err)
		return state.ServiceRecord{}, err
	}
	if err := WriteWrapperRequest(request); err != nil {
		markRunFailed(store, runID, "WRAPPER_REQUEST_WRITE_FAILED", err)
		return state.ServiceRecord{}, err
	}
	if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunStarting
		return nil
	}); err != nil {
		_ = os.Remove(request.RequestPath())
		return state.ServiceRecord{}, errors.Wrap(err, "mark run starting")
	}

	// #nosec G204 -- wrapper executable is configured in the repo spec.
	cmd := exec.Command(s.opts.WrapperExe, "__wrap-service", "--request", request.RequestPath())
	cmd.Dir = s.opts.RepoRoot
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(request.RequestPath())
		markRunFailed(store, runID, "WRAPPER_START_FAILED", err)
		return state.ServiceRecord{}, errors.Wrap(err, "start wrapper")
	}

	pid := cmd.Process.Pid
	log.Info().Str("service", svc.Name).Int("pid", pid).Msg("service started")
	wrapperIdentity, err := runstate.ReadProcessIdentity(pid)
	if err != nil {
		_ = terminatePIDGroup(context.Background(), pid, time.Second)
		markRunFailed(store, runID, "WRAPPER_IDENTITY_FAILED", err)
		return state.ServiceRecord{}, errors.Wrap(err, "read launched wrapper identity")
	}

	handshakeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	owner, ready, err := waitWrapperHandshake(handshakeCtx, request, wrapperIdentity)
	cancel()
	if err != nil {
		_ = terminatePIDGroup(context.Background(), pid, time.Second)
		_ = os.Remove(request.RequestPath())
		markRunFailed(store, runID, "WRAPPER_HANDSHAKE_FAILED", err)
		return state.ServiceRecord{}, err
	}
	if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunStarting
		record.Wrapper = &owner.Wrapper
		record.Child = &ready.Child
		record.ChildPGID = ready.ChildPGID
		return nil
	}); err != nil {
		_ = terminatePIDGroup(context.Background(), pid, time.Second)
		return state.ServiceRecord{}, errors.Wrap(err, "persist ready run")
	}

	rec := state.ServiceRecord{
		Name:              svc.Name,
		PID:               pid,
		RunID:             runID,
		WrapperStartToken: owner.Wrapper.StartToken,
		ChildPID:          ready.Child.PID,
		ChildStartToken:   ready.Child.StartToken,
		ChildPGID:         ready.ChildPGID,
		Command:           svc.Command,
		Cwd:               cwd,
		Env:               state.SanitizeEnv(svc.Env),
		StdoutLog:         request.StdoutPath(),
		StderrLog:         request.StderrPath(),
		ExitInfo:          request.ExitPath(),
		StartedAt:         ready.WrittenAt,
		Spec:              specRecordFromServiceSpec(svc, cwd),
	}
	if svc.Health != nil {
		rec.HealthType = svc.Health.Type
		rec.HealthAddress = svc.Health.Address
		rec.HealthURL = svc.Health.URL
	}
	return rec, nil
}

// CompleteHealth evaluates the service health contract and records the result.
// A service without a health check transitions immediately to ready.
func (s *Supervisor) CompleteHealth(ctx context.Context, svc engine.ServiceSpec, runID string) error {
	store, err := runstate.NewStore(s.opts.RepoRoot)
	if err != nil {
		return err
	}
	startedAt := time.Now()
	if svc.Health == nil {
		return store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
			record.Phase = runstate.RunReady
			record.Health = &runstate.HealthResult{
				Healthy:   true,
				CheckedAt: time.Now().UTC(),
				Detail:    "no health check configured",
			}
			return nil
		})
	}

	timeout := s.opts.ReadyTimeout
	if svc.Health.TimeoutMs > 0 {
		timeout = time.Duration(svc.Health.TimeoutMs) * time.Millisecond
	}
	healthContext, cancel := context.WithTimeout(ctx, timeout)
	healthErr := waitReady(healthContext, svc)
	cancel()
	checkedAt := time.Now().UTC()
	duration := time.Since(startedAt)
	if healthErr != nil {
		_ = store.UpdateRun(context.Background(), runID, func(record *runstate.RunRecord) error {
			record.Phase = runstate.RunFailed
			record.Health = &runstate.HealthResult{
				Healthy:    false,
				CheckedAt:  checkedAt,
				DurationMs: duration.Milliseconds(),
				Detail:     healthErr.Error(),
			}
			record.LastError = &runstate.ErrorRecord{
				Code:    "E_HEALTH_TIMEOUT",
				Message: healthErr.Error(),
			}
			return nil
		})
		return healthErr
	}
	return store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunReady
		record.Health = &runstate.HealthResult{
			Healthy:    true,
			CheckedAt:  checkedAt,
			DurationMs: duration.Milliseconds(),
		}
		return nil
	})
}

// StopPreparedService stops only processes whose PID and start token match the
// prepared run record. It records exited or unknown and never clears the
// environment index; the controller owns that mutation.
func (s *Supervisor) StopPreparedService(ctx context.Context, runID string) error {
	store, err := runstate.NewStore(s.opts.RepoRoot)
	if err != nil {
		return err
	}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		return errors.Wrap(err, "load run for stop")
	}
	if run.Wrapper == nil || run.Child == nil || run.ChildPGID <= 0 {
		return errors.Errorf("run %q has no complete ownership handshake", runID)
	}
	wrapperStatus, err := runstate.InspectProcess(run.Wrapper)
	if err != nil {
		return errors.Wrap(err, "inspect wrapper before stop")
	}
	childStatus, err := runstate.InspectProcess(run.Child)
	if err != nil {
		return errors.Wrap(err, "inspect child before stop")
	}
	if wrapperStatus == runstate.ProcessMismatch || childStatus == runstate.ProcessMismatch {
		markRunUnknown(store, runID, "PROCESS_IDENTITY_MISMATCH", "PID start token changed before stop")
		return errors.Errorf("run %q process identity mismatch", runID)
	}
	if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunStopping
		return nil
	}); err != nil {
		return errors.Wrap(err, "mark run stopping")
	}

	if wrapperStatus == runstate.ProcessMatches {
		if err := terminatePIDGroup(ctx, run.Wrapper.PID, s.opts.ShutdownTimeout); err != nil {
			markRunUnknown(store, runID, "STOP_FAILED", err.Error())
			return err
		}
	} else if childStatus == runstate.ProcessMatches {
		if err := terminatePIDGroup(ctx, run.Child.PID, s.opts.ShutdownTimeout); err != nil {
			markRunUnknown(store, runID, "STOP_FAILED", err.Error())
			return err
		}
	}

	wrapperStatus, err = runstate.InspectProcess(run.Wrapper)
	if err != nil {
		return errors.Wrap(err, "inspect wrapper after stop")
	}
	childStatus, err = runstate.InspectProcess(run.Child)
	if err != nil {
		return errors.Wrap(err, "inspect child after stop")
	}
	if wrapperStatus != runstate.ProcessAbsent || childStatus != runstate.ProcessAbsent {
		message := "owned process remains or identity changed after stop"
		markRunUnknown(store, runID, "STOP_FAILED", message)
		return errors.Errorf("run %q: %s", runID, message)
	}

	var exitSummary *runstate.ExitSummary
	runDir, pathErr := store.RunDir(runID)
	if pathErr == nil {
		exitInfo, readErr := state.ReadExitInfo(filepath.Join(runDir, ExitRecordName))
		if readErr == nil {
			exitSummary = &runstate.ExitSummary{
				ExitedAt: exitInfo.ExitedAt,
				ExitCode: exitInfo.ExitCode,
				Signal:   exitInfo.Signal,
				Error:    exitInfo.Error,
			}
		}
	}
	return store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunExited
		record.Exit = exitSummary
		return nil
	})
}

func markRunUnknown(store *runstate.Store, runID string, code string, message string) {
	if store == nil || runID == "" {
		return
	}
	_ = store.UpdateRun(context.Background(), runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunUnknown
		record.LastError = &runstate.ErrorRecord{Code: code, Message: message}
		return nil
	})
}

func waitWrapperHandshake(
	ctx context.Context,
	request *WrapperRequest,
	wrapperIdentity *runstate.ProcessIdentity,
) (*OwnerRecord, *ReadyRecord, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var owner *OwnerRecord
	for {
		if owner == nil {
			record, err := ReadOwnerRecord(request.OwnerPath())
			if err == nil {
				if err := ValidateOwnerRecord(ctx, request, wrapperIdentity, record); err != nil {
					return nil, nil, err
				}
				owner = record
			} else if !os.IsNotExist(errors.Cause(err)) {
				return nil, nil, err
			}
		}
		if owner != nil {
			record, err := ReadReadyRecord(request.ReadyPath())
			if err == nil {
				if err := ValidateReadyRecord(ctx, request, owner, record); err != nil {
					return nil, nil, err
				}
				return owner, record, nil
			} else if !os.IsNotExist(errors.Cause(err)) {
				return nil, nil, err
			}
		}

		select {
		case <-ctx.Done():
			if owner == nil {
				return nil, nil, stderrors.Join(ErrOwnerRecordMissing, ctx.Err())
			}
			return nil, nil, stderrors.Join(ErrReadyRecordMissing, ctx.Err())
		case <-ticker.C:
		}
	}
}

func markRunFailed(store *runstate.Store, runID string, code string, failure error) {
	if store == nil || runID == "" || failure == nil {
		return
	}
	_ = store.UpdateRun(context.Background(), runID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunFailed
		record.LastError = &runstate.ErrorRecord{
			Code:    code,
			Message: failure.Error(),
		}
		return nil
	})
}

func runstateHealthRecord(service engine.ServiceSpec) *runstate.HealthCheckRecord {
	if service.Health == nil {
		return nil
	}
	return &runstate.HealthCheckRecord{
		Type:      service.Health.Type,
		Address:   service.Health.Address,
		URL:       service.Health.URL,
		TimeoutMs: service.Health.TimeoutMs,
	}
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := append([]string{}, base...)
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

func waitReady(ctx context.Context, svc engine.ServiceSpec) error {
	h := svc.Health
	if h == nil {
		return nil
	}

	switch strings.ToLower(h.Type) {
	case "tcp":
		if h.Address == "" {
			return errors.Errorf("service %q health tcp missing address", svc.Name)
		}
		return waitTCP(ctx, h.Address)
	case "http":
		url := h.URL
		if url == "" {
			url = h.Address
		}
		if url == "" {
			return errors.Errorf("service %q health http missing url", svc.Name)
		}
		return waitHTTP(ctx, url)
	default:
		return errors.Errorf("service %q unsupported health type %q", svc.Name, h.Type)
	}
}

func waitTCP(ctx context.Context, address string) error {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()

	for {
		d := net.Dialer{Timeout: 200 * time.Millisecond}
		conn, err := d.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "tcp health timeout")
		case <-t.C:
		}
	}
}

func waitHTTP(ctx context.Context, url string) error {
	t := time.NewTicker(300 * time.Millisecond)
	defer t.Stop()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "http health timeout")
		case <-t.C:
		}
	}
}

func terminatePIDGroup(ctx context.Context, pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	ctxDeadline, ok := ctx.Deadline()
	if ok {
		remaining := time.Until(ctxDeadline)
		if remaining < timeout {
			timeout = remaining
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	deadline := time.Now().Add(timeout)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()

	for {
		if !state.ProcessAlive(pid) {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}

	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}

	killDeadline := time.Now().Add(2 * time.Second)
	for state.ProcessAlive(pid) && time.Now().Before(killDeadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}

	if state.ProcessAlive(pid) {
		return errors.New("failed to stop service")
	}
	return nil
}

func specRecordFromServiceSpec(svc engine.ServiceSpec, cwd string) *state.ServiceSpecRecord {
	rec := &state.ServiceSpecRecord{
		Name:    svc.Name,
		Cwd:     cwd,
		Command: svc.Command,
	}
	if svc.Health != nil {
		rec.Health = &state.HealthCheckRecord{
			Type:      svc.Health.Type,
			Address:   svc.Health.Address,
			URL:       svc.Health.URL,
			TimeoutMs: svc.Health.TimeoutMs,
		}
	}
	return rec
}
