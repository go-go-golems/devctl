package runstate

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrOperationBusy   = stderrors.New("runstate operation busy")
	ErrLockUnsupported = stderrors.New("runstate repository lock unsupported")
)

type LockMetadata struct {
	PID         int              `json:"pid"`
	Identity    *ProcessIdentity `json:"identity,omitempty"`
	OperationID string           `json:"operation_id"`
	Command     []string         `json:"command"`
	AcquiredAt  time.Time        `json:"acquired_at"`
}

type BusyError struct {
	Owner LockMetadata
	Cause error
}

func (e *BusyError) Error() string {
	if e.Owner.PID > 0 {
		return "runstate operation busy: lock held by PID " + strconv.Itoa(e.Owner.PID)
	}
	return "runstate operation busy"
}

func (e *BusyError) Unwrap() []error {
	return []error{ErrOperationBusy, e.Cause}
}

type Locker interface {
	WithExclusive(
		ctx context.Context,
		metadata LockMetadata,
		fn func(context.Context) error,
	) error
}

type RepositoryLocker struct {
	path         string
	pollInterval time.Duration
	now          func() time.Time
}

var _ Locker = (*RepositoryLocker)(nil)

func NewLocker(repoRoot string) (*RepositoryLocker, error) {
	store, err := NewStore(repoRoot)
	if err != nil {
		return nil, err
	}
	return &RepositoryLocker{
		path:         filepath.Join(store.repoRoot, stateDirName, "lock"),
		pollInterval: 25 * time.Millisecond,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

func (l *RepositoryLocker) WithExclusive(
	ctx context.Context,
	metadata LockMetadata,
	fn func(context.Context) error,
) error {
	if fn == nil {
		return errors.New("repository lock: nil operation")
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return errors.Wrap(err, "create repository lock directory")
	}
	if metadata.PID == 0 {
		metadata.PID = os.Getpid()
	}
	if metadata.Identity == nil {
		identity, err := ReadProcessIdentity(metadata.PID)
		if err != nil && !stderrors.Is(err, ErrProcessIdentityUnsupported) {
			return errors.Wrap(err, "identify repository lock owner")
		}
		metadata.Identity = identity
	}
	if metadata.AcquiredAt.IsZero() {
		metadata.AcquiredAt = l.now()
	}
	return withFileLock(ctx, l.path, l.pollInterval, metadata, fn)
}

func readLockMetadata(path string) LockMetadata {
	data, err := os.ReadFile(path)
	if err != nil {
		return LockMetadata{}
	}
	var metadata LockMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return LockMetadata{}
	}
	return metadata
}

func writeLockMetadata(file *os.File, metadata LockMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal repository lock metadata")
	}
	data = append(data, '\n')
	if err := file.Truncate(0); err != nil {
		return errors.Wrap(err, "truncate repository lock metadata")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return errors.Wrap(err, "seek repository lock metadata")
	}
	n, err := file.Write(data)
	if err != nil {
		return errors.Wrap(err, "write repository lock metadata")
	}
	if n != len(data) {
		return errors.New("write repository lock metadata: short write")
	}
	if err := file.Sync(); err != nil {
		return errors.Wrap(err, "sync repository lock metadata")
	}
	return nil
}
