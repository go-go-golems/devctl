package supervise

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/stretchr/testify/require"
)

func TestSupervisor_StartStop_Sleep(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-supervise-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	s := New(Options{RepoRoot: repoRoot, ReadyTimeout: 1 * time.Second, ShutdownTimeout: 2 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	st, err := s.Start(ctx, engine.LaunchPlan{
		Services: []engine.ServiceSpec{
			{Name: "sleep", Command: []string{"bash", "-lc", "sleep 10"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, st.Services, 1)
	require.True(t, state.ProcessAlive(st.Services[0].PID))

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, s.Stop(stopCtx, st))

	deadline := time.Now().Add(3 * time.Second)
	for state.ProcessAlive(st.Services[0].PID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	require.False(t, state.ProcessAlive(st.Services[0].PID))
}

func TestSupervisor_StartServiceAlreadyRunningFails(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-supervise-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	s := New(Options{RepoRoot: repoRoot, ReadyTimeout: 1 * time.Second, ShutdownTimeout: 2 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	spec := engine.ServiceSpec{Name: "sleep", Command: []string{"bash", "-lc", "sleep 10"}}
	st, err := s.Start(ctx, engine.LaunchPlan{Services: []engine.ServiceSpec{spec}})
	require.NoError(t, err)
	require.Len(t, st.Services, 1)
	pid := st.Services[0].PID
	require.True(t, state.ProcessAlive(pid))

	err = s.StartService(ctx, st, spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already running")
	require.Equal(t, pid, st.Services[0].PID)
	require.True(t, state.ProcessAlive(pid))

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, s.Stop(stopCtx, st))
}

func TestSupervisor_StartDoesNotPersistRawSpecEnv(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-supervise-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	s := New(Options{RepoRoot: repoRoot, ReadyTimeout: 1 * time.Second, ShutdownTimeout: 2 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	st, err := s.Start(ctx, engine.LaunchPlan{
		Services: []engine.ServiceSpec{
			{
				Name:    "sleep",
				Command: []string{"bash", "-lc", "sleep 10"},
				Env:     map[string]string{"API_KEY": "secret-value"},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, st.Services, 1)
	require.Equal(t, "[REDACTED]", st.Services[0].Env["API_KEY"])
	require.NotNil(t, st.Services[0].Spec)
	require.NoError(t, state.Save(repoRoot, st))

	b, err := os.ReadFile(state.StatePath(repoRoot))
	require.NoError(t, err)
	require.NotContains(t, string(b), "secret-value")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, s.Stop(stopCtx, st))
}

func TestSupervisor_ReadinessTimeoutStopsServices(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-supervise-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	s := New(Options{RepoRoot: repoRoot, ReadyTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reserve a free port so we can target an address that will not become ready.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, ln.Close())

	pidFile := filepath.Join(repoRoot, "pid.txt")
	_, err = s.Start(ctx, engine.LaunchPlan{
		Services: []engine.ServiceSpec{
			{
				Name:    "sleep",
				Command: []string{"bash", "-lc", "echo $$ > \"$DEVCTL_TEST_PID_FILE\"; sleep 10"},
				Env:     map[string]string{"DEVCTL_TEST_PID_FILE": pidFile},
				Health:  &engine.HealthCheck{Type: "tcp", Address: "127.0.0.1:" + portStr, TimeoutMs: 2000},
			},
		},
	})
	require.Error(t, err)

	// Ensure we don't leak the long-running process if readiness fails.
	b, readErr := os.ReadFile(pidFile)
	require.NoError(t, readErr)
	var pid int
	_, scanErr := fmt.Sscanf(string(b), "%d", &pid)
	require.NoError(t, scanErr)
	require.Greater(t, pid, 0)

	deadline := time.Now().Add(3 * time.Second)
	for state.ProcessAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	require.False(t, state.ProcessAlive(pid))
}

func TestSupervisor_PostReadyCrashIsObservable(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-supervise-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	s := New(Options{RepoRoot: repoRoot, ReadyTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, ln.Close())

	st, err := s.Start(ctx, engine.LaunchPlan{
		Services: []engine.ServiceSpec{
			{
				Name:    "crashy",
				Command: []string{"bash", "-lc", "timeout 1s python3 -m http.server " + portStr + " --bind 127.0.0.1"},
				Health:  &engine.HealthCheck{Type: "tcp", Address: "127.0.0.1:" + portStr, TimeoutMs: 2000},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, st.Services, 1)
	pid := st.Services[0].PID
	require.True(t, state.ProcessAlive(pid))

	deadline := time.Now().Add(5 * time.Second)
	for state.ProcessAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	require.False(t, state.ProcessAlive(pid))

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	_ = s.Stop(stopCtx, st)
}

func TestCompleteHealthWithoutCheckRejectsExitedRun(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	require.NoError(t, err)

	runID, err := runstate.NewRunID()
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(context.Background(), runstate.RunRecord{
		RunID:     runID,
		Service:   "fast-exit",
		Phase:     runstate.RunStarting,
		Spec:      runstate.ServiceSpecRecord{Name: "fast-exit", Command: []string{"false"}},
		Wrapper:   &runstate.ProcessIdentity{PID: 999_998, StartToken: "gone"},
		Child:     &runstate.ProcessIdentity{PID: 999_999, StartToken: "gone"},
		ChildPGID: 999_999,
	}))
	runDir, err := store.RunDir(runID)
	require.NoError(t, err)
	exitCode := 1
	require.NoError(t, state.WriteExitInfo(filepath.Join(runDir, ExitRecordName), state.ExitInfo{
		Service:  "fast-exit",
		ExitedAt: time.Now().UTC(),
		ExitCode: &exitCode,
	}))

	s := New(Options{RepoRoot: repoRoot})
	err = s.CompleteHealth(context.Background(), engine.ServiceSpec{Name: "fast-exit"}, runID)
	require.ErrorContains(t, err, "exited before readiness")

	run, err := store.LoadRun(context.Background(), runID)
	require.NoError(t, err)
	require.Equal(t, runstate.RunExited, run.Phase)
	require.NotNil(t, run.Exit)
	require.Equal(t, exitCode, *run.Exit.ExitCode)
	require.Equal(t, "E_PROCESS_EXIT", run.LastError.Code)
}

func TestTerminateOwnedProcessGroupsKillsTermResistantChildGroup(t *testing.T) {
	wrapper := exec.Command("sh", "-c", "sleep 30")
	wrapper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, wrapper.Start())

	child := exec.Command("sh", "-c", "trap '' TERM; while :; do sleep 1; done")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, child.Start())

	wrapperDone := make(chan error, 1)
	childDone := make(chan error, 1)
	go func() { wrapperDone <- wrapper.Wait() }()
	go func() { childDone <- child.Wait() }()
	t.Cleanup(func() {
		_ = wrapper.Process.Kill()
		_ = child.Process.Kill()
	})

	err := terminateOwnedProcessGroups(
		context.Background(),
		wrapper.Process.Pid,
		child.Process.Pid,
		100*time.Millisecond,
	)
	require.NoError(t, err)

	select {
	case <-wrapperDone:
	case <-time.After(time.Second):
		t.Fatal("wrapper process was not reaped")
	}
	select {
	case <-childDone:
	case <-time.After(time.Second):
		t.Fatal("child process group survived SIGKILL escalation")
	}
}
