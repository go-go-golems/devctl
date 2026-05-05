package cmds

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/spf13/cobra"
)

func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "List and inspect devctl profiles",
	}
	cmd.AddCommand(newProfilesListCmd())
	cmd.AddCommand(newProfilesActiveCmd())
	return cmd
}

func newProfilesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List profiles from .devctl.yaml and .devctl.override.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadStacked(opts.Config, config.DefaultOverridePath(opts.RepoRoot))
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Profiles))
			for name := range cfg.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			active := cfg.ResolveProfile(opts.Profile)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "PROFILE\tACTIVE\tDISPLAY NAME\tDESCRIPTION\tPLUGINS")
			for _, name := range names {
				profile := cfg.Profiles[name]
				activeMarker := ""
				if name == active {
					activeMarker = "*"
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					name,
					activeMarker,
					profile.DisplayName,
					profile.Description,
					strings.Join(profile.Plugins, ","),
				)
			}
			return tw.Flush()
		},
	}
	AddRepoFlags(cmd)
	return cmd
}

func newProfilesActiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "Show the resolved active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadStacked(opts.Config, config.DefaultOverridePath(opts.RepoRoot))
			if err != nil {
				return err
			}
			active := cfg.ResolveProfile(opts.Profile)
			if active == "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(none)")
				return nil
			}
			if cfg.GetProfile(active) == nil {
				return fmt.Errorf("profile %q not found", active)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), active)
			return nil
		},
	}
	AddRepoFlags(cmd)
	return cmd
}
