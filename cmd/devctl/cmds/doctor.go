package cmds

import (
	"context"

	"github.com/go-go-golems/devctl/pkg/operator"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type DoctorCommand struct {
	*glazedcmds.CommandDescription
}

type DoctorSettings struct {
	Plugins bool `glazed:"plugins"`
}

var _ glazedcmds.GlazeCommand = (*DoctorCommand)(nil)

func NewDoctorCommand() (*DoctorCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	return &DoctorCommand{CommandDescription: glazedcmds.NewCommandDescription(
		"doctor",
		glazedcmds.WithShort("Inspect configuration, durable state, ownership, and log health"),
		glazedcmds.WithFlags(
			fields.New("plugins", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Start plugins to verify handshakes")),
		),
		glazedcmds.WithSections(repoSection),
	)}, nil
}

func (c *DoctorCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	settings := DoctorSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return errors.Wrap(err, "decode doctor settings")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	controller, err := newOperatorController(repositoryContext.RepoRoot)
	if err != nil {
		return err
	}
	report, err := controller.Doctor(ctx, operator.DoctorRequest{
		RepoRoot: repositoryContext.RepoRoot, ConfigPath: repositoryContext.ConfigPath,
		Profile: repositoryContext.Profile, Cwd: repositoryContext.Cwd,
		Timeout: repositoryContext.Timeout, Plugins: settings.Plugins,
	})
	if err != nil {
		return err
	}
	for _, check := range report.Checks {
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("check", check.Check),
			types.MRP("scope", check.Scope),
			types.MRP("status", check.Status),
			types.MRP("code", check.Code),
			types.MRP("summary", check.Summary),
			types.MRP("path", check.Path),
			types.MRP("service", check.Service),
			types.MRP("run_id", check.RunID),
			types.MRP("remediation", check.Remediation),
		)); err != nil {
			return err
		}
	}
	return nil
}

func newDoctorCmd() *cobra.Command {
	command, err := NewDoctorCommand()
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}
