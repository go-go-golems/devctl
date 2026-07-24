package cmds

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestProfilesActiveExplicitDefaultAndNoProfile(t *testing.T) {
	repoRoot := t.TempDir()
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
profiles:
  default:
    plugins: [api]
plugins:
  - id: api
    path: devctl-api
`), 0o644))

	rows := executeProfilesCommand(t, "profiles", "active", "--repo-root", repoRoot)
	require.Equal(t, "", rows[0]["profile"])
	require.Equal(t, false, rows[0]["configured"])

	rows = executeProfilesCommand(t, "profiles", "active", "--repo-root", repoRoot, "--profile", "default")
	require.Equal(t, "default", rows[0]["profile"])
	require.Equal(t, true, rows[0]["configured"])
}

func TestProfilesListIncludesOverrideProfileAndActiveMarker(t *testing.T) {
	repoRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, ".devctl.yaml"), []byte(`
profiles:
  shared:
    display_name: Shared
    plugins: [api]
plugins:
  - id: api
    path: devctl-api
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, ".devctl.override.yaml"), []byte(`
profile:
  active: local
profiles:
  local:
    display_name: Local
    description: Local override profile
    plugins: [api]
`), 0o644))

	rows := executeProfilesCommand(t, "profiles", "list", "--repo-root", repoRoot)
	require.Len(t, rows, 2)
	require.Equal(t, "local", rows[0]["profile"])
	require.Equal(t, true, rows[0]["active"])
	require.Equal(t, "Local override profile", rows[0]["description"])
	require.Equal(t, "shared", rows[1]["profile"])
	require.Equal(t, false, rows[1]["active"])
}

func executeProfilesCommand(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddCommands(root))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append(args, "--output", "json"))
	require.NoError(t, root.Execute())
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &rows))
	return rows
}
