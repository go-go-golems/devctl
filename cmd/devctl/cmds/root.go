package cmds

import (
	"github.com/go-go-golems/devctl/cmd/devctl/cmds/dev"
	"github.com/spf13/cobra"
)

func AddCommands(root *cobra.Command) error {
	root.AddCommand(dev.NewCmd())
	root.AddCommand(newSchemaCmd())
	root.AddCommand(newTuiCmd())
	root.AddCommand(newWrapServiceCmd())

	constructors := []func() (*cobra.Command, error){
		newPlanCmd,
		newBuildCmd,
		newPrepareCmd,
		newValidateCmd,
		newPluginsCmd,
		newProfilesCmd,
		newUpCmd,
		newDownCmd,
		newStatusCmd,
		newLogsCmd,
		newDoctorCmd,
		newStreamCmd,
		newRestartCmd,
	}
	for _, construct := range constructors {
		command, err := construct()
		if err != nil {
			return err
		}
		root.AddCommand(command)
	}
	return nil
}
