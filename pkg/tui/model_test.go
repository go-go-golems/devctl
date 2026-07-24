package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/stretchr/testify/require"
)

func TestOverviewSelectionSurvivesSnapshotAndNamesConfirmationTarget(t *testing.T) {
	controller := &fakeController{}
	model := NewModel(Options{Context: t.Context(), Controller: controller, RepoRoot: t.TempDir()})
	model.Update(SnapshotMsg{Snapshot: testSnapshot(1)})
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, []string{"worker"}, model.overview.SelectedServices())

	model.Update(SnapshotMsg{Snapshot: testSnapshot(2)})
	require.Equal(t, []string{"worker"}, model.overview.SelectedServices())
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, model.confirmation)
	require.Equal(t, "restart", model.confirmation.Kind)
	require.Equal(t, []string{"worker"}, model.confirmation.Services)
	require.Contains(t, model.View(), "Confirm restart for services [worker]?")
}

func TestSameRevisionUpdatesHealthWithoutAddingRunHistory(t *testing.T) {
	model := NewModel(Options{Context: t.Context(), Controller: &fakeController{}, RepoRoot: t.TempDir()})
	first := testSnapshot(7)
	model.Update(SnapshotMsg{Snapshot: first})
	sameRevision := testSnapshot(7)
	sameRevision.Services[0].Health = &runstate.HealthResult{Healthy: true}
	model.Update(SnapshotMsg{Snapshot: sameRevision})
	require.True(t, model.overview.Snapshot.Services[0].Health.Healthy)
	require.Empty(t, model.runs.Entries)
}

func TestLogsBufferIsBoundedWhilePausedAndPreservesViewState(t *testing.T) {
	logs := NewLogsModel()
	logs.MaxRecords = 3
	logs.MaxBytes = 12
	logs.Services = []string{"api"}
	logs.Search = "match"
	logs.Wrap = true
	logs.TogglePause()
	for sequence, text := range []string{"one", "match-two", "three", "four"} {
		logs.Add(runlog.LogRecord{
			RunID: "run", Sequence: uint64(sequence + 1), Service: "api",
			Stream: runlog.StreamStdout, Text: text,
		})
	}
	require.LessOrEqual(t, len(logs.Records), 3)
	require.Positive(t, logs.Dropped)
	require.True(t, logs.Paused)
	require.Equal(t, "match", logs.Search)
	require.True(t, logs.Wrap)
	require.Less(t, logs.VisibleCount, len(logs.Records))
	logs.TogglePause()
	require.Equal(t, len(logs.Records), logs.VisibleCount)
}

func TestTypedPartialOperationPopulatesRunsWithoutParsingText(t *testing.T) {
	model := NewModel(Options{Context: t.Context(), Controller: &fakeController{}, RepoRoot: t.TempDir()})
	operationErr := &operator.OperatorError{
		Code: operator.CodePartialFailure, Message: "one service failed",
	}
	result := operator.OperationResult{
		OperationID: "op-1", Kind: "restart", Status: "partial",
		Outcomes: []operator.ServiceOutcome{{
			Service: "api", Error: &operator.OperatorError{
				Code: operator.CodeHealthTimeout, Message: "not ready",
			},
		}},
	}
	model.Update(OperationDoneMsg{Result: result, Err: operationErr})
	require.Len(t, model.runs.Entries, 1)
	require.Equal(t, "partial", model.runs.Entries[0].Result.Status)
	require.Contains(t, model.runs.View(24), operator.CodeHealthTimeout)
	require.Contains(t, model.status, "partial")
}

func TestModalPrecedencePreventsQuit(t *testing.T) {
	model := NewModel(Options{Context: t.Context(), Controller: &fakeController{}, RepoRoot: t.TempDir()})
	model.Update(SnapshotMsg{Snapshot: testSnapshot(1)})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.Nil(t, command)
	require.Nil(t, model.confirmation)
}

func TestDirectNavigationPreservesLogFilters(t *testing.T) {
	model := NewModel(Options{Context: t.Context(), Controller: &fakeController{}, RepoRoot: t.TempDir()})
	model.logs.Services = []string{"api"}
	model.logs.Search = "ready"
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	require.Equal(t, ViewLogs, model.active)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	require.Equal(t, []string{"api"}, model.logs.Services)
	require.Equal(t, "ready", model.logs.Search)
}

func TestViewsAtOperatorTerminalSizes(t *testing.T) {
	model := NewModel(Options{Context: t.Context(), Controller: &fakeController{}, RepoRoot: t.TempDir()})
	model.Update(SnapshotMsg{Snapshot: testSnapshot(1)})
	for _, size := range []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 120, Height: 30},
		{Width: 44, Height: 16},
	} {
		model.Update(size)
		view := model.View()
		require.Contains(t, view, "[1] Overview")
		require.Contains(t, view, "[2] Logs")
		require.Contains(t, view, "[3] Runs")
		require.Contains(t, view, "api")
		require.NotContains(t, view, "Pipeline")
		require.NotContains(t, view, "Plugins")
		require.NotContains(t, view, "Streams")
	}
}

