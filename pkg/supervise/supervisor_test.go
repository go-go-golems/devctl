package supervise

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
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
