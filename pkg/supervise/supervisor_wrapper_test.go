package supervise

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/stretchr/testify/require"
)

func TestSupervisorWrapperHandshakeCreatesRunArtifacts(t *testing.T) {
	repoRoot := t.TempDir()
	devctlBinary := buildDevctlForSupervisorTest(t)
	supervisor := New(Options{
		RepoRoot:        repoRoot,
		WrapperExe:      devctlBinary,
		ReadyTimeout:    time.Second,
		ShutdownTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	legacyState, err := supervisor.Start(ctx, engine.LaunchPlan{
		Services: []engine.ServiceSpec{
			{
				Name:    "wrapped-sleep",
				Command: []string{"sh", "-c", "printf 'out\\r\\n'; printf 'err\\n' >&2; sleep 30"},
				Env:     map[string]string{"API_KEY": "secret-value"},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, legacyState.Services, 1)
	service := legacyState.Services[0]
	require.NotEmpty(t, service.RunID)
	require.NotEmpty(t, service.WrapperStartToken)
	require.Positive(t, service.ChildPID)
	require.NotEmpty(t, service.ChildStartToken)
	require.Equal(t, service.ChildPID, service.ChildPGID)
	require.True(t, state.ProcessAlive(service.PID))

	store, err := runstate.NewStore(repoRoot)
	require.NoError(t, err)
	run, err := store.LoadRun(context.Background(), service.RunID)
	require.NoError(t, err)
	require.Equal(t, runstate.RunReady, run.Phase)
	require.Equal(t, service.PID, run.Wrapper.PID)
	require.Equal(t, service.ChildPID, run.Child.PID)

	runDir, err := store.RunDir(service.RunID)
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(runDir, WrapperRequestName))
	require.FileExists(t, filepath.Join(runDir, OwnerRecordName))
	require.FileExists(t, filepath.Join(runDir, ReadyRecordName))
	require.Eventually(t, func() bool {
		info, statErr := os.Stat(filepath.Join(runDir, JournalLogName))
		return statErr == nil && info.Size() > 0
	}, time.Second, 10*time.Millisecond)

	journal, err := os.Open(filepath.Join(runDir, JournalLogName))
	require.NoError(t, err)
	defer func() { _ = journal.Close() }()
	scanner := bufio.NewScanner(journal)
	records := make([]runlog.LogRecord, 0, 2)
	for scanner.Scan() {
		var record runlog.LogRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
		records = append(records, record)
	}
	require.NoError(t, scanner.Err())
	require.Len(t, records, 2)
	textByStream := map[runlog.StreamKind]string{}
	for index, record := range records {
		require.Equal(t, uint64(index+1), record.Sequence)
		require.Equal(t, service.RunID, record.RunID)
		textByStream[record.Stream] = record.Text
	}
	require.Equal(t, "out", textByStream[runlog.StreamStdout])
	require.Equal(t, "err", textByStream[runlog.StreamStderr])
	stdoutRaw, err := os.ReadFile(filepath.Join(runDir, StdoutLogName))
	require.NoError(t, err)
	require.Equal(t, "out\r\n", string(stdoutRaw))
	stderrRaw, err := os.ReadFile(filepath.Join(runDir, StderrLogName))
	require.NoError(t, err)
	require.Equal(t, "err\n", string(stderrRaw))

	runJSON, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	require.NoError(t, err)
	require.NotContains(t, string(runJSON), "secret-value")
	require.Contains(t, string(runJSON), "[REDACTED]")

	stopContext, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, supervisor.Stop(stopContext, legacyState))
}

func buildDevctlForSupervisorTest(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	binary := filepath.Join(t.TempDir(), "devctl")

	command := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./cmd/devctl")
	command.Dir = repositoryRoot
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return binary
}
