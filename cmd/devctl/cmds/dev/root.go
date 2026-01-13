package dev

import (
	"github.com/go-go-golems/devctl/cmd/devctl/cmds/dev/smoketest"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dev",
		Short:  "Developer tooling (dev-only)",
		Hidden: true,
	}

	cmd.AddCommand(smoketest.NewCmd())
	return cmd
}
