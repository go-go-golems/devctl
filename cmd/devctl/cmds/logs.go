package cmds

import (
	"context"
	stderrors "errors"
	"io"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type LogsCommand struct {
	*glazedcmds.CommandDescription
}

type LogsSettings struct {
	Services   []string `glazed:"services"`
	Follow     bool     `glazed:"follow"`
	Tail       int      `glazed:"tail"`
	Since      string   `glazed:"since"`
	Until      string   `glazed:"until"`
	Sources    []string `glazed:"source"`
	Streams    []string `glazed:"stream"`
	Levels     []string `glazed:"level"`
	Contains   string   `glazed:"contains"`
	RunIDs     []string `glazed:"run"`
	Timestamps bool     `glazed:"timestamps"`
	NoPrefix   bool     `glazed:"no-prefix"`
	ANSI       string   `glazed:"ansi"`
}

var _ glazedcmds.GlazeCommand = (*LogsCommand)(nil)

func (c *LogsCommand) BuildGlazedProcessor(
	vals *values.Values,
	writer io.Writer,
) (middlewares.Processor, bool, error) {
	logValues, exists := vals.Get(schema.DefaultSlug)
	if !exists {
		return nil, false, errors.New("logs settings are unavailable")
	}
	follow, _ := logValues.GetField("follow")
	outputValues, exists := vals.Get(glazedsettings.GlazedSlug)
	if !exists {
		return nil, false, errors.New("glazed output settings are unavailable")
	}
	output, _ := outputValues.GetField("output")
	if follow != true || output != "json" {
		return nil, false, nil
	}
	processor, err := newJSONLinesProcessor(outputValues, writer)
	return processor, true, err
}

func (c *LogsCommand) PrepareGlazedValues(vals *values.Values) error {
	logValues, exists := vals.Get(schema.DefaultSlug)
	if !exists {
		return errors.New("logs settings are unavailable")
	}
	followValue, exists := logValues.Fields.Get("follow")
	if !exists || followValue.Value != true {
		return nil
	}
	glazedValues, exists := vals.Get(glazedsettings.GlazedSlug)
	if !exists {
		return errors.New("glazed output settings are unavailable")
	}
	outputValue, exists := glazedValues.Fields.Get("output")
	if !exists || outputValue.Value != "json" {
		return nil
	}
	objectsValue, exists := glazedValues.Fields.Get("output-as-objects")
	if !exists {
		return errors.New("glazed JSON object output setting is unavailable")
	}
	objectsValue.Value = true
	return nil
}

func NewLogsCommand() (*LogsCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	glazedSection, err := glazedsettings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	glazedSection.OutputSection.Definitions.Delete("stream")
	return &LogsCommand{CommandDescription: glazedcmds.NewCommandDescription(
		"logs",
		glazedcmds.WithShort("Query or follow structured service logs"),
		glazedcmds.WithArguments(
			fields.New("services", fields.TypeStringList, fields.WithHelp("Service names; empty selects all")),
		),
		glazedcmds.WithFlags(
			fields.New("follow", fields.TypeBool, fields.WithDefault(false), fields.WithShortFlag("f"), fields.WithHelp("Follow new records")),
			fields.New("tail", fields.TypeInteger, fields.WithDefault(100), fields.WithShortFlag("n"), fields.WithHelp("Records per run; -1 means all, 0 means no history")),
			fields.New("since", fields.TypeString, fields.WithDefault(""), fields.WithHelp("RFC3339 time or duration before now")),
			fields.New("until", fields.TypeString, fields.WithDefault(""), fields.WithHelp("RFC3339 time or duration before now")),
			fields.New("source", fields.TypeStringList, fields.WithHelp("Sources: service, pipeline, plugin, system")),
			fields.New("stream", fields.TypeStringList, fields.WithHelp("Streams: stdout, stderr, event")),
			fields.New("level", fields.TypeStringList, fields.WithHelp("Log levels")),
			fields.New("contains", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Text substring")),
			fields.New("run", fields.TypeStringList, fields.WithHelp("Run IDs")),
			fields.New("timestamps", fields.TypeBool, fields.WithDefault(true), fields.WithHelp("Include timestamps")),
			fields.New("no-prefix", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Disable text prefixes")),
			fields.New("ansi", fields.TypeString, fields.WithDefault("auto"), fields.WithHelp("ANSI policy: auto, always, never")),
		),
		glazedcmds.WithSections(repoSection, glazedSection),
	)}, nil
}

func (c *LogsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	settings := LogsSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return errors.Wrap(err, "decode logs settings")
	}
	if settings.Follow && settings.Until != "" {
		return errors.New("E_USAGE: --follow and --until are mutually exclusive")
	}
	if settings.Tail < -1 {
		return errors.New("E_USAGE: --tail must be -1 or greater")
	}
	if settings.ANSI != "auto" && settings.ANSI != "always" && settings.ANSI != "never" {
		return errors.New("E_USAGE: --ansi must be auto, always, or never")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	controller, err := newOperatorController(repositoryContext.RepoRoot)
	if err != nil {
		return err
	}
	runIDs, err := resolveLogRuns(ctx, controller, repositoryContext.RepoRoot, settings)
	if err != nil {
		return err
	}
	query, err := logQuery(settings, runIDs)
	if err != nil {
		return err
	}
	reader := controller.Logs()
	if reader == nil {
		return errors.New("E_LOG_CORRUPT: structured log reader is unavailable")
	}

	historyQuery := query
	switch settings.Tail {
	case -1:
		historyQuery.Tail = 0
	case 0:
		historyQuery.Tail = 0
	default:
		historyQuery.Tail = settings.Tail
	}
	cursors := map[string]runlog.Cursor{}
	if settings.Tail != 0 {
		records, queryErr := reader.Query(ctx, historyQuery)
		if err := addLogRecords(ctx, processor, records, settings); err != nil {
			return err
		}
		for _, record := range records {
			cursors[record.RunID] = runlog.Cursor{RunID: record.RunID, Sequence: record.Sequence}
		}
		if queryErr != nil {
			var diagnostic *runlog.ReadError
			if !stderrors.As(queryErr, &diagnostic) ||
				diagnostic.Code != runlog.CodeLogTrailingPartial {
				return queryErr
			}
		}
	} else if settings.Follow {
		records, queryErr := reader.Query(ctx, query)
		if queryErr != nil {
			var diagnostic *runlog.ReadError
			if !stderrors.As(queryErr, &diagnostic) ||
				diagnostic.Code != runlog.CodeLogTrailingPartial {
				return queryErr
			}
		}
		for _, record := range records {
			cursors[record.RunID] = runlog.Cursor{RunID: record.RunID, Sequence: record.Sequence}
		}
	}
	if !settings.Follow {
		return nil
	}
	if len(runIDs) == 0 {
		return nil
	}
	return reader.Follow(ctx, runlog.FollowRequest{
		Query: query,
		After: cursors,
	}, processorLogSink{processor: processor, settings: settings})
}

type processorLogSink struct {
	processor middlewares.Processor
	settings  LogsSettings
}

var _ runlog.LogSink = processorLogSink{}

func (s processorLogSink) Add(ctx context.Context, record runlog.LogRecord) error {
	return s.processor.AddRow(ctx, logRow(record, s.settings))
}

func addLogRecords(
	ctx context.Context,
	processor middlewares.Processor,
	records []runlog.LogRecord,
	settings LogsSettings,
) error {
	for _, record := range records {
		if err := processor.AddRow(ctx, logRow(record, settings)); err != nil {
			return err
		}
	}
	return nil
}

func logRow(record runlog.LogRecord, settings LogsSettings) types.Row {
	text := record.Text
	if settings.ANSI == "never" {
		text = stripANSI(text)
	}
	timestamp := any(nil)
	if settings.Timestamps {
		timestamp = record.Time
	}
	prefix := ""
	if !settings.NoPrefix {
		prefix = strings.TrimSpace(record.Service + " " + string(record.Stream))
	}
	return types.NewRow(
		types.MRP("time", timestamp),
		types.MRP("run_id", record.RunID),
		types.MRP("sequence", record.Sequence),
		types.MRP("source", string(record.Source)),
		types.MRP("service", record.Service),
		types.MRP("stream", string(record.Stream)),
		types.MRP("level", record.Level),
		types.MRP("prefix", prefix),
		types.MRP("text", text),
		types.MRP("partial", record.Partial),
	)
}

func resolveLogRuns(
	ctx context.Context,
	controller operator.Controller,
	repoRoot string,
	settings LogsSettings,
) ([]string, error) {
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		return nil, err
	}
	if len(settings.RunIDs) > 0 {
		runIDs := make([]string, 0, len(settings.RunIDs))
		for _, runID := range settings.RunIDs {
			run, loadErr := store.LoadRun(ctx, runID)
			if loadErr != nil {
				return nil, &operator.OperatorError{
					Code:    operator.CodeServiceUnknown,
					Message: "run is not present in environment state",
					RunID:   runID,
				}
			}
			if len(settings.Services) > 0 && !stringSelection(settings.Services)[run.Service] {
				continue
			}
			runIDs = append(runIDs, runID)
		}
		return runIDs, nil
	}
	snapshot, err := controller.Snapshot(ctx, operator.SnapshotRequest{
		RepoRoot: repoRoot, IncludeRuns: true,
	})
	if err != nil {
		return nil, err
	}
	if !snapshot.Exists {
		if len(settings.Services) > 0 {
			return nil, &operator.OperatorError{
				Code:    operator.CodeServiceUnknown,
				Message: "service is not present because no environment state exists",
				Service: settings.Services[0],
			}
		}
		return []string{}, nil
	}
	selected := stringSelection(settings.Services)
	found := map[string]bool{}
	runIDs := make([]string, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		if len(selected) > 0 && !selected[service.Service] {
			continue
		}
		found[service.Service] = true
		if service.RunID != "" {
			runIDs = append(runIDs, service.RunID)
		}
	}
	for service := range selected {
		if !found[service] {
			return nil, &operator.OperatorError{
				Code:    operator.CodeServiceUnknown,
				Message: "service is not present in environment state",
				Service: service,
			}
		}
	}
	return runIDs, nil
}

func logQuery(settings LogsSettings, runIDs []string) (runlog.Query, error) {
	since, err := parseLogTime(settings.Since)
	if err != nil {
		return runlog.Query{}, errors.Wrap(err, "E_USAGE: parse --since")
	}
	until, err := parseLogTime(settings.Until)
	if err != nil {
		return runlog.Query{}, errors.Wrap(err, "E_USAGE: parse --until")
	}
	sources := make([]runlog.SourceKind, 0, len(settings.Sources))
	for _, source := range settings.Sources {
		switch runlog.SourceKind(source) {
		case runlog.SourceService, runlog.SourcePipeline, runlog.SourcePlugin, runlog.SourceSystem:
			sources = append(sources, runlog.SourceKind(source))
		default:
			return runlog.Query{}, errors.Errorf("E_USAGE: invalid log source %q", source)
		}
	}
	streams := make([]runlog.StreamKind, 0, len(settings.Streams))
	for _, stream := range settings.Streams {
		switch runlog.StreamKind(stream) {
		case runlog.StreamStdout, runlog.StreamStderr, runlog.StreamEvent:
			streams = append(streams, runlog.StreamKind(stream))
		default:
			return runlog.Query{}, errors.Errorf("E_USAGE: invalid log stream %q", stream)
		}
	}
	return runlog.Query{
		RunIDs: runIDs, Services: settings.Services, Sources: sources,
		Streams: streams, Levels: settings.Levels, Since: since, Until: until,
		Contains: settings.Contains,
	}, nil
}

func parseLogTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		result := time.Now().UTC().Add(-duration)
		return &result, nil
	}
	result, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func stripANSI(value string) string {
	var result strings.Builder
	result.Grow(len(value))
	state := 0
	for _, character := range value {
		switch state {
		case 0:
			if character == '\x1b' {
				state = 1
			} else {
				result.WriteRune(character)
			}
		case 1:
			if character == '[' {
				state = 2
			} else {
				state = 0
			}
		case 2:
			if character >= '@' && character <= '~' {
				state = 0
			}
		}
	}
	return result.String()
}

func newLogsCmd() *cobra.Command {
	command, err := NewLogsCommand()
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}
