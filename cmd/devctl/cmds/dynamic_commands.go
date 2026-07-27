package cmds

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/plugincatalog"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type PluginCommandExitError struct {
	Command    string
	ProviderID string
	ExitCode   int
}

func (e *PluginCommandExitError) Error() string {
	return fmt.Sprintf(
		"plugin command %q from provider %q failed with exit_code=%d",
		e.Command, e.ProviderID, e.ExitCode,
	)
}

func AddDynamicPluginCommands(root *cobra.Command, args []string) error {
	repoRoot, configPath, profileName, positionals, err := parseRepoArgs(args)
	if err != nil {
		return err
	}
	discoveryMode := len(positionals) == 0 || positionals[0] == "completion"
	if !discoveryMode && (positionals[0] == "__wrap-service" ||
		rootHasCommand(root, positionals[0])) {
		return nil
	}
	if discoveryMode {
		if _, statErr := os.Stat(configPath); statErr != nil {
			if os.IsNotExist(statErr) {
				return nil
			}
			return statErr
		}
	}
	repo, err := repository.Load(repository.Options{
		RepoRoot: repoRoot, ConfigPath: configPath, ProfileName: profileName,
		Cwd: repoRoot, DryRun: false,
	})
	if err != nil {
		return err
	}
	if len(repo.Specs) == 0 {
		return nil
	}
	reserved := reservedCommandNames(root)
	catalog, loadErr := plugincatalog.Load(repo, reserved)
	if loadErr != nil {
		if catalog != nil && errors.Is(loadErr, plugincatalog.ErrCatalogConflict) {
			if discoveryMode {
				return registerCatalogCommands(root, repo, catalog)
			}
			requestedName := positionals[0]
			if conflicts := catalog.Conflicts[requestedName]; len(conflicts) > 0 {
				return pluginConflictError(requestedName, repo.ProfileName, conflicts)
			}
			if entry, exists := catalog.Commands[requestedName]; exists {
				return registerDynamicCommand(root, repo, catalog, entry)
			}
			return nil
		}
		staticCatalog, staticErr := plugincatalog.Static(repo, reserved)
		if staticErr != nil {
			return staticErr
		}
		if discoveryMode {
			return registerCatalogCommands(root, repo, staticCatalog)
		}
		requestedName := positionals[0]
		if entry, exists := staticCatalog.Commands[requestedName]; exists {
			return registerDynamicCommand(root, repo, staticCatalog, entry)
		}
		if conflicts := staticCatalog.Conflicts[requestedName]; len(conflicts) > 0 {
			return pluginConflictError(requestedName, repo.ProfileName, conflicts)
		}
		return errors.Wrapf(loadErr, "plugin command catalog unavailable; run `devctl plugins refresh --repo-root %s`", repo.Root)
	}
	if discoveryMode {
		return registerCatalogCommands(root, repo, catalog)
	}
	requestedName := positionals[0]
	if conflicts := catalog.Conflicts[requestedName]; len(conflicts) > 0 {
		return pluginConflictError(requestedName, repo.ProfileName, conflicts)
	}
	entry, exists := catalog.Commands[requestedName]
	if !exists {
		return nil
	}
	return registerDynamicCommand(root, repo, catalog, entry)
}

func registerCatalogCommands(
	root *cobra.Command,
	repo *repository.Repository,
	catalog *plugincatalog.Catalog,
) error {
	names := make([]string, 0, len(catalog.Commands))
	for name := range catalog.Commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := registerDynamicCommand(root, repo, catalog, catalog.Commands[name]); err != nil {
			return err
		}
	}
	return nil
}

func registerDynamicCommand(
	root *cobra.Command,
	repo *repository.Repository,
	catalog *plugincatalog.Catalog,
	entry plugincatalog.CommandEntry,
) error {
	spec, exists := repo.SpecByID[entry.ProviderID]
	if !exists {
		return errors.Errorf("PLUGIN_CATALOG_STALE: provider %q is no longer selected; run `devctl plugins refresh`", entry.ProviderID)
	}
	dynamicCommand := &cobra.Command{
		Use:   entry.Name,
		Short: entry.Help,
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, argv []string) error {
			return executeDynamicCommand(command, catalog, spec, entry, argv)
		},
	}
	AddRepoFlags(dynamicCommand)
	root.AddCommand(dynamicCommand)
	return nil
}

func executeDynamicCommand(
	command *cobra.Command,
	catalog *plugincatalog.Catalog,
	spec runtime.PluginSpec,
	entry plugincatalog.CommandEntry,
	argv []string,
) error {
	options, err := getRootOptions(command)
	if err != nil {
		return err
	}
	meta, err := requestMetaFromRootOptions(options)
	if err != nil {
		return err
	}
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  2 * time.Second,
	})
	client, err := factory.Start(command.Context(), spec, runtime.StartOptions{Meta: meta})
	if err != nil {
		return errors.Wrapf(err, "plugin command provider %q startup", entry.ProviderID)
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = client.Close(closeContext)
	}()
	if err := validateRuntimeCatalog(client, catalog, entry); err != nil {
		return err
	}
	configuration, err := config.LoadStacked(options.Config, config.DefaultOverridePath(options.RepoRoot))
	if err != nil {
		return err
	}
	if !options.Strict && configuration.Strictness == "error" {
		options.Strict = true
	}
	pipeline := &engine.Pipeline{
		Clients: []runtime.Client{client},
		Opts:    engine.Options{Strict: options.Strict, DryRun: options.DryRun},
	}
	operationContext, cancel := context.WithTimeout(command.Context(), options.Timeout)
	effectiveConfig, err := pipeline.MutateConfig(operationContext, patch.Config{})
	cancel()
	if err != nil {
		return err
	}
	var output struct {
		ExitCode int `json:"exit_code"`
	}
	operationContext, cancel = context.WithTimeout(command.Context(), options.Timeout)
	err = client.Call(operationContext, "command.run", map[string]any{
		"name": entry.Name, "argv": argv, "config": effectiveConfig,
	}, &output)
	cancel()
	if err != nil {
		return err
	}
	if output.ExitCode != 0 {
		return &PluginCommandExitError{
			Command: entry.Name, ProviderID: entry.ProviderID, ExitCode: output.ExitCode,
		}
	}
	return nil
}

