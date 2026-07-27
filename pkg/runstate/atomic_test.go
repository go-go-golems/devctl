package runstate

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/devctl/internal/testrepo"
	"github.com/stretchr/testify/require"
)

func TestAtomicWriteFailuresBeforeRenamePreserveOldDocument(t *testing.T) {
	injected := stderrors.New("injected filesystem failure")

	tests := []struct {
		name  string
		hooks atomicWriteHooks
	}{
		{
			name: "write",
			hooks: atomicWriteHooks{
				write: func(_ *os.File, _ []byte) (int, error) {
					return 0, injected
				},
			},
		},
		{
			name: "short write",
			hooks: atomicWriteHooks{
				write: func(_ *os.File, contents []byte) (int, error) {
					return len(contents) - 1, nil
				},
			},
		},
		{
			name: "file sync",
			hooks: atomicWriteHooks{
				syncFile: func(_ *os.File) error {
					return injected
				},
			},
		},
		{
			name: "rename",
			hooks: atomicWriteHooks{
				rename: func(_, _ string) error {
					return injected
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := testrepo.New(t)
			path := filepath.Join(repo.DevctlDir, "artifact.json")
			require.NoError(t, WriteJSONAtomic(path, map[string]int{"revision": 1}, 0o600))

			err := writeJSONAtomic(path, map[string]int{"revision": 2}, 0o600, test.hooks)
			require.Error(t, err)

			data, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			require.JSONEq(t, `{"revision": 1}`, string(data))

			temporary, globErr := filepath.Glob(filepath.Join(repo.DevctlDir, ".artifact.json.tmp-*"))
			require.NoError(t, globErr)
			require.Empty(t, temporary)
		})
	}
}

func TestAtomicWriteReportsDirectorySyncFailureAfterReplacement(t *testing.T) {
	repo := testrepo.New(t)
	path := filepath.Join(repo.DevctlDir, "artifact.json")
	require.NoError(t, WriteJSONAtomic(path, map[string]int{"revision": 1}, 0o600))

	injected := stderrors.New("directory sync failed")
	err := writeJSONAtomic(path, map[string]int{"revision": 2}, 0o600, atomicWriteHooks{
		syncDir: func(_ *os.File) error {
			return injected
		},
	})
	require.ErrorIs(t, err, injected)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.JSONEq(t, `{"revision": 2}`, string(data))
}
