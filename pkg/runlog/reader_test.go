package runlog

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/runstate"
)

type collectingSink struct {
	records []LogRecord
}

var _ LogSink = (*collectingSink)(nil)

func (s *collectingSink) Add(_ context.Context, record LogRecord) error {
	s.records = append(s.records, record)
	return nil
}

func TestFileReaderComposesFiltersAndTailsPerRun(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	first := "018f0f65-6c1a-7abc-8def-0123456789ab"
	second := "018f0f65-6c1a-7abc-8def-0123456789ac"
	base := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	createJournalRun(t, store, first, "api", []LogRecord{
		record(first, 1, base.Add(time.Second), "api", StreamStdout, "skip"),
		record(first, 2, base.Add(3*time.Second), "api", StreamStderr, "match-api"),
	})
	createJournalRun(t, store, second, "web", []LogRecord{
		record(second, 1, base.Add(2*time.Second), "web", StreamStderr, "match-web-old"),
		record(second, 2, base.Add(4*time.Second), "web", StreamStderr, "match-web-new"),
	})
	reader, err := NewFileReader(repoRoot)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	records, err := reader.Query(context.Background(), Query{
		Streams:  []StreamKind{StreamStderr},
		Contains: "match",
		Tail:     1,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %#v, want one per run", records)
	}
	if records[0].Text != "match-api" || records[1].Text != "match-web-new" {
		t.Fatalf("unexpected stable merge: %#v", records)
	}
	since := base.Add(2 * time.Second)
	until := base.Add(3 * time.Second)
	records, err = reader.Query(context.Background(), Query{
		Services: []string{"api"},
		Sources:  []SourceKind{SourceService},
		Streams:  []StreamKind{StreamStderr},
		Levels:   []string{"info"},
		Since:    &since,
		Until:    &until,
		Contains: "match",
	})
	if err != nil || len(records) != 1 || records[0].Text != "match-api" {
		t.Fatalf("composed filters: records=%#v err=%v", records, err)
	}
}

func TestFileReaderIgnoresTrailingCrashFragmentAndRejectsTerminatedCorruption(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	runID := "018f0f65-6c1a-7abc-8def-0123456789ab"
	createJournalRun(t, store, runID, "web", []LogRecord{
		record(runID, 1, time.Now().UTC(), "web", StreamStdout, "valid"),
	})
	runDir, _ := store.RunDir(runID)
	path := filepath.Join(runDir, JournalFileName)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	if _, err := file.WriteString(`{"version":`); err != nil {
		t.Fatalf("append fragment: %v", err)
	}
	_ = file.Close()
	reader, _ := NewFileReader(repoRoot)
	records, queryErr := reader.Query(context.Background(), Query{RunIDs: []string{runID}})
	var diagnostic *ReadError
	if len(records) != 1 || !errors.As(queryErr, &diagnostic) ||
		diagnostic.Code != CodeLogTrailingPartial {
		t.Fatalf("records=%#v err=%v", records, queryErr)
	}

	if err := os.WriteFile(path, []byte("{invalid}\n"), 0o600); err != nil {
		t.Fatalf("replace corrupt journal: %v", err)
	}
	_, queryErr = reader.Query(context.Background(), Query{RunIDs: []string{runID}})
	if !errors.As(queryErr, &diagnostic) || diagnostic.Code != CodeLogCorrupt {
		t.Fatalf("corrupt query error = %v", queryErr)
	}
}

func TestFollowResumesAfterCursorAndStopsAtTerminalRun(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	runID := "018f0f65-6c1a-7abc-8def-0123456789ab"
	now := time.Now().UTC()
	createJournalRun(t, store, runID, "web", []LogRecord{
		record(runID, 1, now, "web", StreamStdout, "old"),
		record(runID, 2, now.Add(time.Millisecond), "web", StreamStdout, "new"),
	})
	reader, _ := NewFileReader(repoRoot, WithPollInterval(time.Millisecond))
	sink := &collectingSink{}
	err = reader.Follow(context.Background(), FollowRequest{
		Query: Query{RunIDs: []string{runID}},
		After: map[string]Cursor{runID: {RunID: runID, Sequence: 1}},
	}, sink)
	if err != nil {
		t.Fatalf("follow: %v", err)
	}
	if len(sink.records) != 1 || sink.records[0].Sequence != 2 {
		t.Fatalf("follow records = %#v", sink.records)
	}
}

func TestFollowCancellationReturnsPromptly(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	runID := "018f0f65-6c1a-7abc-8def-0123456789ab"
	createJournalRunWithPhase(t, store, runID, "web", runstate.RunReady, nil)
	reader, _ := NewFileReader(repoRoot, WithPollInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- reader.Follow(ctx, FollowRequest{Query: Query{RunIDs: []string{runID}}}, &collectingSink{})
	}()
	start := time.Now()
	cancel()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
			t.Fatalf("follow cancellation took %s", elapsed)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("follow did not honor cancellation within 250ms")
	}
}

func TestFollowRejectsActiveJournalReplacement(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	runID := "018f0f65-6c1a-7abc-8def-0123456789ab"
	createJournalRunWithPhase(t, store, runID, "web", runstate.RunReady, nil)
	reader, _ := NewFileReader(repoRoot)
	identities, err := reader.journalIdentities([]string{runID})
	if err != nil {
		t.Fatalf("journal identities: %v", err)
	}
	runDir, _ := store.RunDir(runID)
	path := filepath.Join(runDir, JournalFileName)
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatalf("rename journal: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("replace journal: %v", err)
	}
	err = reader.verifyJournalIdentities([]string{runID}, identities)
	var readErr *ReadError
	if !errors.As(err, &readErr) || readErr.Code != CodeLogCorrupt {
		t.Fatalf("replacement error = %v", err)
	}
}

func createJournalRun(t *testing.T, store *runstate.Store, runID, service string, records []LogRecord) {
	t.Helper()
	createJournalRunWithPhase(t, store, runID, service, runstate.RunExited, records)
}

func createJournalRunWithPhase(
	t *testing.T,
	store *runstate.Store,
	runID string,
	service string,
	phase runstate.RunPhase,
	records []LogRecord,
) {
	t.Helper()
	if err := store.CreateRun(context.Background(), runstate.RunRecord{
		RunID: runID, Service: service, Phase: phase,
		Spec: runstate.ServiceSpecRecord{Name: service, Command: []string{"serve"}},
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	runDir, err := store.RunDir(runID)
	if err != nil {
		t.Fatalf("run dir: %v", err)
	}
	file, err := os.OpenFile(filepath.Join(runDir, JournalFileName), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create journal: %v", err)
	}
	encoder := json.NewEncoder(file)
	for _, logRecord := range records {
		if err := encoder.Encode(logRecord); err != nil {
			t.Fatalf("encode record: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close journal: %v", err)
	}
}

func record(
	runID string,
	sequence uint64,
	at time.Time,
	service string,
	stream StreamKind,
	text string,
) LogRecord {
	return LogRecord{
		Version: RecordVersion, RunID: runID, Sequence: sequence, Time: at,
		Source: SourceService, Service: service, Stream: stream, Level: "info", Text: text,
	}
}
