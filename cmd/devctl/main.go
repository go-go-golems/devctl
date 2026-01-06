package main

import (
	"os"

	"github.com/go-go-golems/devctl/cmd/devctl/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "devctl",
	Short:   "devctl is a dev environment orchestrator",
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	},
}

func main() {
	cobra.CheckErr(logging.AddLoggingLayerToRootCommand(rootCmd, "devctl"))
	cmds.AddRootFlags(rootCmd)
	cobra.CheckErr(cmds.AddCommands(rootCmd))
	cobra.CheckErr(cmds.AddDynamicPluginCommands(rootCmd, os.Args))
	cobra.CheckErr(rootCmd.Execute())
}
