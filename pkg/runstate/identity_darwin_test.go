//go:build darwin

package runstate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDarwinProcessIdentityMatchesPIDAndStartToken(t *testing.T) {
	identity, err := ReadProcessIdentity(os.Getpid())
	require.NoError(t, err)
	require.NotEmpty(t, identity.StartToken)

	status, err := InspectProcess(identity)
	require.NoError(t, err)
	require.Equal(t, ProcessMatches, status)

	wrong := *identity
	wrong.StartToken += "-wrong"
	status, err = InspectProcess(&wrong)
	require.NoError(t, err)
	require.Equal(t, ProcessMismatch, status)
}
