//go:build linux

package runstate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseProcStatStartTimeHandlesSpacesAndClosingParenthesis(t *testing.T) {
	stat := []byte("123 (worker name )) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 424242")
	startTime, err := parseProcStatStartTime(stat)
	require.NoError(t, err)
	require.Equal(t, "424242", startTime)
}

func TestProcessIdentityMatchesPIDAndStartToken(t *testing.T) {
	identity, err := ReadProcessIdentity(os.Getpid())
	require.NoError(t, err)

	matches, err := MatchesProcess(identity)
	require.NoError(t, err)
	require.True(t, matches)

	wrong := *identity
	wrong.StartToken += "-wrong"
	matches, err = MatchesProcess(&wrong)
	require.NoError(t, err)
	require.False(t, matches)
}
