package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
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

type PluginsListCommand struct {
	*glazedcmds.CommandDescription
}

var _ glazedcmds.WriterCommand = (*PluginsListCommand)(nil)

func NewPluginsListCommand() (*PluginsListCommand, error) {
	repoLayer, err := getRepoLayer()
	if err != nil {
		return nil, err
	}

	return &PluginsListCommand{
		CommandDescription: glazedcmds.NewCommandDescription(
			"list",
			glazedcmds.WithShort("List configured plugins and their handshake capabilities"),
			glazedcmds.WithParents("plugins"),
			glazedcmds.WithLayersList(repoLayer),
		),
	}, nil
}

func (c *PluginsListCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	rc, err := RepoContextFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	repo, err := repository.Load(repository.Options{
		RepoRoot:   rc.RepoRoot,
		ConfigPath: rc.ConfigPath,
		Cwd:        rc.Cwd,
		DryRun:     rc.DryRun,
	})
	if err != nil {
		return err
	}
	if len(repo.Specs) == 0 {
		return errors.New("no plugins configured (add .devctl.yaml)")
	}

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
		client, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: repo.Request})
		if err != nil {
			return err
		}
		hs := client.Handshake()
		_ = client.Close(ctx)

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
	_, _ = fmt.Fprintln(w, string(b))
	return nil
}

func newPluginsListCmd() *cobra.Command {
	c, err := NewPluginsListCommand()
	cobra.CheckErr(err)

	cmd, err := cli.BuildCobraCommand(c, cli.WithParserConfig(cli.CobraParserConfig{AppName: "devctl"}))
	cobra.CheckErr(err)
	return cmd
}
