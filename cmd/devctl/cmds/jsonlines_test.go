package cmds

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestJSONLinesFormatterWritesOneCompactObjectPerRow(t *testing.T) {
	var output bytes.Buffer
	formatter := jsonLinesFormatter{}
	require.NoError(t, formatter.OutputRow(
		context.Background(),
		types.NewRow(types.MRP("sequence", 1), types.MRP("text", "first")),
		&output,
	))
	require.NoError(t, formatter.OutputRow(
		context.Background(),
		types.NewRow(types.MRP("sequence", 2), types.MRP("text", "second")),
		&output,
	))

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	for index, line := range lines {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		require.Equal(t, float64(index+1), row["sequence"])
		require.NotContains(t, line, "\n")
	}
}