func validateRuntimeCatalog(
	client runtime.Client,
	catalog *plugincatalog.Catalog,
	selected plugincatalog.CommandEntry,
) error {
	handshake := client.Handshake()
	if !client.SupportsOp("command.run") {
		return errors.Errorf("PLUGIN_CATALOG_STALE: provider %q no longer supports command.run; run `devctl plugins refresh`", selected.ProviderID)
	}
	if selected.PluginName != "" && handshake.PluginName != selected.PluginName {
		return errors.Errorf("PLUGIN_CATALOG_STALE: provider identity changed; run `devctl plugins refresh`")
	}
	expected := make([]protocol.CommandSpec, 0)
	for _, entry := range catalog.Commands {
		if entry.ProviderID == selected.ProviderID {
			expected = append(expected, protocol.CommandSpec{
				Name: entry.Name, Help: entry.Help, ArgsSpec: entry.Args,
			})
		}
	}
	for _, entries := range catalog.Conflicts {
		for _, entry := range entries {
			if entry.ProviderID == selected.ProviderID {
				expected = append(expected, protocol.CommandSpec{
					Name: entry.Name, Help: entry.Help, ArgsSpec: entry.Args,
				})
			}
		}
	}
	actual := append([]protocol.CommandSpec{}, handshake.Capabilities.Commands...)
	sort.Slice(expected, func(left, right int) bool { return expected[left].Name < expected[right].Name })
	sort.Slice(actual, func(left, right int) bool { return actual[left].Name < actual[right].Name })
	if len(expected) != len(actual) {
		return errors.Errorf("PLUGIN_CATALOG_STALE: provider command handshake changed; run `devctl plugins refresh`")
	}
	for index := range expected {
		if !equalCommandSpec(expected[index], actual[index]) {
			return errors.Errorf("PLUGIN_CATALOG_STALE: provider command handshake changed; run `devctl plugins refresh`")
		}
	}
	return nil
}

func equalCommandSpec(left protocol.CommandSpec, right protocol.CommandSpec) bool {
	if left.Name != right.Name || left.Help != right.Help || len(left.ArgsSpec) != len(right.ArgsSpec) {
		return false
	}
	for index := range left.ArgsSpec {
		if left.ArgsSpec[index] != right.ArgsSpec[index] {
			return false
		}
	}
	return true
}

func pluginConflictError(
	name string,
	profile string,
	conflicts []plugincatalog.CommandEntry,
) error {
	providers := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		providers = append(providers, conflict.ProviderID)
	}
	sort.Strings(providers)
	return errors.Errorf(
		"PLUGIN_COMMAND_CONFLICT: command %q is ambiguous in profile %q across providers %v",
		name, profile, providers,
	)
}

func reservedCommandNames(root *cobra.Command) map[string]bool {
	reserved := defaultReservedCommandNames()
	for _, command := range root.Commands() {
		reserved[command.Name()] = true
		for _, alias := range command.Aliases {
			reserved[alias] = true
		}
	}
	return reserved
}

func defaultReservedCommandNames() map[string]bool {
	names := []string{
		"up", "down", "restart", "status", "logs", "doctor", "plan",
		"build", "prepare", "validate", "profiles", "plugins", "stream",
		"tui", "completion", "help", "dev", "__wrap-service",
	}
	reserved := make(map[string]bool, len(names))
	for _, name := range names {
		reserved[name] = true
	}
	return reserved
}

func parseRepoArgs(args []string) (string, string, string, []string, error) {
	flagSet := pflag.NewFlagSet("devctl-bootstrap", pflag.ContinueOnError)
	flagSet.ParseErrorsAllowlist.UnknownFlags = true
	flagSet.SetInterspersed(true)
	flagSet.SetOutput(io.Discard)
	flagSet.String("repo-root", "", "")
	flagSet.String("config", "", "")
	flagSet.String("profile", "", "")
	_ = flagSet.Parse(args[1:])

	repoRoot, _ := flagSet.GetString("repo-root")
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			return "", "", "", nil, err
		}
	}
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", "", nil, err
	}
	configPath, _ := flagSet.GetString("config")
	if configPath == "" {
		configPath = config.DefaultPath(repoRoot)
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(repoRoot, configPath)
	}
	profileName, _ := flagSet.GetString("profile")
	return repoRoot, configPath, profileName, flagSet.Args(), nil
}

func rootHasCommand(root *cobra.Command, name string) bool {
	for _, command := range root.Commands() {
		if command.Name() == name {
			return true
		}
		for _, alias := range command.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}
