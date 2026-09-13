package cmds

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestCommandNamespaceRegistersNamesAndAliases(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	namespace := RootCommandNamespace(root)
	command := &cobra.Command{Use: "inspect", Aliases: []string{"show"}}
	require.NoError(t, namespace.Add(root, command))
	require.True(t, namespace.Snapshot()["inspect"])
	require.True(t, namespace.Snapshot()["show"])
	require.True(t, namespace.Snapshot()["help"])
	require.True(t, namespace.Snapshot()["completion"])

	require.Error(t, namespace.Add(root, &cobra.Command{Use: "other", Aliases: []string{"show"}}))
	require.Len(t, root.Commands(), 1)
}

func TestRootCommandNamespaceDiscoversRegisteredCommands(t *testing.T) {
	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddCommands(root))
	namespace := RootCommandNamespace(root).Snapshot()
	for _, name := range []string{"schema", "plugins", "stream", "__wrap-service", "help", "completion"} {
		require.Truef(t, namespace[name], "command %q was not reserved", name)
	}
}
