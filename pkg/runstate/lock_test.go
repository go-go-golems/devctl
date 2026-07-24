package runstate

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/internal/testrepo"
	"github.com/stretchr/testify/require"
)

func TestRepositoryLockContentionHonorsContextAndReportsOwner(t *testing.T) {
	repo := testrepo.New(t)
	first, err := NewLocker(repo.Root)
	require.NoError(t, err)
	second, err := NewLocker(repo.Root)
	require.NoError(t, err)

	acquired := make(chan struct{})
	release := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- first.WithExclusive(context.Background(), LockMetadata{
			OperationID: "first-operation",
			Command:     []string{"devctl", "up"},
		}, func(context.Context) error {
			close(acquired)
			<-release
			return nil
		})
	}()
	<-acquired

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	err = second.WithExclusive(ctx, LockMetadata{
		OperationID: "second-operation",
		Command:     []string{"devctl", "down"},
	}, func(context.Context) error {
		return stderrors.New("second operation unexpectedly acquired lock")
	})
	require.ErrorIs(t, err, ErrOperationBusy)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	var busy *BusyError
	require.ErrorAs(t, err, &busy)
	require.Positive(t, busy.Owner.PID)
	require.Equal(t, "first-operation", busy.Owner.OperationID)
	require.Equal(t, []string{"devctl", "up"}, busy.Owner.Command)

	close(release)
	require.NoError(t, <-firstResult)
}
