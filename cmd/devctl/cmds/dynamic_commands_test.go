package cmds

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestDynamicCommands_RegisterAndRun(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-dyncmd-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")

	cfg := []byte("plugins:\n  - id: cmd\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    priority: 10\n")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, cfg, 0o644))

	root := &cobra.Command{Use: "devctl"}
	AddRootFlags(root)

	err = AddDynamicPluginCommands(root, []string{
		"devctl",
		"--repo-root", repoRoot,
		"--config", cfgPath,
	})
	require.NoError(t, err)

	echoCmd, _, err := root.Find([]string{"echo"})
	require.NoError(t, err)
	require.NotNil(t, echoCmd)

	root.SetArgs([]string{"--repo-root", repoRoot, "--config", cfgPath, "--timeout", (2 * time.Second).String(), "echo", "hello"})
	require.NoError(t, root.Execute())
}

func TestDynamicCommands_SkipsWrapService(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-wrap-skip-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")

	cfg := []byte("plugins:\n  - id: cmd\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    priority: 10\n")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, cfg, 0o644))

	root := &cobra.Command{Use: "devctl"}
	AddRootFlags(root)

	err = AddDynamicPluginCommands(root, []string{
		"devctl",
		"--repo-root", repoRoot,
		"__wrap-service",
		"--service", "svc",
		"--cwd", repoRoot,
		"--stdout-log", filepath.Join(repoRoot, "stdout.log"),
		"--stderr-log", filepath.Join(repoRoot, "stderr.log"),
		"--exit-info", filepath.Join(repoRoot, "exit.json"),
		"--",
		"bash", "-lc", "true",
	})
	require.NoError(t, err)

	found := false
	for _, c := range root.Commands() {
		if c.Name() == "echo" {
			found = true
			break
		}
	}
	require.False(t, found)
}

func findDevctlRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}
