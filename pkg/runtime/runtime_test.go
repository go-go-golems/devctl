package runtime

import (
	"context"
	"errors"
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
	}, StartOptions{})
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
	}, StartOptions{})
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
	}, StartOptions{})
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
	}, StartOptions{})
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

func TestRuntime_CallTimeout(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "timeout", "plugin.py")

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(startCtx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	}, StartOptions{})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	var out struct {
		Pong bool `json:"pong"`
	}
	callCtx, callCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer callCancel()
	err = c.Call(callCtx, "ping", map[string]any{"message": "hi"}, &out)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRuntime_StreamClosesOnClientClose(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "long-running-plugin", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	}, StartOptions{})
	require.NoError(t, err)

	_, events, err := c.StartStream(ctx, "logs.follow", map[string]any{})
	require.NoError(t, err)

	select {
	case <-events:
	case <-time.After(1 * time.Second):
		require.FailNow(t, "expected at least one event before close")
	}

	require.NoError(t, c.Close(context.Background()))

	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-time.After(50 * time.Millisecond):
			if time.Now().After(deadline) {
				require.FailNow(t, "expected events channel to close after client close")
			}
		}
	}
}

func TestRuntime_CallUnsupportedFailsFast(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "ignore-unknown", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	}, StartOptions{})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	callCtx, callCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer callCancel()

	err = c.Call(callCtx, "unknown.op", map[string]any{}, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrUnsupported))
	require.False(t, errors.Is(err, context.DeadlineExceeded))

	var opErr *OpError
	require.True(t, errors.As(err, &opErr))
	require.Equal(t, "unknown.op", opErr.Op)
	require.Equal(t, "t", opErr.PluginID)
	require.Equal(t, "E_UNSUPPORTED", opErr.Code)
}

func TestRuntime_StartStreamUnsupportedFailsFast(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "ignore-unknown", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	}, StartOptions{})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	callCtx, callCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer callCancel()

	_, _, err = c.StartStream(callCtx, "unknown.op", map[string]any{})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrUnsupported))
	require.False(t, errors.Is(err, context.DeadlineExceeded))

	var opErr *OpError
	require.True(t, errors.As(err, &opErr))
	require.Equal(t, "unknown.op", opErr.Op)
	require.Equal(t, "t", opErr.PluginID)
	require.Equal(t, "E_UNSUPPORTED", opErr.Code)
}

func TestRuntime_StartStreamIgnoresStreamsCapabilityForInvocation(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "streams-only-never-respond", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	}, StartOptions{})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	callCtx, callCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer callCancel()

	_, _, err = c.StartStream(callCtx, "telemetry.stream", map[string]any{"count": 1})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrUnsupported))
	require.False(t, errors.Is(err, context.DeadlineExceeded))
}

func TestRuntime_TelemetryStreamFixture(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	plugin := filepath.Join(repoRoot, "..", "..", "testdata", "plugins", "telemetry", "plugin.py")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f := NewFactory(FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := f.Start(ctx, PluginSpec{
		ID:      "t",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: repoRoot,
	}, StartOptions{})
	require.NoError(t, err)
	defer func() { _ = c.Close(context.Background()) }()

	_, events, err := c.StartStream(ctx, "telemetry.stream", map[string]any{"count": 3, "interval_ms": 1})
	require.NoError(t, err)

	var got []int
	for ev := range events {
		if ev.Event == "metric" && ev.Fields != nil {
			if name, ok := ev.Fields["name"].(string); ok && name == "counter" {
				switch v := ev.Fields["value"].(type) {
				case float64:
					got = append(got, int(v))
				case int:
					got = append(got, v)
				}
			}
		}
	}
	require.Equal(t, []int{0, 1, 2}, got)
}
