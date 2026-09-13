package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/pkg/errors"
)

type PluginSpec struct {
	ID       string
	Path     string
	Args     []string
	Env      map[string]string
	WorkDir  string
	Priority int
}

type FactoryOptions struct {
	HandshakeTimeout time.Duration
	EOFGraceTimeout  time.Duration
	ShutdownTimeout  time.Duration
}

type Factory struct {
	opts FactoryOptions
}

type StartOptions struct {
	Meta RequestMeta
}

func NewFactory(opts FactoryOptions) *Factory {
	if opts.HandshakeTimeout <= 0 {
		opts.HandshakeTimeout = 2 * time.Second
	}
	if opts.EOFGraceTimeout <= 0 {
		opts.EOFGraceTimeout = 250 * time.Millisecond
	}
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = 2 * time.Second
	}
	return &Factory{opts: opts}
}

func (f *Factory) Start(ctx context.Context, spec PluginSpec, opts StartOptions) (Client, error) {
	// #nosec G204 -- plugin path and args are defined by the repo configuration.
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	cmd.Dir = spec.WorkDir
	cmd.Env = mergeEnv(os.Environ(), spec.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := enableChildSubreaper(); err != nil {
		return nil, errors.Wrap(err, "enable plugin child subreaper")
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to start plugin %q (%s %s)", spec.ID, spec.Path, strings.Join(spec.Args, " "))
	}

	lifetime := newProcessLifetime(cmd)
	reader := bufio.NewReader(stdout)
	hs, err := readHandshake(ctx, reader, stdout.Close, f.opts.HandshakeTimeout)
	if err != nil {
		// readHandshake synchronizes a canceled read before returning. Drain stderr
		// to completion before allowing Cmd.Wait to close the remaining pipe.
		stderrDone := make(chan struct{})
		stderrTailResult := make(chan string, 1)
		go func() {
			defer close(stderrDone)
			stderrTailResult <- drainStderr(stderr, 2*time.Second, 4096)
		}()
		lifetime.startWaitAfter(stderrDone)
		lifetime.beginShutdown(stdin.Close, f.opts.EOFGraceTimeout, f.opts.ShutdownTimeout)
		stderrTail := <-stderrTailResult
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), f.opts.EOFGraceTimeout+2*f.opts.ShutdownTimeout+time.Second)
		_, cleanupErr := lifetime.awaitShutdown(cleanupContext)
		cleanupCancel()
		pluginCmd := fmt.Sprintf("%s %s", spec.Path, strings.Join(spec.Args, " "))
		if stderrTail != "" {
			return nil, stderrors.Join(errors.Errorf("plugin %q (%s) failed handshake: %v\n\nstderr:\n%s", spec.ID, pluginCmd, err, stderrTail), cleanupErr)
		}
		return nil, stderrors.Join(errors.Errorf("plugin %q (%s) failed handshake: %v", spec.ID, pluginCmd, err), cleanupErr)
	}

	c := newClient(spec, hs, opts.Meta, cmd, stdin, reader, stderr, lifetime, f.opts.EOFGraceTimeout, f.opts.ShutdownTimeout)
	c.start()
	return c, nil
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := append([]string{}, base...)
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

func readHandshake(ctx context.Context, r *bufio.Reader, closeReader func() error, timeout time.Duration) (protocol.Handshake, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	line, err := readLine(ctx, r, closeReader)
	if err != nil {
		return protocol.Handshake{}, err
	}

	var hs protocol.Handshake
	if err := json.Unmarshal(line, &hs); err != nil {
		return protocol.Handshake{}, errors.Wrapf(err, "%s: %s", protocol.ErrProtocolInvalidJSON, string(line))
	}
	if err := protocol.ValidateHandshake(hs); err != nil {
		return protocol.Handshake{}, err
	}
	return hs, nil
}

func readLine(ctx context.Context, r *bufio.Reader, closeReader func() error) ([]byte, error) {
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := r.ReadBytes('\n')
		if err == nil {
			// trim trailing newline
			if len(b) > 0 && b[len(b)-1] == '\n' {
				b = b[:len(b)-1]
			}
			b = []byte(strings.TrimSpace(string(b)))
		}
		ch <- result{b: b, err: err}
	}()

	select {
	case <-ctx.Done():
		if closeReader != nil {
			_ = closeReader()
		}
		// Synchronize the canceled pipe consumer before Cmd.Wait may close pipes.
		<-ch
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			if errors.Is(res.err, io.EOF) {
				return nil, errors.New("plugin process exited before sending handshake (the executable may be missing, the script path may be wrong, or the plugin crashed on startup)")
			}
			return nil, res.err
		}
		return res.b, nil
	}
}

// drainStderr reads up to maxBytes from stderr with a timeout.
// It is best-effort and returns whatever it could read.
func drainStderr(stderr io.ReadCloser, timeout time.Duration, maxBytes int) string {
	type result struct {
		n   int
		buf []byte
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, maxBytes)
		n, _ := io.ReadFull(stderr, buf)
		if n == 0 {
			n, _ = stderr.Read(buf)
		}
		ch <- result{n: n, buf: buf}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		_ = stderr.Close()
		res := <-ch
		if res.n > 0 {
			return string(res.buf[:res.n])
		}
		return ""
	case res := <-ch:
		if res.n > 0 {
			return string(res.buf[:res.n])
		}
		return ""
	}
}
