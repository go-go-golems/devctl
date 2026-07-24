package testrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCreatesIsolatedPrivateDevctlDirectory(t *testing.T) {
	repo := New(t)

	require.NotEqual(t, ".", repo.Root)
	require.True(t, filepath.IsAbs(repo.Root))
	require.Equal(t, filepath.Join(repo.Root, ".devctl"), repo.DevctlDir)

	info, err := os.Stat(repo.DevctlDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestWriteConfigUsesRepositoryRoot(t *testing.T) {
	repo := New(t)
	require.NoError(t, repo.WriteConfig([]byte("plugins: []\n")))

	got, err := os.ReadFile(repo.ConfigPath)
	require.NoError(t, err)
	require.Equal(t, "plugins: []\n", string(got))
	require.Equal(t, repo.Root, filepath.Dir(repo.ConfigPath))
}