func testSnapshot(revision uint64) operator.Snapshot {
	return operator.Snapshot{
		Exists: true, Profile: "dev", Revision: revision,
		Services: []operator.ServiceSnapshot{
			{
				Service: "api", Desired: runstate.DesiredRunning,
				Phase: runstate.RunReady, RunID: "run-api",
			},
			{
				Service: "worker", Desired: runstate.DesiredRunning,
				Phase: runstate.RunFailed, RunID: "run-worker",
				LastError: &runstate.ErrorRecord{Code: "E_EXIT", Message: "exited"},
			},
		},
	}
}

type fakeController struct {
	snapshot operator.Snapshot
	result   operator.OperationResult
	err      error
	reader   runlog.Reader
	calls    []string
}

var _ operator.Controller = (*fakeController)(nil)

func (c *fakeController) Up(
	_ context.Context,
	request operator.UpRequest,
	sink operator.EventSink,
) (operator.OperationResult, error) {
	c.calls = append(c.calls, "up:"+strings.Join(request.Select.Services, ","))
	_ = sink.Send(context.Background(), operator.OperatorEvent{Kind: operator.EventOperationStarted})
	return c.result, c.err
}

func (c *fakeController) Down(
	_ context.Context,
	request operator.DownRequest,
	sink operator.EventSink,
) (operator.OperationResult, error) {
	c.calls = append(c.calls, "down:"+strings.Join(request.Select.Services, ","))
	_ = sink.Send(context.Background(), operator.OperatorEvent{Kind: operator.EventOperationStarted})
	return c.result, c.err
}

func (c *fakeController) Restart(
	_ context.Context,
	request operator.RestartRequest,
	sink operator.EventSink,
) (operator.OperationResult, error) {
	c.calls = append(c.calls, "restart:"+strings.Join(request.Select.Services, ","))
	_ = sink.Send(context.Background(), operator.OperatorEvent{Kind: operator.EventOperationStarted})
	return c.result, c.err
}

func (c *fakeController) Snapshot(
	context.Context,
	operator.SnapshotRequest,
) (operator.Snapshot, error) {
	return c.snapshot, c.err
}

func (c *fakeController) Logs() runlog.Reader {
	return c.reader
}

func (c *fakeController) Doctor(
	context.Context,
	operator.DoctorRequest,
) (operator.DoctorReport, error) {
	return operator.DoctorReport{}, c.err
}

type blockingReader struct{}

func (blockingReader) Query(context.Context, runlog.Query) ([]runlog.LogRecord, error) {
	return nil, nil
}

func (blockingReader) Follow(ctx context.Context, _ runlog.FollowRequest, _ runlog.LogSink) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestFollowCommandHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	controller := &fakeController{reader: blockingReader{}}
	model := NewModel(Options{Context: ctx, Controller: controller, RepoRoot: t.TempDir()})
	model.runIDs = []string{"run"}
	command := model.followOneCmd()
	require.NotNil(t, command)
	cancel()
	done := make(chan tea.Msg, 1)
	go func() { done <- command() }()
	select {
	case message := <-done:
		stopped, ok := message.(followStoppedMsg)
		require.True(t, ok)
		require.ErrorIs(t, stopped.Err, context.Canceled)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("follow command did not stop after cancellation")
	}
}

func TestOnlyOneFollowCommandCanBeInFlight(t *testing.T) {
	controller := &fakeController{reader: blockingReader{}}
	model := NewModel(Options{Context: t.Context(), Controller: controller, RepoRoot: t.TempDir()})
	model.runIDs = []string{"run"}
	require.NotNil(t, model.followOneCmd())
	require.Nil(t, model.followOneCmd())
	model.Update(followStoppedMsg{})
	require.NotNil(t, model.followOneCmd())
}

func TestOperationCommandCallsOnlyController(t *testing.T) {
	controller := &fakeController{
		result: operator.OperationResult{OperationID: "op", Kind: "down", Status: "succeeded"},
	}
	model := NewModel(Options{Context: t.Context(), Controller: controller, RepoRoot: t.TempDir()})
	command := model.operationCmd(confirmation{Kind: "down", Services: []string{"api"}})
	event, ok := command().(EventMsg)
	require.True(t, ok)
	require.Equal(t, operator.EventOperationStarted, event.Event.Kind)
	_, command = model.Update(event)
	require.NotNil(t, command)
	done, ok := command().(OperationDoneMsg)
	require.True(t, ok)
	require.NoError(t, done.Err)
	require.Equal(t, []string{"down:api"}, controller.calls)
}

func TestErrorValuesAreNotClassifiedByMessagePrefix(t *testing.T) {
	model := NewModel(Options{Context: t.Context(), Controller: &fakeController{}, RepoRoot: t.TempDir()})
	model.Update(ErrorMsg{Operation: "snapshot", Err: errors.New("action ok: actually failed")})
	require.Contains(t, model.status, "action ok: actually failed")
	require.Empty(t, model.runs.Entries)
}
