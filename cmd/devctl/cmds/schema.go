package cmds

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandSchema struct {
	Path  string       `json:"path"`
	Use   string       `json:"use"`
	Short string       `json:"short"`
	Flags []flagSchema `json:"flags"`
}

type flagSchema struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Usage     string `json:"usage"`
	Default   string `json:"default,omitempty"`
	Required  bool   `json:"required"`
	Inherited bool   `json:"inherited"`
}

func newSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema [COMMAND...]",
		Short: "Print a compact JSON schema for a command",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			target := command.Root()
			if len(args) > 0 {
				found, remaining, err := command.Root().Find(args)
				if err != nil {
					return err
				}
				if len(remaining) > 0 {
					return errors.Errorf("unknown command path %q", remaining)
				}
				target = found
			}
			schema := commandSchema{Path: target.CommandPath(), Use: target.Use, Short: target.Short}
			addFlags := func(flags *pflag.FlagSet, inherited bool) {
				flags.VisitAll(func(flag *pflag.Flag) {
					annotation := target.Flag(flag.Name)
					required := annotation != nil && len(annotation.Annotations[cobra.BashCompOneRequiredFlag]) > 0
					schema.Flags = append(schema.Flags, flagSchema{
						Name: flag.Name, Shorthand: flag.Shorthand, Type: flag.Value.Type(), Usage: flag.Usage,
						Default: flag.DefValue, Required: required, Inherited: inherited,
					})
				})
			}
			addFlags(target.NonInheritedFlags(), false)
			addFlags(target.InheritedFlags(), true)
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(schema)
		},
	}
}
