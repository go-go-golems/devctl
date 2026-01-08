package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type StatusCommand struct {
	*glazedcmds.CommandDescription
}

var _ glazedcmds.WriterCommand = (*StatusCommand)(nil)

type StatusSettings struct {
	TailLines int `glazed.parameter:"tail-lines"`
}

func NewStatusCommand() (*StatusCommand, error) {
	repoLayer, err := getRepoLayer()
	if err != nil {
		return nil, err
	}

	return &StatusCommand{
		CommandDescription: glazedcmds.NewCommandDescription(
			"status",
			glazedcmds.WithShort("Show status of supervised services"),
			glazedcmds.WithFlags(
				parameters.NewParameterDefinition(
					"tail-lines",
					parameters.ParameterTypeInteger,
					parameters.WithDefault(25),
					parameters.WithHelp("How many stderr lines to include for dead services"),
				),
			),
			glazedcmds.WithLayersList(repoLayer),
		),
	}, nil
}

func (c *StatusCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	s := StatusSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, &s); err != nil {
		return err
	}
	rc, err := RepoContextFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	st, err := state.Load(rc.RepoRoot)
	if err != nil {
		return err
	}

	type svc struct {
		Name   string          `json:"name"`
		PID    int             `json:"pid"`
		Alive  bool            `json:"alive"`
		Stdout string          `json:"stdout_log"`
		Stderr string          `json:"stderr_log"`
		Exit   *state.ExitInfo `json:"exit,omitempty"`
	}
	var services []svc

	for _, svcState := range st.Services {
		alive := state.ProcessAlive(svcState.PID)
		var exitInfo *state.ExitInfo
		if !alive && svcState.ExitInfo != "" {
			if _, err := os.Stat(svcState.ExitInfo); err == nil {
				ei, err := state.ReadExitInfo(svcState.ExitInfo)
				if err == nil {
					exitInfo = ei
					if s.TailLines > 0 && len(exitInfo.StderrTail) > s.TailLines {
						exitInfo.StderrTail = append([]string{}, exitInfo.StderrTail[len(exitInfo.StderrTail)-s.TailLines:]...)
					}
					if s.TailLines > 0 && len(exitInfo.StdoutTail) > s.TailLines {
						exitInfo.StdoutTail = append([]string{}, exitInfo.StdoutTail[len(exitInfo.StdoutTail)-s.TailLines:]...)
					}
					if exitInfo.StderrTail == nil && s.TailLines > 0 {
						if lines, err := state.TailLines(svcState.StderrLog, s.TailLines, 2<<20); err == nil {
							exitInfo.StderrTail = lines
						}
					}
				}
			}
		}
		if !alive && exitInfo == nil && s.TailLines > 0 {
			lines, err := state.TailLines(svcState.StderrLog, s.TailLines, 2<<20)
			if err == nil {
				exitInfo = &state.ExitInfo{
					Service:    svcState.Name,
					PID:        svcState.PID,
					StartedAt:  st.CreatedAt,
					ExitedAt:   time.Now(),
					Error:      "exit info unavailable (older state); stderr tail captured at status time",
					StderrTail: lines,
				}
			}
		}

		services = append(services, svc{
			Name:   svcState.Name,
			PID:    svcState.PID,
			Alive:  alive,
			Stdout: svcState.StdoutLog,
			Stderr: svcState.StderrLog,
			Exit:   exitInfo,
		})
	}

	b, err := json.MarshalIndent(map[string]any{"services": services}, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal status")
	}
	_, _ = fmt.Fprintln(w, string(b))
	return nil
}

func newStatusCmd() *cobra.Command {
	c, err := NewStatusCommand()
	cobra.CheckErr(err)

	cmd, err := cli.BuildCobraCommand(c, cli.WithParserConfig(cli.CobraParserConfig{AppName: "devctl"}))
	cobra.CheckErr(err)
	return cmd
}
