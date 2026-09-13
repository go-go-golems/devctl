package cmds

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureProcessStdout captures framework output written directly to os.Stdout.
// Glazed's current Cobra builder owns structured-output setup and writes there
// rather than through cobra.Command.OutOrStdout.
func captureProcessStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	output := make(chan []byte, 1)
	readErr := make(chan error, 1)
	go func() {
		data, err := io.ReadAll(reader)
		output <- data
		readErr <- err
	}()

	runErr := run()
	require.NoError(t, writer.Close())
	os.Stdout = original
	data := <-output
	require.NoError(t, <-readErr)
	require.NoError(t, reader.Close())
	return string(data), runErr
}
