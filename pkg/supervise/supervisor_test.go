package supervise

import (
	"context"
	"os"
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
