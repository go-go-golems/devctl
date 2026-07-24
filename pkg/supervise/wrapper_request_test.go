package supervise

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/internal/testrepo"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/stretchr/testify/require"
)

const wrapperRequestTestRunID = "01890f47-6c4e-7c5a-bb6d-6f2e9f4c0004"

func TestWrapperRequestRoundTripIsPrivateAndPathBound(t *testing.T) {
	repo := testrepo.New(t)
	store := createWrapperRequestTestRun(t, repo.Root)
	request, err := NewWrapperRequest(
		store,
		wrapperRequestTestRunID,
		"api",
		repo.Root,
		[]string{"sleep", "30"},
		map[string]string{"API_KEY": "secret-value"},
	)
	require.NoError(t, err)
	require.NoError(t, WriteWrapperRequest(request))

	info, err := os.Stat(request.RequestPath())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	loaded, err := LoadWrapperRequest(request.RequestPath())
	require.NoError(t, err)
	require.Equal(t, request, loaded)
	require.Equal(t, "secret-value", loaded.Environment["API_KEY"])

	data, err := os.ReadFile(request.RequestPath())
	require.NoError(t, err)
	wrongPath := filepath.Join(repo.DevctlDir, "request.json")
	require.NoError(t, os.WriteFile(wrongPath, data, 0o600))
	_, err = LoadWrapperRequest(wrongPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match")
}

func TestWaitWrapperHandshakeDistinguishesMissingOwnerAndReady(t *testing.T) {
	repo := testrepo.New(t)
	store := createWrapperRequestTestRun(t, repo.Root)
	request, err := NewWrapperRequest(store, wrapperRequestTestRunID, "api", repo.Root, []string{"sleep", "30"}, nil)
	require.NoError(t, err)
	identity, err := runstate.ReadProcessIdentity(os.Getpid())
	require.NoError(t, err)

	ownerContext, ownerCancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer ownerCancel()
	_, _, err = waitWrapperHandshake(ownerContext, request, identity)
	require.ErrorIs(t, err, ErrOwnerRecordMissing)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	require.NoError(t, WriteOwnerRecord(request.OwnerPath(), OwnerRecord{
		Version:   HandshakeVersion,
		RunID:     request.RunID,
		Service:   request.Service,
		Wrapper:   *identity,
		WrittenAt: time.Now().UTC(),
	}))
	readyContext, readyCancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer readyCancel()
	_, _, err = waitWrapperHandshake(readyContext, request, identity)
	require.ErrorIs(t, err, ErrReadyRecordMissing)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func createWrapperRequestTestRun(t *testing.T, repoRoot string) *runstate.Store {
	t.Helper()
	store, err := runstate.NewStore(repoRoot)
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(context.Background(), runstate.RunRecord{
		RunID:   wrapperRequestTestRunID,
		Service: "api",
		Phase:   runstate.RunPlanned,
		Spec: runstate.ServiceSpecRecord{
			Name:    "api",
			Command: []string{"sleep", "30"},
		},
	}))
	return store
}
