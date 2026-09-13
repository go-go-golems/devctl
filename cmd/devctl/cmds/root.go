package cmds

import (
	"github.com/go-go-golems/devctl/cmd/devctl/cmds/dev"
	"github.com/spf13/cobra"
)

func AddCommands(root *cobra.Command) error {
	namespace := RootCommandNamespace(root)
	for _, command := range []*cobra.Command{
		dev.NewCmd(),
		newSchemaCmd(),
		newTuiCmd(),
		newWrapServiceCmd(),
	} {
		if err := namespace.Add(root, command); err != nil {
			return err
		}
	}

	constructors := []func() (*cobra.Command, error){
		newPlanCmd,
		newBuildCmd,
		newPrepareCmd,
		newValidateCmd,
		func() (*cobra.Command, error) { return newPluginsCmd(namespace) },
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
		if err := namespace.Add(root, command); err != nil {
			return err
		}
	}
	return nil
}
