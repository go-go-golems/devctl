package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRuntime_HandshakeAndCall(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "ok-python", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	var out struct {
		Pong bool `json:"pong"`
	}
	require.NoError(t, c.Call(ctx, "ping", map[string]any{"message": "hi"}, &out))
	require.True(t, out.Pong)
}

func TestRuntime_NoiseBeforeHandshakeFailsStart(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "noisy-handshake", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	_, err = f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	})
	require.Error(t, err)
}

func TestRuntime_NoiseAfterHandshakeFailsCall(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "noisy-after-handshake", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	time.Sleep(50 * time.Millisecond)

	var out struct {
		Pong bool `json:"pong"`
	}
	err = c.Call(ctx, "ping", map[string]any{"message": "hi"}, &out)
	require.Error(t, err)
}

func TestRuntime_Stream(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "stream", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	_, events, err := c.StartStream(ctx, "stream.start", map[string]any{"source": "x"})
	require.NoError(t, err)

	var messages []string
	for ev := range events {
		if ev.Event == "log" {
			messages = append(messages, ev.Message)
		}
	}
	require.Equal(t, []string{"hello", "world"}, messages)
}
