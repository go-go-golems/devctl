package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
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

type StreamSettings struct {
	PluginID    string `glazed:"plugin"`
	Operation   string `glazed:"op"`
	InputJSON   string `glazed:"input-json"`
	InputFile   string `glazed:"input-file"`
	StartTimout string `glazed:"start-timeout"`
}

type StreamCommand struct {
	*glazedcmds.CommandDescription
}

var _ glazedcmds.GlazeCommand = (*StreamCommand)(nil)

func newStreamCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "stream",
		Short: "Start and inspect protocol streams",
	}
	streamCommand, err := NewStreamCommand()
	cobra.CheckErr(err)
	command.AddCommand(buildGlazedCommand(streamCommand))
	return command
}

func NewStreamCommand() (*StreamCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	return &StreamCommand{CommandDescription: glazedcmds.NewCommandDescription(
		"start",
		glazedcmds.WithShort("Start a stream operation and emit its events"),
		glazedcmds.WithFlags(
			fields.New("plugin", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Explicit plugin ID")),
			fields.New("op", fields.TypeString, fields.WithDefault(""), fields.WithHelp("Stream operation name")),
			fields.New("input-json", fields.TypeString, fields.WithDefault(""), fields.WithHelp("JSON object passed to the operation")),
			fields.New("input-file", fields.TypeString, fields.WithDefault(""), fields.WithHelp("JSON input file")),
			fields.New("start-timeout", fields.TypeString, fields.WithDefault("2s"), fields.WithHelp("Stream handshake timeout")),
		),
		glazedcmds.WithSections(repoSection),
	)}, nil
}

func (c *StreamCommand) PrepareGlazedValues(vals *values.Values) error {
	outputValues, exists := vals.Get(glazedsettings.GlazedSlug)
	if !exists {
		return errors.New("glazed output settings are unavailable")
	}
	settings, err := glazedsettings.NewOutputFormatterSettings(outputValues)
	if err != nil {
		return err
	}
	if humanStreamOutput(settings) {
		return nil
	}
	if value, ok := outputValues.Fields.Get("stream"); ok {
		value.Value = true
	}
	if value, ok := outputValues.Fields.Get("output-as-objects"); ok {
		value.Value = true
	}
	return nil
}

func (c *StreamCommand) BuildGlazedProcessor(
	vals *values.Values,
	writer io.Writer,
) (middlewares.Processor, bool, error) {
	outputValues, exists := vals.Get(glazedsettings.GlazedSlug)
	if !exists {
		return nil, false, errors.New("glazed output settings are unavailable")
	}
	settings, err := glazedsettings.NewOutputFormatterSettings(outputValues)
	if err != nil {
		return nil, false, err
	}
	if !humanStreamOutput(settings) {
		if settings.Output == "json" {
			processor, err := newJSONLinesProcessor(outputValues, writer)
			return processor, true, err
		}
		return nil, false, nil
	}
	return &humanEventProcessor{writer: writer}, true, nil
}

func humanStreamOutput(settings *glazedsettings.OutputFormatterSettings) bool {
	return settings.Output == "table" &&
		settings.TableFormat != "csv" &&
		settings.TableFormat != "tsv" &&
		settings.TableFormat != "markdown" &&
		settings.TableFormat != "html"
}

func (c *StreamCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	settings := StreamSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return errors.Wrap(err, "decode stream settings")
	}
	if settings.Operation == "" {
		return errors.New("E_USAGE: --op is required")
	}
	startTimeout, err := time.ParseDuration(settings.StartTimout)
	if err != nil {
		return errors.Wrap(err, "E_USAGE: parse --start-timeout")
	}
	if startTimeout <= 0 {
		return errors.New("E_USAGE: --start-timeout must be greater than zero")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	repo, err := repository.Load(repository.Options{
		RepoRoot: repositoryContext.RepoRoot, ConfigPath: repositoryContext.ConfigPath,
		ProfileName: repositoryContext.Profile, Cwd: repositoryContext.Cwd,
		DryRun: repositoryContext.DryRun,
	})
	if err != nil {
		return err
	}
	if len(repo.Specs) == 0 {
		return errors.New("no plugins configured (add .devctl.yaml)")
	}
	input, err := loadStreamInput(settings.InputJSON, settings.InputFile)
	if err != nil {
		return err
	}
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})
	client, spec, err := selectStreamProvider(
		ctx, factory, repo.Specs, repo.Request, settings.PluginID, settings.Operation,
	)
	if err != nil {
		return err
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Close(closeContext)
	}()
	if !client.SupportsOp(settings.Operation) {
		return &runtime.OpError{
			PluginID: spec.ID, Op: settings.Operation,
			Code: protocol.ErrUnsupported, Message: "op not declared in handshake capabilities",
		}
	}
	startContext, cancel := context.WithTimeout(ctx, startTimeout)
	streamID, events, err := client.StartStream(startContext, settings.Operation, input)
	cancel()
	if err != nil {
		return err
	}
	if err := processor.AddRow(ctx, streamHeaderRow(spec.ID, settings.Operation, streamID)); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := processor.AddRow(ctx, streamEventRow(spec.ID, settings.Operation, streamID, event)); err != nil {
				return err
			}
		}
	}
}

