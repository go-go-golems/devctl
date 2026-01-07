package runtime

import (
	"bufio"
	"context"
	"encoding/json"
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
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = 2 * time.Second
	}
	return &Factory{opts: opts}
}

func (f *Factory) Start(ctx context.Context, spec PluginSpec, opts StartOptions) (Client, error) {
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

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(stdout)
	hs, err := readHandshake(ctx, reader, f.opts.HandshakeTimeout)
	if err != nil {
		_ = terminateProcessGroup(cmd, f.opts.ShutdownTimeout)
		return nil, err
	}

	c := newClient(spec, hs, opts.Meta, cmd, stdin, reader, stderr, f.opts.ShutdownTimeout)
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

func readHandshake(ctx context.Context, r *bufio.Reader, timeout time.Duration) (protocol.Handshake, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	line, err := readLine(ctx, r)
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

func readLine(ctx context.Context, r *bufio.Reader) ([]byte, error) {
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
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			if errors.Is(res.err, io.EOF) {
				return nil, errors.Wrap(res.err, "unexpected EOF reading line")
			}
			return nil, res.err
		}
		return res.b, nil
	}
}

func terminateProcessGroup(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = cmd.Process.Kill()
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(timeout):
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		return errors.New("timeout waiting for process to exit")
	case err := <-done:
		return err
	}
}
