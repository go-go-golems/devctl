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
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Options struct {
	RepoRoot        string
	ShutdownTimeout time.Duration
	ReadyTimeout    time.Duration
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

	for _, svc := range plan.Services {
		if svc.Health == nil {
			continue
		}
		readyCtx, cancel := context.WithTimeout(ctx, s.opts.ReadyTimeout)
		err := waitReady(readyCtx, svc)
		cancel()
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

	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return state.ServiceRecord{}, errors.Wrap(err, "open stdout log")
	}
	defer func() { _ = stdoutFile.Close() }()

	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return state.ServiceRecord{}, errors.Wrap(err, "open stderr log")
	}
	defer func() { _ = stderrFile.Close() }()

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
	log.Info().Str("service", svc.Name).Int("pid", pid).Msg("service started")
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Warn().Err(err).Str("service", svc.Name).Int("pid", pid).Msg("service exited")
		} else {
			log.Info().Str("service", svc.Name).Int("pid", pid).Msg("service exited")
		}
	}()

	return state.ServiceRecord{
		Name:      svc.Name,
		PID:       pid,
		Command:   svc.Command,
		Cwd:       cwd,
		Env:       svc.Env,
		StdoutLog: stdoutPath,
		StderrLog: stderrPath,
	}, nil
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
		if !processAlive(pid) {
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
	for processAlive(pid) && time.Now().Before(killDeadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}

	if processAlive(pid) {
		return errors.New("failed to stop service")
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if stderrors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}
