package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
)

type streamHandle struct {
	key      string
	pluginID string
	op       string
	streamID string

	cancel context.CancelFunc
	client runtime.Client

	stoppedByUser bool
}

type streamManager struct {
	mu      sync.Mutex
	byKey   map[string]*streamHandle
	factory *runtime.Factory
	opts    RootOptions
	pub     message.Publisher
	tuiCtx  context.Context
}

func RegisterUIStreamRunner(tuiCtx context.Context, bus *Bus, opts RootOptions) {
	if tuiCtx == nil {
		tuiCtx = context.Background()
	}
	m := &streamManager{
		byKey: map[string]*streamHandle{},
		factory: runtime.NewFactory(runtime.FactoryOptions{
			HandshakeTimeout: 2 * time.Second,
			ShutdownTimeout:  3 * time.Second,
		}),
		opts:   opts,
		pub:    bus.Publisher,
		tuiCtx: tuiCtx,
	}

	bus.AddHandler("devctl-ui-streams", TopicUIActions, func(msg *message.Message) error {
		defer msg.Ack()

		var env Envelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil {
			return nil
		}

		switch env.Type {
		case UITypeStreamStartRequest:
			var req StreamStartRequest
			if err := json.Unmarshal(env.Payload, &req); err != nil {
				return nil
			}
			return m.handleStart(req)
		case UITypeStreamStopRequest:
			var req StreamStopRequest
			if err := json.Unmarshal(env.Payload, &req); err != nil {
				return nil
			}
			return m.handleStop(req)
		default:
			return nil
		}
	})
}

func (m *streamManager) handleStart(req StreamStartRequest) error {
	if m.opts.RepoRoot == "" {
		return nil
	}
	if req.Op == "" {
		return nil
	}

	streamCtx, cancel := context.WithCancel(m.tuiCtx)
	created := false
	defer func() {
		if !created {
			cancel()
		}
	}()

	repo, err := repository.Load(repository.Options{RepoRoot: m.opts.RepoRoot, ConfigPath: m.opts.Config, Cwd: m.opts.RepoRoot, DryRun: m.opts.DryRun})
	if err != nil {
		_ = m.publishStreamEnded(StreamEnded{
			StreamKey: streamKey(req.PluginID, req.Op, req.Input),
			PluginID:  req.PluginID,
			Op:        req.Op,
			At:        time.Now(),
			Ok:        false,
			Error:     err.Error(),
		})
		return nil
	}

	pluginID := req.PluginID
	specs := orderedSpecs(repo.Specs)

	var client runtime.Client
	if pluginID != "" {
		var spec runtime.PluginSpec
		found := false
		for _, s := range specs {
			if s.ID == pluginID {
				spec = s
				found = true
				break
			}
		}
		if !found {
			_ = m.publishStreamEnded(StreamEnded{
				StreamKey: streamKey(pluginID, req.Op, req.Input),
				PluginID:  pluginID,
				Op:        req.Op,
				At:        time.Now(),
				Ok:        false,
				Error:     "unknown plugin id",
			})
			return nil
		}
		// NOTE: Use the TUI-scoped stream context for plugin lifetime.
		client, err = m.factory.Start(streamCtx, spec, runtime.StartOptions{Meta: repo.Request})
		if err != nil {
			_ = m.publishStreamEnded(StreamEnded{
				StreamKey: streamKey(pluginID, req.Op, req.Input),
				PluginID:  pluginID,
				Op:        req.Op,
				At:        time.Now(),
				Ok:        false,
				Error:     err.Error(),
			})
			return nil
		}
		if !client.SupportsOp(req.Op) {
			closeClient(client)
			_ = m.publishStreamEnded(StreamEnded{
				StreamKey: streamKey(pluginID, req.Op, req.Input),
				PluginID:  pluginID,
				Op:        req.Op,
				At:        time.Now(),
				Ok:        false,
				Error:     "op not declared in handshake capabilities",
			})
			return nil
		}
	} else {
		for _, spec := range specs {
			c, err := m.factory.Start(streamCtx, spec, runtime.StartOptions{Meta: repo.Request})
			if err != nil {
				continue
			}
			if c.SupportsOp(req.Op) {
				client = c
				pluginID = spec.ID
				break
			}
			closeClient(c)
		}
		if client == nil {
			_ = m.publishStreamEnded(StreamEnded{
				StreamKey: streamKey("", req.Op, req.Input),
				PluginID:  "",
				Op:        req.Op,
				At:        time.Now(),
				Ok:        false,
				Error:     "no configured plugin supports op",
			})
			return nil
		}
	}

	key := streamKey(pluginID, req.Op, req.Input)

	m.mu.Lock()
	if m.byKey[key] != nil {
		m.mu.Unlock()
		closeClient(client)
		return nil
	}
	h := &streamHandle{
		key:      key,
		pluginID: pluginID,
		op:       req.Op,
		cancel:   cancel,
		client:   client,
	}
	m.byKey[key] = h
	m.mu.Unlock()
	created = true

	startCtx, startCancel := context.WithTimeout(streamCtx, 2*time.Second)
	streamID, events, err := client.StartStream(startCtx, req.Op, req.Input)
	startCancel()
	if err != nil {
		m.mu.Lock()
		delete(m.byKey, key)
		m.mu.Unlock()
		closeClient(client)
		_ = m.publishStreamEnded(StreamEnded{
			StreamKey: key,
			PluginID:  pluginID,
			Op:        req.Op,
			At:        time.Now(),
			Ok:        false,
			Error:     err.Error(),
		})
		return nil
	}
	h.streamID = streamID

	_ = m.publishStreamStarted(StreamStarted{
		StreamKey: key,
		PluginID:  pluginID,
		Op:        req.Op,
		StreamID:  streamID,
		At:        time.Now(),
	})

	go m.forwardEvents(streamCtx, h, events)
	return nil
}