func streamHeaderRow(pluginID, operation, streamID string) types.Row {
	return types.NewRow(
		types.MRP("kind", "stream"),
		types.MRP("plugin_id", pluginID),
		types.MRP("op", operation),
		types.MRP("stream_id", streamID),
	)
}

func streamEventRow(pluginID, operation, streamID string, event protocol.Event) types.Row {
	return types.NewRow(
		types.MRP("kind", "event"),
		types.MRP("plugin_id", pluginID),
		types.MRP("op", operation),
		types.MRP("stream_id", streamID),
		types.MRP("event", event.Event),
		types.MRP("level", event.Level),
		types.MRP("message", event.Message),
		types.MRP("fields", event.Fields),
		types.MRP("ok", event.Ok),
	)
}

type humanEventProcessor struct {
	writer io.Writer
}

var _ middlewares.Processor = (*humanEventProcessor)(nil)

func (p *humanEventProcessor) AddRow(_ context.Context, row types.Row) error {
	kind, _ := row.Get("kind")
	if kind == "stream" {
		_, err := fmt.Fprintf(
			p.writer, "plugin=%v op=%v stream_id=%v\n",
			rowValue(row, "plugin_id"), rowValue(row, "op"), rowValue(row, "stream_id"),
		)
		return err
	}
	event := protocol.Event{
		Event:   stringRowValue(row, "event"),
		Level:   stringRowValue(row, "level"),
		Message: stringRowValue(row, "message"),
	}
	if fieldsValue, ok := row.Get("fields"); ok {
		event.Fields, _ = fieldsValue.(map[string]any)
	}
	if okValue, exists := row.Get("ok"); exists {
		event.Ok, _ = okValue.(*bool)
	}
	_, err := fmt.Fprintln(p.writer, formatProtocolEvent(event))
	return err
}

func (p *humanEventProcessor) Close(context.Context) error {
	return nil
}

func rowValue(row types.Row, key string) any {
	value, _ := row.Get(key)
	return value
}

func stringRowValue(row types.Row, key string) string {
	value, _ := row.Get(key)
	result, _ := value.(string)
	return result
}

func loadStreamInput(inputJSON, inputFile string) (map[string]any, error) {
	if inputJSON != "" && inputFile != "" {
		return nil, errors.New("E_USAGE: use only one of --input-json or --input-file")
	}
	if inputJSON == "" && inputFile == "" {
		return map[string]any{}, nil
	}
	var data []byte
	if inputFile != "" {
		var err error
		data, err = os.ReadFile(inputFile)
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(inputJSON)
	}
	var output map[string]any
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, errors.Wrap(err, "E_USAGE: decode stream input")
	}
	if output == nil {
		output = map[string]any{}
	}
	return output, nil
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
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Priority != ordered[right].Priority {
			return ordered[left].Priority < ordered[right].Priority
		}
		return ordered[left].ID < ordered[right].ID
	})
	if pluginID != "" {
		for _, spec := range ordered {
			if spec.ID == pluginID {
				client, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: meta})
				if err != nil {
					return nil, runtime.PluginSpec{}, err
				}
				return client, spec, nil
			}
		}
		return nil, runtime.PluginSpec{}, errors.Errorf("E_USAGE: unknown plugin id %q", pluginID)
	}
	for _, spec := range ordered {
		client, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: meta})
		if err != nil {
			continue
		}
		if client.SupportsOp(op) {
			return client, spec, nil
		}
		closeContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.Close(closeContext)
		cancel()
	}
	return nil, runtime.PluginSpec{}, errors.Errorf("no configured plugin supports op %q", op)
}

func formatProtocolEvent(event protocol.Event) string {
	if event.Event == "end" {
		if event.Ok == nil {
			return "[end]"
		}
		return fmt.Sprintf("[end ok=%v]", *event.Ok)
	}
	message := strings.TrimSpace(event.Message)
	if message == "" && len(event.Fields) > 0 {
		if data, err := json.Marshal(event.Fields); err == nil {
			message = string(data)
		}
	}
	if message == "" {
		message = "-"
	}
	if event.Level != "" {
		return fmt.Sprintf("[%s level=%s] %s", event.Event, event.Level, message)
	}
	return fmt.Sprintf("[%s] %s", event.Event, message)
}
