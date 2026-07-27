package cmds

import (
	"context"
	"encoding/json"
	"io"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/formatters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	rowmiddleware "github.com/go-go-golems/glazed/pkg/middlewares/row"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
)

type jsonLinesFormatter struct{}

var _ formatters.RowOutputFormatter = jsonLinesFormatter{}

func (jsonLinesFormatter) RegisterTableMiddlewares(*middlewares.TableProcessor) error {
	return nil
}

func (jsonLinesFormatter) RegisterRowMiddlewares(*middlewares.TableProcessor) error {
	return nil
}

func (jsonLinesFormatter) ContentType() string {
	return "application/x-ndjson"
}

func (jsonLinesFormatter) Close(context.Context, io.Writer) error {
	return nil
}

func (jsonLinesFormatter) OutputRow(_ context.Context, row types.Row, writer io.Writer) error {
	return json.NewEncoder(writer).Encode(types.RowToMap(row))
}

func newJSONLinesProcessor(
	glazedValues *values.SectionValues,
	writer io.Writer,
) (middlewares.Processor, error) {
	processor, err := glazedsettings.SetupTableProcessor(glazedValues)
	if err != nil {
		return nil, err
	}
	formatter := jsonLinesFormatter{}
	if err := formatter.RegisterRowMiddlewares(processor); err != nil {
		return nil, err
	}
	processor.AddRowMiddleware(rowmiddleware.NewOutputMiddleware(formatter, writer))
	return processor, nil
}
