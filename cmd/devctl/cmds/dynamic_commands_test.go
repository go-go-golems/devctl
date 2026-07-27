package cmds

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/plugincatalog"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	devruntime "github.com/go-go-golems/devctl/pkg/runtime"
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
	refreshDynamicCatalogForTest(t, repoRoot, cfgPath, "")

	root := &cobra.Command{Use: "devctl"}

	err = AddDynamicPluginCommands(root, []string{
		"devctl",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"echo",
	})
	require.NoError(t, err)

	echoCmd, _, err := root.Find([]string{"echo"})
	require.NoError(t, err)
	require.NotNil(t, echoCmd)

	root.SetArgs([]string{"echo", "--repo-root", repoRoot, "--config", cfgPath, "--timeout", (2 * time.Second).String(), "hello"})
	require.NoError(t, root.Execute())
}

func TestDynamicCommands_SkipsBuiltIns(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-dyncmd-skip-builtins-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")

	cfg := []byte("plugins:\n  - id: cmd\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    priority: 10\n")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, cfg, 0o644))

	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddCommands(root))

	// If we are invoking a built-in command (like `status`), dynamic command discovery should be skipped.
	err = AddDynamicPluginCommands(root, []string{
		"devctl",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"status",
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

func TestDynamicCommands_RespectsProfileFiltering(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "devctl-dyncmd-profile-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(repoRoot) }()

	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")

	cfg := []byte("profiles:\n  commands:\n    plugins: [cmd]\n  empty:\n    plugins: []\nplugins:\n  - id: cmd\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    priority: 10\n")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, cfg, 0o644))

	root := &cobra.Command{Use: "devctl"}
	err = AddDynamicPluginCommands(root, []string{
		"devctl",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--profile", "empty",
		"echo",
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

	root = &cobra.Command{Use: "devctl"}
	refreshDynamicCatalogForTest(t, repoRoot, cfgPath, "commands")
	err = AddDynamicPluginCommands(root, []string{
		"devctl",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--profile", "commands",
		"echo",
	})
	require.NoError(t, err)

	echoCmd, _, err := root.Find([]string{"echo"})
	require.NoError(t, err)
	require.NotNil(t, echoCmd)
}

func TestDynamicCommands_MissingCatalogDoesNotStartPlugins(t *testing.T) {
	repoRoot := t.TempDir()
	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"plugins:\n  - id: cmd\n    path: python3\n    args:\n      - \""+plugin+"\"\n",
	), 0o600))
	root := &cobra.Command{Use: "devctl"}
	err := AddDynamicPluginCommands(root, []string{
		"devctl", "--repo-root", repoRoot, "--config", cfgPath, "echo",
	})
	require.ErrorIs(t, err, plugincatalog.ErrCatalogMissing)
	_, statErr := os.Stat(plugincatalog.CachePath(repoRoot))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDynamicCommands_StaticDeclarationNeedsNoRefresh(t *testing.T) {
	repoRoot := t.TempDir()
	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"plugins:\n  - id: cmd\n    path: python3\n    args:\n      - \""+plugin+"\"\n    commands:\n      - name: echo\n        help: Echo values\n",
	), 0o600))
	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddDynamicPluginCommands(root, []string{
		"devctl", "--repo-root", repoRoot, "--config", cfgPath, "echo",
	}))
	command, _, err := root.Find([]string{"echo"})
	require.NoError(t, err)
	require.Equal(t, "echo", command.Name())
}

func TestDynamicCommands_RegisterStaticDeclarationsForHelp(t *testing.T) {
	repoRoot := t.TempDir()
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"plugins:\n"+
			"  - id: cmd\n"+
			"    path: missing-provider\n"+
			"    commands:\n"+
			"      - name: echo\n"+
			"        help: Echo values\n",
	), 0o600))
	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddCommands(root))

	require.NoError(t, AddDynamicPluginCommands(root, []string{
		"devctl", "--repo-root", repoRoot, "--config", cfgPath, "--help",
	}))

	command, _, err := root.Find([]string{"echo"})
	require.NoError(t, err)
	require.Equal(t, "echo", command.Name())
	require.Equal(t, "Echo values", command.Short)
}

