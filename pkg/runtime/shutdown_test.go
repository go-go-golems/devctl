package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRuntimeShutdownEscalation(t *testing.T) {
	tests := []struct {
		mode ShutdownMode
		want ShutdownMode
	}{
		{mode: "eof", want: ShutdownEOF},
		{mode: "early", want: ShutdownEOF},
		{mode: "term", want: ShutdownTerminated},
		{mode: "kill", want: ShutdownKilled},
		{mode: "child", want: ShutdownTerminated},
	}
	for _, test := range tests {
		t.Run(string(test.mode), func(t *testing.T) {
			client, marker := startShutdownFixture(t, string(test.mode))
			start := time.Now()
			require.NoError(t, client.Close(t.Context()))
			reporter := client.(ShutdownReporter)
			result, ok := reporter.ShutdownResult()
			require.True(t, ok)
			require.Equal(t, test.want, result.Mode)
			if test.want == ShutdownEOF {
				require.Less(t, time.Since(start), 150*time.Millisecond)
			} else {
				require.GreaterOrEqual(t, time.Since(start), 75*time.Millisecond)
			}
			if test.mode == "child" {
				data, err := os.ReadFile(marker)
				require.NoError(t, err)
				pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
				require.NoError(t, err)
				require.Eventually(t, func() bool {
					err := syscall.Kill(pid, 0)
					return err == syscall.ESRCH
				}, time.Second, 10*time.Millisecond, "owned descendant remained alive")
			}
		})
	}
}

func TestRuntimeConcurrentCloseUsesOneResult(t *testing.T) {
	client, _ := startShutdownFixture(t, "term")
	const callers = 8
	errors := make(chan error, callers)
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errors <- client.Close(context.Background())
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	result, ok := client.(ShutdownReporter).ShutdownResult()
	require.True(t, ok)
	require.Equal(t, ShutdownTerminated, result.Mode)
}

func TestRuntimeCanceledCloseDoesNotAbandonCleanup(t *testing.T) {
	client, _ := startShutdownFixture(t, "kill")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, client.Close(ctx), context.Canceled)
	require.Eventually(t, func() bool {
		result, ok := client.(ShutdownReporter).ShutdownResult()
		return ok && result.Mode == ShutdownKilled
	}, time.Second, 10*time.Millisecond)
}

func startShutdownFixture(t *testing.T, mode string) (Client, string) {
	t.Helper()
	dir := t.TempDir()
	plugin := filepath.Join(dir, "plugin.py")
	marker := filepath.Join(dir, "marker")
	code := `import json, os, signal, subprocess, sys, time
mode, marker = sys.argv[1], sys.argv[2]
def emit(value): print(json.dumps(value), flush=True)
def on_term(_signal, _frame):
    if mode in ("term", "child"):
        if mode == "term":
            with open(marker, "w", encoding="utf-8") as handle: handle.write("term")
        raise SystemExit(0)
if mode == "kill": signal.signal(signal.SIGTERM, signal.SIG_IGN)
else: signal.signal(signal.SIGTERM, on_term)
child = None
if mode == "child":
    child = subprocess.Popen(["sleep", "60"])
    with open(marker, "w", encoding="utf-8") as handle: handle.write(str(child.pid))
emit({"type":"handshake","protocol_version":"v2","plugin_name":"shutdown","capabilities":{"ops":[]}})
if mode == "early": raise SystemExit(0)
for _line in sys.stdin: pass
if mode == "eof":
    with open(marker, "w", encoding="utf-8") as handle: handle.write("eof")
else:
    while True: time.sleep(0.05)
`
	require.NoError(t, os.WriteFile(plugin, []byte(code), 0o600))
	factory := NewFactory(FactoryOptions{
		HandshakeTimeout: time.Second,
		EOFGraceTimeout:  80 * time.Millisecond,
		ShutdownTimeout:  150 * time.Millisecond,
	})
	client, err := factory.Start(t.Context(), PluginSpec{
		ID: "shutdown", Path: "python3", Args: []string{plugin, mode, marker}, WorkDir: dir,
	}, StartOptions{})
	require.NoError(t, err)
	return client, marker
}
