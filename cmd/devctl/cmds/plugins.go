package cmds

import (
	"context"
	stderrors "errors"
	"sort"

	"github.com/go-go-golems/devctl/pkg/plugincatalog"
	"github.com/go-go-golems/devctl/pkg/repository"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newPluginsCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "plugins",
		Short: "Inspect plugins and manage the dynamic command catalog",
	}
	command.AddCommand(newPluginsListCmd())
	command.AddCommand(newPluginsCommandsCmd())
	command.AddCommand(newPluginsRefreshCmd())
	return command
}

type PluginsCommand struct {
	*glazedcmds.CommandDescription
	kind string
}

var _ glazedcmds.GlazeCommand = (*PluginsCommand)(nil)

func NewPluginsCommand(kind string) (*PluginsCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	short := map[string]string{
		"list":     "List selected configured plugins without starting them",
		"commands": "List validated dynamic root commands",
		"refresh":  "Start selected providers and refresh the command catalog",
	}[kind]
	if short == "" {
		return nil, errors.Errorf("unknown plugins command %q", kind)
	}
	return &PluginsCommand{
		CommandDescription: glazedcmds.NewCommandDescription(
			kind,
			glazedcmds.WithShort(short),
			glazedcmds.WithParents("plugins"),
			glazedcmds.WithSections(repoSection),
		),
		kind: kind,
	}, nil
}

func (c *PluginsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	repo, err := repository.Load(repository.Options{
		RepoRoot: repositoryContext.RepoRoot, ConfigPath: repositoryContext.ConfigPath,
		ProfileName: repositoryContext.Profile, Cwd: repositoryContext.Cwd,
		DryRun: repositoryContext.DryRun,
	})
	if err != nil {
		return err
	}
	switch c.kind {
	case "list":
		specs := append([]runtimePluginSpec{}, pluginRows(repo)...)
		sort.Slice(specs, func(left, right int) bool { return specs[left].ID < specs[right].ID })
		for _, spec := range specs {
			if err := processor.AddRow(ctx, types.NewRow(
				types.MRP("id", spec.ID),
				types.MRP("path", spec.Path),
				types.MRP("args", spec.Args),
				types.MRP("workdir", spec.WorkDir),
				types.MRP("priority", spec.Priority),
				types.MRP("profile", repo.ProfileName),
			)); err != nil {
				return err
			}
		}
		return nil
	case "commands":
		catalog, loadErr := plugincatalog.Load(repo, defaultReservedCommandNames())
		if loadErr != nil {
			if !stderrors.Is(loadErr, plugincatalog.ErrCatalogMissing) &&
				!stderrors.Is(loadErr, plugincatalog.ErrCatalogStale) {
				return loadErr
			}
			catalog, err = plugincatalog.Static(repo, defaultReservedCommandNames())
			if err != nil {
				return err
			}
		}
		return addCatalogRows(ctx, processor, catalog)
	case "refresh":
		catalog, err := plugincatalog.Refresh(ctx, repo, plugincatalog.RefreshOptions{
			Reserved: defaultReservedCommandNames(),
		})
		if err != nil {
			return err
		}
		return addCatalogRows(ctx, processor, catalog)
	default:
		return errors.Errorf("unsupported plugins command %q", c.kind)
	}
}

type runtimePluginSpec struct {
	ID       string
	Path     string
	Args     []string
	WorkDir  string
	Priority int
}

func pluginRows(repo *repository.Repository) []runtimePluginSpec {
	rows := make([]runtimePluginSpec, 0, len(repo.Specs))
	for _, spec := range repo.Specs {
		rows = append(rows, runtimePluginSpec{
			ID: spec.ID, Path: spec.Path, Args: spec.Args,
			WorkDir: spec.WorkDir, Priority: spec.Priority,
		})
	}
	return rows
}

func addCatalogRows(
	ctx context.Context,
	processor middlewares.Processor,
	catalog *plugincatalog.Catalog,
) error {
	names := make([]string, 0, len(catalog.Commands))
	for name := range catalog.Commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := catalog.Commands[name]
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("name", entry.Name),
			types.MRP("help", entry.Help),
			types.MRP("provider_id", entry.ProviderID),
			types.MRP("plugin_name", entry.PluginName),
			types.MRP("profile", catalog.Profile),
			types.MRP("fingerprint", catalog.ConfigFingerprint),
			types.MRP("conflict", false),
		)); err != nil {
			return err
		}
	}
	conflictNames := make([]string, 0, len(catalog.Conflicts))
	for name := range catalog.Conflicts {
		conflictNames = append(conflictNames, name)
	}
	sort.Strings(conflictNames)
	for _, name := range conflictNames {
		for _, entry := range catalog.Conflicts[name] {
			if err := processor.AddRow(ctx, types.NewRow(
				types.MRP("name", name),
				types.MRP("help", entry.Help),
				types.MRP("provider_id", entry.ProviderID),
				types.MRP("plugin_name", entry.PluginName),
				types.MRP("profile", catalog.Profile),
				types.MRP("fingerprint", catalog.ConfigFingerprint),
				types.MRP("conflict", true),
			)); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildPluginsSubcommand(kind string) *cobra.Command {
	command, err := NewPluginsCommand(kind)
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}

func newPluginsListCmd() *cobra.Command {
	return buildPluginsSubcommand("list")
}

func newPluginsCommandsCmd() *cobra.Command {
	return buildPluginsSubcommand("commands")
}

func newPluginsRefreshCmd() *cobra.Command {
	return buildPluginsSubcommand("refresh")
}
