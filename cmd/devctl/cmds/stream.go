package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newStreamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start and inspect protocol streams",
	}
	cmd.AddCommand(newStreamStartCmd())
	return cmd
}

func newStreamStartCmd() *cobra.Command {
	var pluginID string
	var op string
	var inputJSON string
	var inputFile string
	var startTimeout time.Duration
	var rawJSON bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a stream op and print its events",
		RunE: func(cmd *cobra.Command, args []string) error {
			if op == "" {
				return errors.New("--op is required")
			}

			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			meta, err := requestMetaFromRootOptions(opts)
			if err != nil {
				return err
			}

			repo, err := repository.Load(repository.Options{RepoRoot: opts.RepoRoot, ConfigPath: opts.Config, ProfileName: opts.Profile, Cwd: meta.Cwd, DryRun: opts.DryRun})
			if err != nil {
				return err
			}
			if len(repo.Specs) == 0 {
				return errors.New("no plugins configured (add .devctl.yaml)")
			}

			input, err := loadStreamInput(inputJSON, inputFile)
			if err != nil {
				return err
			}

			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  3 * time.Second,
			})

			c, spec, err := selectStreamProvider(cmd.Context(), factory, repo.Specs, repo.Request, pluginID, op)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close(context.Background()) }()

			startCtx := cmd.Context()
			if startTimeout <= 0 {
				startTimeout = 2 * time.Second
			}
			var cancel context.CancelFunc
			startCtx, cancel = context.WithTimeout(startCtx, startTimeout)
			defer cancel()

			if !c.SupportsOp(op) {
				return &runtime.OpError{
					PluginID: spec.ID,
					Op:       op,
					Code:     protocol.ErrUnsupported,
					Message:  "op not declared in handshake capabilities",
				}
			}

			streamID, events, err := c.StartStream(startCtx, op, input)
			if err != nil {
				return err
			}

			if rawJSON {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "{\"plugin_id\":%q,\"op\":%q,\"stream_id\":%q}\n", spec.ID, op, streamID)
				enc := json.NewEncoder(cmd.OutOrStdout())
				for ev := range events {
					_ = enc.Encode(ev)
				}
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "plugin=%s op=%s stream_id=%s\n", spec.ID, op, streamID)
			for ev := range events {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), formatProtocolEvent(ev))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pluginID, "plugin", "", "Plugin id to use (defaults to first plugin that supports --op)")
	cmd.Flags().StringVar(&op, "op", "", "Stream-start operation name (required)")
	cmd.Flags().StringVar(&inputJSON, "input-json", "", "JSON object to pass as input to the stream-start op")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a JSON file to pass as input to the stream-start op")
	cmd.Flags().DurationVar(&startTimeout, "start-timeout", 2*time.Second, "Timeout for starting the stream (getting stream_id)")
	cmd.Flags().BoolVar(&rawJSON, "json", false, "Print raw protocol.Event JSON lines")
	AddRepoFlags(cmd)
	return cmd
}

func loadStreamInput(inputJSON, inputFile string) (map[string]any, error) {
	if inputJSON != "" && inputFile != "" {
		return nil, errors.New("use only one of --input-json or --input-file")
	}
	if inputJSON == "" && inputFile == "" {
		return map[string]any{}, nil
	}

	var b []byte
	if inputFile != "" {
		var err error
		b, err = os.ReadFile(inputFile)
		if err != nil {
			return nil, err
		}
	} else {
		b = []byte(inputJSON)
	}

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func selectStreamProvider(
	ctx context.Context,
	factory *runtime.Factory,
	specs []runtime.PluginSpec,
	meta runtime.RequestMeta,
	pluginID string,
	op string,
) (runtime.Client, runtime.PluginSpec, error) {
	ordered := append([]runtime.PluginSpec{}, specs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		return ordered[i].ID < ordered[j].ID
	})

	if pluginID != "" {
		for _, s := range ordered {
			if s.ID == pluginID {
				c, err := factory.Start(ctx, s, runtime.StartOptions{Meta: meta})
				if err != nil {
					return nil, runtime.PluginSpec{}, err
				}
				return c, s, nil
			}
		}
		return nil, runtime.PluginSpec{}, errors.Errorf("unknown plugin id %q", pluginID)
	}

	for _, s := range ordered {
		c, err := factory.Start(ctx, s, runtime.StartOptions{Meta: meta})
		if err != nil {
			continue
		}
		if c.SupportsOp(op) {
			return c, s, nil
		}
		_ = c.Close(context.Background())
	}
	return nil, runtime.PluginSpec{}, errors.Errorf("no configured plugin supports op %q", op)
}

func formatProtocolEvent(ev protocol.Event) string {
	if ev.Event == "end" {
		if ev.Ok == nil {
			return "[end]"
		}
		return fmt.Sprintf("[end ok=%v]", *ev.Ok)
	}
	msg := strings.TrimSpace(ev.Message)
	if msg == "" && len(ev.Fields) > 0 {
		if b, err := json.Marshal(ev.Fields); err == nil {
			msg = string(b)
		}
	}
	if msg == "" {
		msg = "-"
	}
	if ev.Level != "" {
		return fmt.Sprintf("[%s level=%s] %s", ev.Event, ev.Level, msg)
	}
	return fmt.Sprintf("[%s] %s", ev.Event, msg)
}
