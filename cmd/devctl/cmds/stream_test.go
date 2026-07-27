package cmds

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestStreamCommandJSONLinesAndHumanOutput(t *testing.T) {
	repoRoot := t.TempDir()
	pluginPath := filepath.Join(findDevctlRootForTest(t), "testdata", "plugins", "stream", "plugin.py")
	configPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(
		"plugins:\n  - id: stream\n    path: python3\n    args:\n      - \""+pluginPath+"\"\n",
	), 0o600))

	stdout, stderr, err := runDevctlCommand(t,
		"stream", "start",
		"--repo-root", repoRoot,
		"--config", configPath,
		"--op", "stream.start",
		"--output", "json",
	)
	require.NoError(t, err, stderr)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 4)
	rows := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row), line)
		rows = append(rows, row)
	}
	require.Equal(t, "stream", rows[0]["kind"])
	require.Equal(t, "s1", rows[0]["stream_id"])
	require.Equal(t, "hello", rows[1]["message"])
	require.Equal(t, "world", rows[2]["message"])
	require.Equal(t, "end", rows[3]["event"])

	stdout, stderr, err = runDevctlCommand(t,
		"stream", "start",
		"--repo-root", repoRoot,
		"--config", configPath,
		"--op", "stream.start",
	)
	require.NoError(t, err, stderr)
	require.Equal(t,
		"plugin=stream op=stream.start stream_id=s1\n"+
			"[log level=info] hello\n"+
			"[log level=info] world\n"+
			"[end ok=true]\n",
		stdout,
	)
}

func runDevctlCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := &cobra.Command{Use: "devctl", SilenceUsage: true, SilenceErrors: true}
	require.NoError(t, AddCommands(root))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}