func (m *streamManager) handleStop(req StreamStopRequest) error {
	m.mu.Lock()
	h := m.byKey[req.StreamKey]
	if h != nil {
		h.stoppedByUser = true
		delete(m.byKey, req.StreamKey)
	}
	m.mu.Unlock()

	if h == nil {
		return nil
	}
	if h.cancel != nil {
		h.cancel()
	}
	closeClient(h.client)
	return nil
}

func (m *streamManager) forwardEvents(ctx context.Context, h *streamHandle, events <-chan protocol.Event) {
	var endOk *bool

	// Ensure cleanup and ended event no matter how we exit.
	defer func() {
		if h.cancel != nil {
			h.cancel()
		}
		closeClient(h.client)
		m.mu.Lock()
		_, still := m.byKey[h.key]
		if still {
			delete(m.byKey, h.key)
		}
		m.mu.Unlock()

		ok := true
		errText := ""
		if endOk != nil {
			ok = *endOk
		} else if h.stoppedByUser || ctx.Err() != nil {
			ok = false
			if h.stoppedByUser {
				errText = "stopped"
			} else {
				errText = ctx.Err().Error()
			}
		}
		_ = m.publishStreamEnded(StreamEnded{
			StreamKey: h.key,
			PluginID:  h.pluginID,
			Op:        h.op,
			StreamID:  h.streamID,
			At:        time.Now(),
			Ok:        ok,
			Error:     errText,
		})
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			_ = m.publishStreamEvent(StreamEvent{
				StreamKey: h.key,
				PluginID:  h.pluginID,
				Op:        h.op,
				StreamID:  h.streamID,
				At:        time.Now(),
				Event:     ev,
			})
			if ev.Event == "end" {
				endOk = ev.Ok
				return
			}
		}
	}
}

func closeClient(client runtime.Client) {
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = client.Close(ctx)
}

func orderedSpecs(specs []runtime.PluginSpec) []runtime.PluginSpec {
	out := append([]runtime.PluginSpec{}, specs...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func streamKey(pluginID, op string, input map[string]any) string {
	h := sha256.New()
	_, _ = h.Write([]byte(pluginID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(op))
	_, _ = h.Write([]byte{0})
	if input != nil {
		if b, err := json.Marshal(input); err == nil {
			_, _ = h.Write(b)
		}
	}
	return pluginID + ":" + op + ":" + hex.EncodeToString(h.Sum(nil))[:12]
}

func (m *streamManager) publishStreamStarted(ev StreamStarted) error {
	env, err := NewEnvelope(DomainTypeStreamStarted, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return m.pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func (m *streamManager) publishStreamEvent(ev StreamEvent) error {
	env, err := NewEnvelope(DomainTypeStreamEvent, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return m.pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func (m *streamManager) publishStreamEnded(ev StreamEnded) error {
	env, err := NewEnvelope(DomainTypeStreamEnded, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return m.pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}
