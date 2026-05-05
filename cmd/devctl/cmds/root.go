package cmds

import (
	"github.com/go-go-golems/devctl/cmd/devctl/cmds/dev"
	"github.com/spf13/cobra"
)

func AddCommands(root *cobra.Command) error {
	root.AddCommand(dev.NewCmd())
	root.AddCommand(newPlanCmd())
	root.AddCommand(newPluginsCmd())
	root.AddCommand(newProfilesCmd())

	root.AddCommand(newUpCmd())
	root.AddCommand(newDownCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newLogsCmd())
	root.AddCommand(newStreamCmd())
	root.AddCommand(newTuiCmd())
	root.AddCommand(newWrapServiceCmd())
	root.AddCommand(newRestartCmd())
	root.AddCommand(newStopServiceCmd())
	root.AddCommand(newStartServiceCmd())
	return nil
}
