package runlog

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/pkg/errors"
)

func (r *FileReader) Follow(ctx context.Context, request FollowRequest, sink LogSink) error {
	if sink == nil {
		return errors.New("follow requires a log sink")
	}
	runIDs, err := r.resolveRunIDs(ctx, request.Query.RunIDs)
	if err != nil {
		return err
	}
	request.Query.RunIDs = runIDs
	request.Query.Tail = 0
	cursors := make(map[string]uint64, len(runIDs))
	for _, runID := range runIDs {
		if cursor, exists := request.After[runID]; exists {
			if cursor.RunID != "" && cursor.RunID != runID {
				return errors.Errorf("cursor run %q does not match %q", cursor.RunID, runID)
			}
			cursors[runID] = cursor.Sequence
		}
	}
	identities, err := r.journalIdentities(runIDs)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if err := r.verifyJournalIdentities(runIDs, identities); err != nil {
			return err
		}
		records, queryErr := r.Query(ctx, request.Query)
		var diagnostic *ReadError
		if queryErr != nil && (!errors.As(queryErr, &diagnostic) || diagnostic.Code != CodeLogTrailingPartial) {
			return queryErr
		}
		emitted := false
		for _, record := range records {
			if record.Sequence <= cursors[record.RunID] {
				continue
			}
			if err := sink.Add(ctx, record); err != nil {
				return err
			}
			cursors[record.RunID] = record.Sequence
			emitted = true
		}
		terminal, err := r.allRunsTerminal(ctx, runIDs)
		if err != nil {
			return err
		}
		if terminal && !emitted && queryErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *FileReader) journalIdentities(runIDs []string) (map[string]os.FileInfo, error) {
	identities := make(map[string]os.FileInfo, len(runIDs))
	for _, runID := range runIDs {
		runDir, err := r.store.RunDir(runID)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(filepath.Join(runDir, JournalFileName))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		identities[runID] = info
	}
	return identities, nil
}

func (r *FileReader) verifyJournalIdentities(runIDs []string, identities map[string]os.FileInfo) error {
	for _, runID := range runIDs {
		runDir, err := r.store.RunDir(runID)
		if err != nil {
			return err
		}
		info, err := os.Stat(filepath.Join(runDir, JournalFileName))
		original, existed := identities[runID]
		switch {
		case os.IsNotExist(err) && !existed:
			continue
		case err != nil:
			return err
		case !existed:
			identities[runID] = info
		case !os.SameFile(original, info):
			return &ReadError{
				Code: CodeLogCorrupt, RunID: runID,
				Path:  filepath.Join(runDir, JournalFileName),
				cause: errors.New("active run journal was replaced"),
			}
		}
	}
	return nil
}

func (r *FileReader) allRunsTerminal(ctx context.Context, runIDs []string) (bool, error) {
	for _, runID := range runIDs {
		run, err := r.store.LoadRun(ctx, runID)
		if err != nil {
			return false, err
		}
		if run.Phase != runstate.RunExited && run.Phase != runstate.RunFailed {
			return false, nil
		}
	}
	return len(runIDs) > 0, nil
}
