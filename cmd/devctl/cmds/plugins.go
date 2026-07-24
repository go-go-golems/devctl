package cmds

import (
	"context"
	stderrors "errors"
	"sort"

	"github.com/go-go-golems/devctl/pkg/plugincatalog"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
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
	command.AddCommand(newPluginsInspectCmd())
	command.AddCommand(newPluginsRunCmd())
	command.AddCommand(newPluginsRefreshCmd())
	return command
}

type PluginsCommand struct {
	*glazedcmds.CommandDescription
	kind       string
	providerID string
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
		"inspect":  "Inspect one selected plugin and its catalog commands",
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
		catalog, err := loadDiagnosticCatalog(repo)
		if err != nil {
			return err
		}
		return addCatalogRows(ctx, processor, catalog)
	case "inspect":
		spec, exists := repo.SpecByID[c.providerID]
		if !exists {
			return errors.Errorf("E_USAGE: plugin %q is not selected", c.providerID)
		}
		catalog, err := loadDiagnosticCatalog(repo)
		if err != nil {
			return err
		}
		return addPluginInspectionRows(ctx, processor, repo.ProfileName, spec, catalog)
	case "refresh":
		catalog, refreshErr := plugincatalog.Refresh(ctx, repo, plugincatalog.RefreshOptions{
			Reserved: defaultReservedCommandNames(),
		})
		if catalog != nil {
			if err := addCatalogRows(ctx, processor, catalog); err != nil {
				return err
			}
		}
		return refreshErr
	default:
		return errors.Errorf("unsupported plugins command %q", c.kind)
	}
}

func (c *PluginsCommand) ConfigureCobra(command *cobra.Command) {
	if c.kind == "inspect" {
		command.Use = "inspect PLUGIN"
		command.Args = cobra.ExactArgs(1)
	}
}

func (c *PluginsCommand) SetCobraArgs(args []string) error {
	if c.kind != "inspect" {
		return nil
	}
	if len(args) != 1 {
		return errors.Errorf("E_USAGE: inspect requires exactly one plugin ID")
	}
	c.providerID = args[0]
	return nil
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

func loadDiagnosticCatalog(repo *repository.Repository) (*plugincatalog.Catalog, error) {
	catalog, loadErr := plugincatalog.Load(repo, defaultReservedCommandNames())
	if loadErr == nil || stderrors.Is(loadErr, plugincatalog.ErrCatalogConflict) {
		return catalog, nil
	}
	if !stderrors.Is(loadErr, plugincatalog.ErrCatalogMissing) &&
		!stderrors.Is(loadErr, plugincatalog.ErrCatalogStale) {
		return nil, loadErr
	}
	return plugincatalog.Static(repo, defaultReservedCommandNames())
}

func addPluginInspectionRows(
	ctx context.Context,
	processor middlewares.Processor,
	profile string,
	spec runtime.PluginSpec,
	catalog *plugincatalog.Catalog,
) error {
	entries := make([]plugincatalog.CommandEntry, 0)
	for _, entry := range catalog.Commands {
		if entry.ProviderID == spec.ID {
			entries = append(entries, entry)
		}
	}
	for _, conflicts := range catalog.Conflicts {
		for _, entry := range conflicts {
			if entry.ProviderID == spec.ID {
				entries = append(entries, entry)
			}
		}
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })
	if len(entries) == 0 {
		entries = append(entries, plugincatalog.CommandEntry{})
	}
	for _, entry := range entries {
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("id", spec.ID),
			types.MRP("path", spec.Path),
			types.MRP("args", spec.Args),
			types.MRP("workdir", spec.WorkDir),
			types.MRP("priority", spec.Priority),
			types.MRP("profile", profile),
			types.MRP("command", entry.Name),
			types.MRP("command_help", entry.Help),
			types.MRP("command_args", entry.Args),
			types.MRP("conflict", len(catalog.Conflicts[entry.Name]) > 0),
		)); err != nil {
			return err
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

func newPluginsInspectCmd() *cobra.Command {
	return buildPluginsSubcommand("inspect")
}

func newPluginsRefreshCmd() *cobra.Command {
	return buildPluginsSubcommand("refresh")
}

func newPluginsRunCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "run PLUGIN COMMAND -- [ARGS...]",
		Short: "Run a catalog command through an explicit provider",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			options, err := getRootOptions(command)
			if err != nil {
				return err
			}
			repo, err := repository.Load(repository.Options{
				RepoRoot: options.RepoRoot, ConfigPath: options.Config,
				ProfileName: options.Profile, Cwd: options.RepoRoot, DryRun: options.DryRun,
			})
			if err != nil {
				return err
			}
			catalog, err := loadDiagnosticCatalog(repo)
			if err != nil {
				return err
			}
			entry, exists := catalogEntry(catalog, args[0], args[1])
			if !exists {
				return errors.Errorf(
					"E_USAGE: plugin %q does not provide catalog command %q",
					args[0], args[1],
				)
			}
			spec, exists := repo.SpecByID[args[0]]
			if !exists {
				return errors.Errorf("E_USAGE: plugin %q is not selected", args[0])
			}
			return executeDynamicCommand(command, catalog, spec, entry, args[2:])
		},
	}
	AddRepoFlags(command)
	return command
}

func catalogEntry(
	catalog *plugincatalog.Catalog,
	providerID string,
	name string,
) (plugincatalog.CommandEntry, bool) {
	if catalog == nil {
		return plugincatalog.CommandEntry{}, false
	}
	if entry, exists := catalog.Commands[name]; exists && entry.ProviderID == providerID {
		return entry, true
	}
	for _, entry := range catalog.Conflicts[name] {
		if entry.ProviderID == providerID {
			return entry, true
		}
	}
	return plugincatalog.CommandEntry{}, false
}