func TestDynamicCommands_RegisterCachedCommandsForCompletion(t *testing.T) {
	repoRoot := t.TempDir()
	devctlRoot := findDevctlRootForTest(t)
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "command", "plugin.py")
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"plugins:\n  - id: cmd\n    path: python3\n    args:\n      - \""+plugin+"\"\n",
	), 0o600))
	refreshDynamicCatalogForTest(t, repoRoot, cfgPath, "")
	root := &cobra.Command{Use: "devctl"}
	root.AddCommand(&cobra.Command{Use: "completion"})

	require.NoError(t, AddDynamicPluginCommands(root, []string{
		"devctl", "--repo-root", repoRoot, "--config", cfgPath, "completion", "bash",
	}))

	command, _, err := root.Find([]string{"echo"})
	require.NoError(t, err)
	require.Equal(t, "echo", command.Name())
}

func TestDynamicCommands_UnrelatedCatalogConflictDoesNotDisableCommand(t *testing.T) {
	repoRoot := t.TempDir()
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"plugins:\n"+
			"  - id: first\n"+
			"    path: /bin/true\n"+
			"    commands:\n"+
			"      - name: echo\n"+
			"        help: First echo\n"+
			"      - name: seed\n"+
			"        help: Seed data\n"+
			"  - id: second\n"+
			"    path: /bin/true\n"+
			"    commands:\n"+
			"      - name: echo\n"+
			"        help: Second echo\n",
	), 0o600))
	repo, err := repository.Load(repository.Options{
		RepoRoot: repoRoot, ConfigPath: cfgPath, Cwd: repoRoot,
	})
	require.NoError(t, err)
	_, err = plugincatalog.Refresh(t.Context(), repo, plugincatalog.RefreshOptions{
		Reserved: defaultReservedCommandNames(),
	})
	require.ErrorIs(t, err, plugincatalog.ErrCatalogConflict)

	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddDynamicPluginCommands(root, []string{
		"devctl", "--repo-root", repoRoot, "--config", cfgPath, "seed",
	}))
	command, _, err := root.Find([]string{"seed"})
	require.NoError(t, err)
	require.Equal(t, "seed", command.Name())

	root = &cobra.Command{Use: "devctl"}
	err = AddDynamicPluginCommands(root, []string{
		"devctl", "--repo-root", repoRoot, "--config", cfgPath, "echo",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PLUGIN_COMMAND_CONFLICT")
}

func refreshDynamicCatalogForTest(
	t *testing.T,
	repoRoot string,
	configPath string,
	profile string,
) {
	t.Helper()
	repo, err := repository.Load(repository.Options{
		RepoRoot: repoRoot, ConfigPath: configPath, ProfileName: profile, Cwd: repoRoot,
	})
	require.NoError(t, err)
	_, err = plugincatalog.Refresh(t.Context(), repo, plugincatalog.RefreshOptions{
		Reserved: defaultReservedCommandNames(),
	})
	require.NoError(t, err)
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
	_, thisFile, _, ok := goruntime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}

func TestValidateRuntimeCatalogRejectsHandshakeDrift(t *testing.T) {
	entry := plugincatalog.CommandEntry{
		Name: "echo", Help: "Echo", ProviderID: "provider", PluginName: "plugin",
	}
	catalog := &plugincatalog.Catalog{
		Commands:  map[string]plugincatalog.CommandEntry{"echo": entry},
		Conflicts: map[string][]plugincatalog.CommandEntry{},
	}
	client := &catalogTestClient{handshake: protocol.Handshake{
		PluginName: "plugin",
		Capabilities: protocol.Capabilities{
			Ops: []string{"command.run"},
			Commands: []protocol.CommandSpec{
				{Name: "echo", Help: "Changed help"},
			},
		},
	}}
	err := validateRuntimeCatalog(client, catalog, entry)
	require.Error(t, err)
	require.Contains(t, err.Error(), "PLUGIN_CATALOG_STALE")
}

type catalogTestClient struct {
	handshake protocol.Handshake
}

var _ devruntime.Client = (*catalogTestClient)(nil)

func (c *catalogTestClient) Spec() devruntime.PluginSpec {
	return devruntime.PluginSpec{ID: "provider"}
}

func (c *catalogTestClient) Handshake() protocol.Handshake {
	return c.handshake
}

func (c *catalogTestClient) SupportsOp(operation string) bool {
	for _, supported := range c.handshake.Capabilities.Ops {
		if supported == operation {
			return true
		}
	}
	return false
}

func (c *catalogTestClient) Call(context.Context, string, any, any) error {
	return nil
}

func (c *catalogTestClient) StartStream(
	context.Context,
	string,
	any,
) (string, <-chan protocol.Event, error) {
	return "", nil, nil
}

func (c *catalogTestClient) Close(context.Context) error {
	return nil
}
