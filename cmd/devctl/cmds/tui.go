package cmds

import (
	"context"
	stderrors "errors"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/go-go-golems/devctl/pkg/tui/models"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func newTuiCmd() *cobra.Command {
	var refresh time.Duration
	var altScreen bool
	var debugLogs bool

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI for devctl",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}

			if !debugLogs {
				zerolog.SetGlobalLevel(zerolog.Disabled)
				log.Logger = zerolog.New(io.Discard)
			}

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			bus, err := tui.NewInMemoryBus()
			if err != nil {
				return err
			}

			tui.RegisterDomainToUITransformer(bus)

			tui.RegisterUIActionRunner(bus, tui.RootOptions{
				RepoRoot: opts.RepoRoot,
				Config:   opts.Config,
				Strict:   opts.Strict,
				DryRun:   opts.DryRun,
				Timeout:  opts.Timeout,
			})

			model := models.NewRootModel(models.RootModelOptions{
				PublishAction: func(req tui.ActionRequest) error {
					return tui.PublishAction(bus.Publisher, req)
				},
				PublishStreamStart: func(req tui.StreamStartRequest) error {
					return tui.PublishStreamStart(bus.Publisher, req)
				},
				PublishStreamStop: func(req tui.StreamStopRequest) error {
					return tui.PublishStreamStop(bus.Publisher, req)
				},
			})
			programOptions := []tea.ProgramOption{
				tea.WithInput(cmd.InOrStdin()),
				tea.WithOutput(cmd.OutOrStdout()),
			}
			if altScreen {
				programOptions = append(programOptions, tea.WithAltScreen())
			}
			program := tea.NewProgram(model, programOptions...)
			tui.RegisterUIForwarder(bus, program)

			watcher := &tui.StateWatcher{
				RepoRoot: opts.RepoRoot,
				Interval: refresh,
				Pub:      bus.Publisher,
			}

			eg, egCtx := errgroup.WithContext(ctx)
			eg.Go(func() error {
				err := bus.Run(egCtx)
				if stderrors.Is(err, context.Canceled) {
					return nil
				}
				return err
			})
			eg.Go(func() error {
				err := watcher.Run(egCtx)
				if stderrors.Is(err, context.Canceled) {
					return nil
				}
				return err
			})
			eg.Go(func() error {
				_, err := program.Run()
				cancel()
				if stderrors.Is(err, context.Canceled) {
					return nil
				}
				return err
			})

			if err := eg.Wait(); err != nil {
				return errors.Wrap(err, "tui")
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&refresh, "refresh", 1*time.Second, "Refresh interval for state polling")
	cmd.Flags().BoolVar(&altScreen, "alt-screen", true, "Use the terminal alternate screen buffer")
	cmd.Flags().BoolVar(&debugLogs, "debug-logs", false, "Allow zerolog output to stdout/stderr while the TUI runs (may corrupt the UI)")
	return cmd
}
