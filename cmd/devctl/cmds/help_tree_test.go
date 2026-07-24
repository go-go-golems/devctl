package cmds

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestPublicHelpCommandTreeGolden(t *testing.T) {
	root := &cobra.Command{Use: "devctl"}
	require.NoError(t, AddCommands(root))
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()

	const golden = `build	Run the build phase (config.mutate + build.run)
completion	Generate the autocompletion script for the specified shell
  bash	Generate the autocompletion script for bash
  fish	Generate the autocompletion script for fish
  powershell	Generate the autocompletion script for powershell
  zsh	Generate the autocompletion script for zsh
doctor	Inspect configuration, durable state, ownership, and log health
down	Stop all or selected services
help	Help about any command
logs	Query or follow structured service logs
plan	Compute a merged launch plan from selected plugins
plugins	Inspect plugins and manage the dynamic command catalog
  commands	List validated dynamic root commands
  inspect	Inspect one selected plugin and its catalog commands
  list	List selected configured plugins without starting them
  refresh	Start selected providers and refresh the command catalog
  run	Run a catalog command through an explicit provider
prepare	Run the prepare phase (config.mutate + prepare.run)
profiles	List and inspect devctl profiles
  active	Show the resolved active profile
  list	List profiles from repository configuration
restart	Restart all or selected services
status	Show durable service status
stream	Start and inspect protocol streams
  start	Start a stream operation and emit its events
tui	Interactive terminal UI for devctl
up	Start all or selected services
validate	Run the validation phase (config.mutate + validate.run)`

	require.Equal(t, golden, strings.Join(publicHelpTree(root, 0), "\n"))
}

func publicHelpTree(command *cobra.Command, depth int) []string {
	lines := make([]string, 0)
	for _, child := range command.Commands() {
		if child.Hidden {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"%s%s\t%s", strings.Repeat("  ", depth), child.Name(), child.Short,
		))
		lines = append(lines, publicHelpTree(child, depth+1)...)
	}
	return lines
}
