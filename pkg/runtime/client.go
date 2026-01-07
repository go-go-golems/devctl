package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Client interface {
	Spec() PluginSpec
	Handshake() protocol.Handshake
	SupportsOp(op string) bool
	Call(ctx context.Context, op string, input any, output any) error
	StartStream(ctx context.Context, op string, input any) (streamID string, events <-chan protocol.Event, err error)
	Close(ctx context.Context) error
}

type client struct {
	spec PluginSpec
	hs   protocol.Handshake
	meta RequestMeta

	cmd             *exec.Cmd
	stdin           io.WriteCloser
	stdout          *bufio.Reader
	stderr          io.ReadCloser
	shutdownTimeout time.Duration

	writerMu sync.Mutex
	router   *router
	nextID   uint64

	startOnce sync.Once
	closing   atomic.Bool
}

func newClient(spec PluginSpec, hs protocol.Handshake, meta RequestMeta, cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, stderr io.ReadCloser, shutdownTimeout time.Duration) *client {
	return &client{
		spec:            spec,
		hs:              hs,
		meta:            meta,
		cmd:             cmd,
		stdin:           stdin,
		stdout:          stdout,
		stderr:          stderr,
		shutdownTimeout: shutdownTimeout,
		router:          newRouter(),
	}
}

func (c *client) start() {
	c.startOnce.Do(func() {
		go c.readStdoutLoop()
		go c.readStderrLoop()
	})
}

func (c *client) Spec() PluginSpec                { return c.spec }
func (c *client) Handshake() protocol.Handshake   { return c.hs }
func (c *client) SupportsOp(op string) bool       { return contains(c.hs.Capabilities.Ops, op) }
func (c *client) Close(ctx context.Context) error { return c.close(ctx) }

func (c *client) Call(ctx context.Context, op string, input any, output any) error {
	if !c.SupportsOp(op) {
		return &OpError{
			PluginID: c.spec.ID,
			Op:       op,
			Code:     protocol.ErrUnsupported,
			Message:  "op not declared in handshake capabilities",
		}
	}

	rid := c.nextRequestID()
	respCh := c.router.register(rid)

	reqBytes, err := json.Marshal(input)
	if err != nil {
		return err
	}

	req := protocol.Request{
		Type:      protocol.FrameRequest,
		RequestID: rid,
		Op:        op,
		Ctx:       requestContextFrom(ctx, c.meta),
		Input:     reqBytes,
	}

	if err := c.writeFrame(req); err != nil {
		c.router.cancel(rid, err)
		return err
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return errors.New("request canceled")
		}
		if !resp.Ok {
			if resp.Error != nil {
				return &OpError{
					PluginID: c.spec.ID,
					Op:       op,
					Code:     resp.Error.Code,
					Message:  resp.Error.Message,
					Details:  resp.Error.Details,
				}
			}
			return errors.New("plugin returned ok=false without error")
		}
		if output != nil && len(resp.Output) > 0 {
			if err := json.Unmarshal(resp.Output, output); err != nil {
				return err
			}
		}
		return nil
	case <-ctx.Done():
		c.router.cancel(rid, ctx.Err())
		return ctx.Err()
	}
}

func (c *client) StartStream(ctx context.Context, op string, input any) (string, <-chan protocol.Event, error) {
	if !contains(c.hs.Capabilities.Ops, op) && !contains(c.hs.Capabilities.Streams, op) {
		return "", nil, &OpError{
			PluginID: c.spec.ID,
			Op:       op,
			Code:     protocol.ErrUnsupported,
			Message:  "op not declared in handshake capabilities",
		}
	}

	rid := c.nextRequestID()
	respCh := c.router.register(rid)

	reqBytes, err := json.Marshal(input)
	if err != nil {
		return "", nil, err
	}

	req := protocol.Request{
		Type:      protocol.FrameRequest,
		RequestID: rid,
		Op:        op,
		Ctx:       requestContextFrom(ctx, c.meta),
		Input:     reqBytes,
	}
	if err := c.writeFrame(req); err != nil {
		c.router.cancel(rid, err)
		return "", nil, err
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return "", nil, errors.New("request canceled")
		}
		if !resp.Ok {
			if resp.Error != nil {
				return "", nil, &OpError{
					PluginID: c.spec.ID,
					Op:       op,
					Code:     resp.Error.Code,
					Message:  resp.Error.Message,
					Details:  resp.Error.Details,
				}
			}
			return "", nil, errors.New("plugin returned ok=false without error")
		}
		var out struct {
			StreamID string `json:"stream_id"`
		}
		if err := json.Unmarshal(resp.Output, &out); err != nil {
			return "", nil, err
		}
		if out.StreamID == "" {
			return "", nil, errors.New("missing stream_id in response output")
		}
		events := c.router.subscribe(out.StreamID)
		return out.StreamID, events, nil

	case <-ctx.Done():
		c.router.cancel(rid, ctx.Err())
		return "", nil, ctx.Err()
	}
}

func (c *client) nextRequestID() string {
	n := atomic.AddUint64(&c.nextID, 1)
	return c.spec.ID + "-" + itoa(n)
}

func (c *client) writeFrame(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writerMu.Lock()
	defer c.writerMu.Unlock()
	_, err = c.stdin.Write(append(b, '\n'))
	return err
}

func (c *client) readStdoutLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			c.router.failAll(err)
			if !c.closing.Load() && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				log.Error().Err(err).Str("plugin", c.spec.ID).Msg("stdout read error")
			}
			return
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			continue
		}

		var envelope struct {
			Type protocol.FrameType `json:"type"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			c.router.failAll(errors.Wrapf(err, "%s: %s", protocol.ErrProtocolStdoutContamination, string(line)))
			return
		}

		switch envelope.Type {
		case protocol.FrameResponse:
			var resp protocol.Response
			if err := json.Unmarshal(line, &resp); err != nil {
				c.router.failAll(err)
				return
			}
			c.router.deliver(resp.RequestID, resp)
		case protocol.FrameEvent:
			var ev protocol.Event
			if err := json.Unmarshal(line, &ev); err != nil {
				c.router.failAll(err)
				return
			}
			c.router.publish(ev)
		case protocol.FrameHandshake, protocol.FrameRequest:
			c.router.failAll(errors.Errorf("%s: unexpected frame type %q", protocol.ErrProtocolUnexpectedFrame, envelope.Type))
			return
		default:
			c.router.failAll(errors.Errorf("%s: unknown frame type %q", protocol.ErrProtocolUnexpectedFrame, envelope.Type))
			return
		}
	}
}

func (c *client) readStderrLoop() {
	r := bufio.NewReader(c.stderr)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if c.closing.Load() || errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				return
			}
			log.Error().Err(err).Str("plugin", c.spec.ID).Msg("stderr read error")
			return
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			continue
		}
		log.Info().Str("plugin", c.spec.ID).Msg(string(line))
	}
}

func (c *client) close(ctx context.Context) error {
	if c.cmd == nil {
		return nil
	}
	c.closing.Store(true)
	_ = c.stdin.Close()
	_ = terminateProcessGroup(c.cmd, c.shutdownTimeout)
	return nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func requestContextFrom(ctx context.Context, meta RequestMeta) protocol.RequestContext {
	rc := protocol.RequestContext{}
	if deadline, ok := ctx.Deadline(); ok {
		rc.DeadlineMs = time.Until(deadline).Milliseconds()
		if rc.DeadlineMs < 0 {
			rc.DeadlineMs = 0
		}
	}
	rc.RepoRoot = meta.RepoRoot
	rc.Cwd = meta.Cwd
	rc.DryRun = meta.DryRun
	return rc
}
