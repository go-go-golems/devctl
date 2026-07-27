package cmds

import (
	"context"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newTuiCmd() *cobra.Command {
	var refresh time.Duration
	var altScreen bool
	var debugLogs bool
	command := &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI for devctl",
		RunE: func(command *cobra.Command, _ []string) error {
			options, err := getRootOptions(command)
			if err != nil {
				return err
			}
			if !debugLogs {
				zerolog.SetGlobalLevel(zerolog.Disabled)
				log.Logger = zerolog.New(io.Discard)
			}
			controller, err := newOperatorController(options.RepoRoot)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(command.Context())
			defer cancel()
			model := tui.NewModel(tui.Options{
				Context: ctx, Controller: controller,
				RepoRoot: options.RepoRoot, ConfigPath: options.Config,
				Profile: options.Profile, Cwd: options.RepoRoot,
				Strict: options.Strict, DryRun: options.DryRun,
				Timeout: options.Timeout, Refresh: refresh,
			})
			programOptions := []tea.ProgramOption{
				tea.WithInput(command.InOrStdin()),
				tea.WithOutput(command.OutOrStdout()),
				tea.WithContext(ctx),
			}
			if altScreen {
				programOptions = append(programOptions, tea.WithAltScreen())
			}
			if _, err := tea.NewProgram(model, programOptions...).Run(); err != nil {
				return errors.Wrap(err, "tui")
			}
			return nil
		},
	}
	command.Flags().DurationVar(&refresh, "refresh", time.Second, "Snapshot refresh interval")
	command.Flags().BoolVar(&altScreen, "alt-screen", true, "Use the terminal alternate screen buffer")
	command.Flags().BoolVar(&debugLogs, "debug-logs", false, "Allow zerolog output while the TUI runs")
	AddRepoFlags(command)
	return command
}
