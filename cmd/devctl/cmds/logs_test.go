package cmds

import (
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/stretchr/testify/require"
)

func TestLogsFollowJSONForcesIndividualObjects(t *testing.T) {
	command, err := NewLogsCommand()
	require.NoError(t, err)
	logSection, exists := command.Description().Schema.Get(schema.DefaultSlug)
	require.True(t, exists)
	glazedSection, exists := command.Description().Schema.Get(glazedsettings.GlazedSlug)
	require.True(t, exists)

	logValues, err := values.NewSectionValues(
		logSection,
		values.WithFieldValue("follow", true),
	)
	require.NoError(t, err)
	outputValues, err := values.NewSectionValues(
		glazedSection,
		values.WithFieldValue("output", "json"),
		values.WithFieldValue("output-as-objects", false),
	)
	require.NoError(t, err)
	parsed := values.New(
		values.WithSectionValues(schema.DefaultSlug, logValues),
		values.WithSectionValues(glazedsettings.GlazedSlug, outputValues),
	)

	require.NoError(t, command.PrepareGlazedValues(parsed))
	value, exists := outputValues.GetField("output-as-objects")
	require.True(t, exists)
	require.Equal(t, true, value)
}

func TestLogsNonFollowJSONKeepsArrayOutput(t *testing.T) {
	command, err := NewLogsCommand()
	require.NoError(t, err)
	logSection, exists := command.Description().Schema.Get(schema.DefaultSlug)
	require.True(t, exists)
	glazedSection, exists := command.Description().Schema.Get(glazedsettings.GlazedSlug)
	require.True(t, exists)

	logValues, err := values.NewSectionValues(
		logSection,
		values.WithFieldValue("follow", false),
	)
	require.NoError(t, err)
	outputValues, err := values.NewSectionValues(
		glazedSection,
		values.WithFieldValue("output", "json"),
		values.WithFieldValue("output-as-objects", false),
	)
	require.NoError(t, err)
	parsed := values.New(
		values.WithSectionValues(schema.DefaultSlug, logValues),
		values.WithSectionValues(glazedsettings.GlazedSlug, outputValues),
	)

	require.NoError(t, command.PrepareGlazedValues(parsed))
	value, exists := outputValues.GetField("output-as-objects")
	require.True(t, exists)
	require.Equal(t, false, value)
}
