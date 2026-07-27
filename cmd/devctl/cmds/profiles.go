package cmds

import (
	"context"
	"sort"

	"github.com/go-go-golems/devctl/pkg/config"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newProfilesCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "profiles",
		Short: "List and inspect devctl profiles",
	}
	command.AddCommand(buildProfilesSubcommand("list"))
	command.AddCommand(buildProfilesSubcommand("active"))
	return command
}

type ProfilesCommand struct {
	*glazedcmds.CommandDescription
	kind string
}

var _ glazedcmds.GlazeCommand = (*ProfilesCommand)(nil)

func NewProfilesCommand(kind string) (*ProfilesCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	short := map[string]string{
		"list":   "List profiles from repository configuration",
		"active": "Show the resolved active profile",
	}[kind]
	if short == "" {
		return nil, errors.Errorf("unknown profiles command %q", kind)
	}
	return &ProfilesCommand{
		CommandDescription: glazedcmds.NewCommandDescription(
			kind,
			glazedcmds.WithShort(short),
			glazedcmds.WithParents("profiles"),
			glazedcmds.WithSections(repoSection),
		),
		kind: kind,
	}, nil
}

func (c *ProfilesCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	cfg, err := config.LoadStacked(
		repositoryContext.ConfigPath,
		config.DefaultOverridePath(repositoryContext.RepoRoot),
	)
	if err != nil {
		return err
	}
	active := cfg.ResolveProfile(repositoryContext.Profile)
	switch c.kind {
	case "active":
		if active != "" && cfg.GetProfile(active) == nil {
			return errors.Errorf("E_CONFIG_INVALID: profile %q not found", active)
		}
		return processor.AddRow(ctx, types.NewRow(
			types.MRP("profile", active),
			types.MRP("configured", active != ""),
		))
	case "list":
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			profile := cfg.Profiles[name]
			if err := processor.AddRow(ctx, types.NewRow(
				types.MRP("profile", name),
				types.MRP("active", name == active),
				types.MRP("display_name", profile.DisplayName),
				types.MRP("description", profile.Description),
				types.MRP("plugins", profile.Plugins),
			)); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.Errorf("unsupported profiles command %q", c.kind)
	}
}

func buildProfilesSubcommand(kind string) *cobra.Command {
	command, err := NewProfilesCommand(kind)
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}
