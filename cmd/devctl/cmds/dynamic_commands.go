package cmds

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func AddDynamicPluginCommands(root *cobra.Command, args []string) error {
	repoRoot, cfgPath, positionals, err := parseRepoArgs(args)
	if err != nil {
		return err
	}
	if len(positionals) > 0 && positionals[0] == "__wrap-service" {
		return nil
	}
	if len(positionals) == 0 {
		return nil
	}
	if positionals[0] != "completion" && rootHasCommand(root, positionals[0]) {
		return nil
	}

	repo, err := repository.Load(repository.Options{RepoRoot: repoRoot, ConfigPath: cfgPath, Cwd: repoRoot, DryRun: false})
	if err != nil {
		return err
	}
	if len(repo.Specs) == 0 {
		return nil
	}

	factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})

	type provider struct {
		spec runtime.PluginSpec
		cmd  protocol.CommandSpec
	}
	byName := map[string]provider{}

	for _, spec := range repo.Specs {
		c, err := factory.Start(context.Background(), spec, runtime.StartOptions{Meta: repo.Request})
		if err != nil {
			log.Warn().Err(err).Str("plugin", spec.ID).Msg("failed to start plugin for command discovery")
			continue
		}

		hs := c.Handshake()
		_ = c.Close(context.Background())
		if !c.SupportsOp("command.run") {
			continue
		}
		for _, cmdSpec := range hs.Capabilities.Commands {
			if cmdSpec.Name == "" {
				continue
			}
			if existing, ok := byName[cmdSpec.Name]; ok {
				log.Warn().Str("command", cmdSpec.Name).Str("a", existing.spec.ID).Str("b", spec.ID).Msg("command name collision; keeping first")
				continue
			}
			byName[cmdSpec.Name] = provider{spec: spec, cmd: cmdSpec}
		}
	}

	for name, prov := range byName {
		prov := prov
		root.AddCommand(&cobra.Command{
			Use:   name,
			Short: prov.cmd.Help,
			Args:  cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, argv []string) error {
				opts, err := getRootOptions(cmd)
				if err != nil {
					return err
				}
				meta, err := requestMetaFromRootOptions(opts)
				if err != nil {
					return err
				}

				cfg, err := config.LoadOptional(opts.Config)
				if err != nil {
					return err
				}
				if !opts.Strict && cfg.Strictness == "error" {
					opts.Strict = true
				}

				factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
				client, err := factory.Start(cmd.Context(), prov.spec, runtime.StartOptions{Meta: meta})
				if err != nil {
					return err
				}
				defer func() { _ = client.Close(context.Background()) }()

				p := &engine.Pipeline{Clients: []runtime.Client{client}, Opts: engine.Options{Strict: opts.Strict, DryRun: opts.DryRun}}
				opCtx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
				conf, err := p.MutateConfig(opCtx, patch.Config{})
				cancel()
				if err != nil {
					return err
				}

				var cmdOut struct {
					ExitCode int `json:"exit_code"`
				}
				opCtx, cancel = context.WithTimeout(cmd.Context(), opts.Timeout)
				err = client.Call(opCtx, "command.run", map[string]any{
					"name":   name,
					"argv":   argv,
					"config": conf,
				}, &cmdOut)
				cancel()
				if err != nil {
					return err
				}
				if cmdOut.ExitCode != 0 {
					return errors.Errorf("command %q failed with exit_code=%d", name, cmdOut.ExitCode)
				}
				return nil
			},
		})
	}

	return nil
}

func parseRepoArgs(args []string) (string, string, []string, error) {
	fs := pflag.NewFlagSet("devctl-bootstrap", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetInterspersed(true)
	fs.SetOutput(io.Discard)
	fs.String("repo-root", "", "")
	fs.String("config", "", "")
	_ = fs.Parse(args[1:])

	repoRoot := ""
	cfgPath := ""

	var err error
	repoRoot, _ = fs.GetString("repo-root")
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return "", "", nil, err
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return "", "", nil, err
	}

	cfgPath, _ = fs.GetString("config")
	if cfgPath == "" {
		cfgPath = config.DefaultPath(repoRoot)
	} else if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(repoRoot, cfgPath)
	}
	return repoRoot, cfgPath, fs.Args(), nil
}

func rootHasCommand(root *cobra.Command, name string) bool {
	for _, c := range root.Commands() {
		if c.Name() == name {
			return true
		}
		for _, a := range c.Aliases {
			if a == name {
				return true
			}
		}
	}
	return false
}
