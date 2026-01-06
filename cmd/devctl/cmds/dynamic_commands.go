package cmds

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type CommandSpec struct {
	Name     string       `json:"name"`
	Help     string       `json:"help,omitempty"`
	ArgsSpec []CommandArg `json:"args_spec,omitempty"`
}

type CommandArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func AddDynamicPluginCommands(root *cobra.Command, args []string) error {
	repoRoot, cfgPath, err := parseRepoArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.LoadOptional(cfgPath)
	if err != nil {
		return err
	}
	specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: repoRoot})
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return nil
	}

	factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})

	type provider struct {
		spec runtime.PluginSpec
		cmd  CommandSpec
	}
	byName := map[string]provider{}

	for _, spec := range specs {
		c, err := factory.Start(context.Background(), spec)
		if err != nil {
			log.Warn().Err(err).Str("plugin", spec.ID).Msg("failed to start plugin for command discovery")
			continue
		}

		var out struct {
			Commands []CommandSpec `json:"commands"`
		}
		callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = c.Call(callCtx, "commands.list", map[string]any{}, &out)
		callCancel()
		_ = c.Close(context.Background())
		if err != nil {
			continue
		}

		for _, cmdSpec := range out.Commands {
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

				cfg, err := config.LoadOptional(opts.Config)
				if err != nil {
					return err
				}
				if !opts.Strict && cfg.Strictness == "error" {
					opts.Strict = true
				}

				specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: opts.RepoRoot})
				if err != nil {
					return err
				}

				var target *runtime.PluginSpec
				for _, s := range specs {
					if s.ID == prov.spec.ID {
						tmp := s
						target = &tmp
						break
					}
				}
				if target == nil {
					return errors.Errorf("command provider plugin not found: %s", prov.spec.ID)
				}

				factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
				client, err := factory.Start(cmd.Context(), *target)
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

func parseRepoArgs(args []string) (repoRoot string, cfgPath string, err error) {
	fs := pflag.NewFlagSet("devctl-bootstrap", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	fs.SetInterspersed(true)
	fs.SetOutput(io.Discard)
	fs.String("repo-root", "", "")
	fs.String("config", "", "")
	_ = fs.Parse(args[1:])

	repoRoot, _ = fs.GetString("repo-root")
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return "", "", err
	}

	cfgPath, _ = fs.GetString("config")
	if cfgPath == "" {
		cfgPath = config.DefaultPath(repoRoot)
	} else if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(repoRoot, cfgPath)
	}
	return repoRoot, cfgPath, nil
}
