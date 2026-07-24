package runstate

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/internal/testrepo"
	"github.com/stretchr/testify/require"
)

const testRunID = "01890f47-6c4e-7c5a-bb6d-6f2e9f4c0001"

func TestEnvironmentGoldenRoundTrip(t *testing.T) {
	repo := testrepo.New(t)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(repo.Root, WithClock(func() time.Time { return now }))
	require.NoError(t, err)

	err = store.CreateEnvironment(context.Background(), EnvironmentState{
		Profile: "development",
		Services: map[string]ServiceSlot{
			"web": {Name: "web", Desired: DesiredStopped},
			"api": {Name: "api", Desired: DesiredRunning},
		},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(store.StatePath())
	require.NoError(t, err)
	normalized := strings.ReplaceAll(string(data), filepath.ToSlash(repo.Root), "<REPO_ROOT>")
	golden, err := os.ReadFile("testdata/environment-v2.golden.json")
	require.NoError(t, err)
	require.Equal(t, string(golden), normalized)

	loaded, err := store.LoadEnvironment(context.Background())
	require.NoError(t, err)
	require.Equal(t, uint64(1), loaded.Revision)
	require.Equal(t, DesiredRunning, loaded.Services["api"].Desired)

	info, err := os.Stat(store.StatePath())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestUpdateChecksRevisionAndLeavesOriginalOnMutationError(t *testing.T) {
	repo := testrepo.New(t)
	store, err := NewStore(repo.Root)
	require.NoError(t, err)
	require.NoError(t, store.CreateEnvironment(context.Background(), EnvironmentState{}))

	err = store.Update(context.Background(), 99, func(state *EnvironmentState) error {
		state.Profile = "wrong"
		return nil
	})
	require.ErrorIs(t, err, ErrRevisionConflict)

	mutationErr := stderrors.New("mutation rejected")
	err = store.Update(context.Background(), 1, func(state *EnvironmentState) error {
		state.Profile = "not-persisted"
		return mutationErr
	})
	require.ErrorIs(t, err, mutationErr)

	loaded, err := store.LoadEnvironment(context.Background())
	require.NoError(t, err)
	require.Equal(t, uint64(1), loaded.Revision)
	require.Empty(t, loaded.Profile)
}

func TestUpdateIsAtomicForConcurrentReaders(t *testing.T) {
	repo := testrepo.New(t)
	store, err := NewStore(repo.Root)
	require.NoError(t, err)
	require.NoError(t, store.CreateEnvironment(context.Background(), EnvironmentState{}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var readers sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					state, loadErr := store.LoadEnvironment(ctx)
					if loadErr != nil && !stderrors.Is(loadErr, context.Canceled) {
						errs <- loadErr
						return
					}
					if state != nil && state.Revision == 0 {
						errs <- stderrors.New("reader observed zero revision")
						return
					}
				}
			}
		}()
	}

	for revision := uint64(1); revision <= 25; revision++ {
		err := store.Update(context.Background(), revision, func(state *EnvironmentState) error {
			state.Profile = "revision-" + strconv.FormatUint(revision+1, 10)
			return nil
		})
		require.NoError(t, err)
	}
	cancel()
	readers.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestCreateAndUpdateRun(t *testing.T) {
	repo := testrepo.New(t)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(repo.Root, WithClock(func() time.Time { return now }))
	require.NoError(t, err)

	run := RunRecord{
		RunID:   testRunID,
		Service: "api",
		Phase:   RunPlanned,
		Spec: ServiceSpecRecord{
			Name:        "api",
			Command:     []string{"go", "run", "./cmd/api"},
			Environment: map[string]string{"API_KEY": "[REDACTED]"},
		},
	}
	require.NoError(t, store.CreateRun(context.Background(), run))

	info, err := os.Stat(filepath.Join(repo.Root, ".devctl", "runs", testRunID))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	require.NoError(t, store.UpdateRun(context.Background(), testRunID, func(record *RunRecord) error {
		record.Phase = RunStarting
		return nil
	}))
	loaded, err := store.LoadRun(context.Background(), testRunID)
	require.NoError(t, err)
	require.Equal(t, RunStarting, loaded.Phase)
	require.Equal(t, filepath.ToSlash(filepath.Join(".devctl", "runs", testRunID)), loaded.ArtifactDir)
}

func TestPathsRejectEscapesAndInvalidIdentifiers(t *testing.T) {
	repo := testrepo.New(t)
	store, err := NewStore(repo.Root)
	require.NoError(t, err)

	_, err = store.RunDir("../../outside")
	require.ErrorIs(t, err, ErrInvalidPath)
	_, err = safeJoin(store.RunsDir(), "..", "..", "outside")
	require.ErrorIs(t, err, ErrInvalidPath)

	err = store.CreateEnvironment(context.Background(), EnvironmentState{
		Services: map[string]ServiceSlot{
			"../outside": {Name: "../outside", Desired: DesiredRunning},
		},
	})
	require.ErrorIs(t, err, ErrInvalidPath)
}
