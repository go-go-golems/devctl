package runlog

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/pkg/errors"
)

const (
	CodeLogTrailingPartial = "E_LOG_TRAILING_PARTIAL"
	CodeLogCorrupt         = "E_LOG_CORRUPT"
)

type ReadError struct {
	Code   string
	RunID  string
	Path   string
	Offset int64
	cause  error
}

func (e *ReadError) Error() string {
	return fmt.Sprintf("%s: run %s at byte %d: %v", e.Code, e.RunID, e.Offset, e.cause)
}

func (e *ReadError) Unwrap() error {
	return e.cause
}

type FileReader struct {
	store        *runstate.Store
	pollInterval time.Duration
}

type FileReaderOption func(*FileReader)

func WithPollInterval(interval time.Duration) FileReaderOption {
	return func(reader *FileReader) {
		if interval > 0 {
			reader.pollInterval = interval
		}
	}
}

func NewFileReader(repoRoot string, options ...FileReaderOption) (*FileReader, error) {
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		return nil, err
	}
	reader := &FileReader{store: store, pollInterval: 50 * time.Millisecond}
	for _, option := range options {
		option(reader)
	}
	return reader, nil
}

var _ Reader = (*FileReader)(nil)

func (r *FileReader) Query(ctx context.Context, query Query) ([]LogRecord, error) {
	runIDs, err := r.resolveRunIDs(ctx, query.RunIDs)
	if err != nil {
		return nil, err
	}
	combined := make([]LogRecord, 0)
	var trailingError error
	for _, runID := range runIDs {
		records, readErr := r.readRun(ctx, runID)
		var diagnostic *ReadError
		if readErr != nil {
			if !errors.As(readErr, &diagnostic) || diagnostic.Code != CodeLogTrailingPartial {
				return nil, readErr
			}
			trailingError = readErr
		}
		filtered := filterRecords(records, query)
		if query.Tail > 0 && len(filtered) > query.Tail {
			filtered = filtered[len(filtered)-query.Tail:]
		}
		combined = append(combined, filtered...)
	}
	sort.Slice(combined, func(left, right int) bool {
		if !combined[left].Time.Equal(combined[right].Time) {
			return combined[left].Time.Before(combined[right].Time)
		}
		if combined[left].RunID != combined[right].RunID {
			return combined[left].RunID < combined[right].RunID
		}
		return combined[left].Sequence < combined[right].Sequence
	})
	return combined, trailingError
}

func (r *FileReader) readRun(ctx context.Context, runID string) ([]LogRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runDir, err := r.store.RunDir(runID)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(runDir, JournalFileName)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogRecord{}, nil
		}
		return nil, errors.Wrap(err, "open run log journal")
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReaderSize(file, 64*1024)
	records := make([]LogRecord, 0)
	var offset int64
	var previous uint64
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineOffset := offset
			offset += int64(len(line))
			if line[len(line)-1] != '\n' {
				return records, &ReadError{
					Code: CodeLogTrailingPartial, RunID: runID, Path: path,
					Offset: lineOffset, cause: errors.New("ignored unterminated final journal record"),
				}
			}
			line = bytes.TrimSuffix(line, []byte{'\n'})
			var record LogRecord
			if err := json.Unmarshal(line, &record); err != nil {
				return nil, &ReadError{
					Code: CodeLogCorrupt, RunID: runID, Path: path,
					Offset: lineOffset, cause: errors.Wrap(err, "decode terminated journal record"),
				}
			}
			if record.Version != RecordVersion || record.RunID != runID || record.Sequence <= previous {
				return nil, &ReadError{
					Code: CodeLogCorrupt, RunID: runID, Path: path,
					Offset: lineOffset, cause: errors.New("record version, run ID, or sequence is invalid"),
				}
			}
			previous = record.Sequence
			records = append(records, record)
		}
		if readErr != nil {
			if readErr == io.EOF {
				return records, nil
			}
			return nil, errors.Wrap(readErr, "read run log journal")
		}
	}
}

func (r *FileReader) resolveRunIDs(ctx context.Context, requested []string) ([]string, error) {
	if len(requested) > 0 {
		runIDs := append([]string{}, requested...)
		sort.Strings(runIDs)
		return uniqueStrings(runIDs), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(r.store.RunsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, errors.Wrap(err, "list run log directories")
	}
	runIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			runIDs = append(runIDs, entry.Name())
		}
	}
	sort.Strings(runIDs)
	return runIDs, nil
}

func filterRecords(records []LogRecord, query Query) []LogRecord {
	services := stringSet(query.Services)
	sources := sourceSet(query.Sources)
	streams := streamSet(query.Streams)
	levels := stringSet(query.Levels)
	filtered := make([]LogRecord, 0, len(records))
	for _, record := range records {
		if len(services) > 0 && !services[record.Service] {
			continue
		}
		if len(sources) > 0 && !sources[record.Source] {
			continue
		}
		if len(streams) > 0 && !streams[record.Stream] {
			continue
		}
		if len(levels) > 0 && !levels[record.Level] {
			continue
		}
		if query.Since != nil && record.Time.Before(*query.Since) {
			continue
		}
		if query.Until != nil && record.Time.After(*query.Until) {
			continue
		}
		if query.Contains != "" && !strings.Contains(record.Text, query.Contains) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sourceSet(values []SourceKind) map[SourceKind]bool {
	result := make(map[SourceKind]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func streamSet(values []StreamKind) map[StreamKind]bool {
	result := make(map[StreamKind]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
