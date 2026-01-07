package cmds

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Plugin discovery and inspection",
	}
	cmd.AddCommand(newPluginsListCmd())
	return cmd
}

func newPluginsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured plugins and their handshake capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			meta, err := requestMetaFromRootOptions(opts)
			if err != nil {
				return err
			}
			repo, err := repository.Load(repository.Options{RepoRoot: opts.RepoRoot, ConfigPath: opts.Config, Cwd: meta.Cwd, DryRun: opts.DryRun})
			if err != nil {
				return err
			}
			if len(repo.Specs) == 0 {
				return errors.New("no plugins configured (add .devctl.yaml)")
			}
			ctx := cmd.Context()

			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  2 * time.Second,
			})

			type pluginInfo struct {
				ID           string                 `json:"id"`
				Path         string                 `json:"path"`
				Args         []string               `json:"args,omitempty"`
				WorkDir      string                 `json:"workdir"`
				Priority     int                    `json:"priority"`
				PluginName   string                 `json:"plugin_name"`
				Protocol     string                 `json:"protocol_version"`
				Ops          []string               `json:"ops,omitempty"`
				Streams      []string               `json:"streams,omitempty"`
				Commands     []protocol.CommandSpec `json:"commands,omitempty"`
				HandshakeRaw any                    `json:"handshake_raw,omitempty"`
			}

			infos := make([]pluginInfo, 0, len(repo.Specs))
			for _, spec := range repo.Specs {
				c, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: repo.Request})
				if err != nil {
					return err
				}
				hs := c.Handshake()
				_ = c.Close(ctx)

				infos = append(infos, pluginInfo{
					ID:         spec.ID,
					Path:       spec.Path,
					Args:       spec.Args,
					WorkDir:    spec.WorkDir,
					Priority:   spec.Priority,
					PluginName: hs.PluginName,
					Protocol:   string(hs.ProtocolVersion),
					Ops:        hs.Capabilities.Ops,
					Streams:    hs.Capabilities.Streams,
					Commands:   hs.Capabilities.Commands,
				})
			}

			b, err := json.MarshalIndent(map[string]any{"plugins": infos}, "", "  ")
			if err != nil {
				return errors.Wrap(err, "marshal output")
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
}
