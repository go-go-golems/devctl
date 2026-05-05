package cmds

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

	out := executeProfilesCommand(t, "profiles", "active", "--repo-root", repoRoot)
	require.Equal(t, "(none)\n", out)

	out = executeProfilesCommand(t, "profiles", "active", "--repo-root", repoRoot, "--profile", "default")
	require.Equal(t, "default\n", out)
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

	out := executeProfilesCommand(t, "profiles", "list", "--repo-root", repoRoot)
	require.Contains(t, out, "local")
	require.Contains(t, out, "Local override profile")
	require.Contains(t, out, "shared")

	localLine := findLineContaining(out, "local")
	require.Contains(t, localLine, "*")
}

func executeProfilesCommand(t *testing.T, args ...string) string {
	t.Helper()
	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddCommands(root))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	return out.String()
}

func findLineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
