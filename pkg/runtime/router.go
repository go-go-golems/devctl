package runtime

import (
	"sync"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/pkg/errors"
)

type router struct {
	mu      sync.Mutex
	pending map[string]chan protocol.Response
	streams map[string][]chan protocol.Event
	buffer  map[string][]protocol.Event
	fatal   error
}

func newRouter() *router {
	return &router{
		pending: map[string]chan protocol.Response{},
		streams: map[string][]chan protocol.Event{},
		buffer:  map[string][]protocol.Event{},
	}
}

func (r *router) register(rid string) chan protocol.Response {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan protocol.Response, 1)
	if r.fatal != nil {
		ch <- protocol.Response{
			Type:      protocol.FrameResponse,
			RequestID: rid,
			Ok:        false,
			Error: &protocol.Error{
				Code:    protocol.ErrRuntime,
				Message: errors.Wrap(r.fatal, "runtime").Error(),
			},
		}
		close(ch)
		return ch
	}
	r.pending[rid] = ch
	return ch
}

func (r *router) deliver(rid string, resp protocol.Response) {
	r.mu.Lock()
	ch, ok := r.pending[rid]
	if ok {
		delete(r.pending, rid)
	}
	r.mu.Unlock()
	if ok {
		ch <- resp
		close(ch)
	}
}

func (r *router) cancel(rid string, err error) {
	r.mu.Lock()
	ch, ok := r.pending[rid]
	if ok {
		delete(r.pending, rid)
	}
	r.mu.Unlock()
	if ok {
		msg := "canceled"
		if err != nil {
			msg = err.Error()
		}
		ch <- protocol.Response{
			Type:      protocol.FrameResponse,
			RequestID: rid,
			Ok:        false,
			Error: &protocol.Error{
				Code:    protocol.ErrCanceled,
				Message: msg,
			},
		}
		close(ch)
	}
}

func (r *router) failAll(err error) {
	r.mu.Lock()
	r.fatal = err
	pending := make(map[string]chan protocol.Response, len(r.pending))
	for rid, ch := range r.pending {
		pending[rid] = ch
	}
	r.pending = map[string]chan protocol.Response{}
	streams := r.streams
	r.streams = map[string][]chan protocol.Event{}
	r.mu.Unlock()

	for rid, ch := range pending {
		ch <- protocol.Response{
			Type:      protocol.FrameResponse,
			RequestID: rid,
			Ok:        false,
			Error: &protocol.Error{
				Code:    protocol.ErrRuntime,
				Message: errors.Wrap(err, "runtime").Error(),
			},
		}
		close(ch)
	}

	for _, subs := range streams {
		for _, ch := range subs {
			close(ch)
		}
	}
}

func (r *router) subscribe(streamID string) <-chan protocol.Event {
	r.mu.Lock()
	ch := make(chan protocol.Event, 16)
	if r.fatal != nil {
		r.mu.Unlock()
		close(ch)
		return ch
	}
	buf := append([]protocol.Event{}, r.buffer[streamID]...)
	delete(r.buffer, streamID)

	ended := false
	for _, ev := range buf {
		if ev.Event == "end" {
			ended = true
			break
		}
	}

	if ended {
		r.mu.Unlock()
		for _, ev := range buf {
			ch <- ev
		}
		close(ch)
		return ch
	}

	r.streams[streamID] = append(r.streams[streamID], ch)
	r.mu.Unlock()

	for _, ev := range buf {
		ch <- ev
	}
	return ch
}

func (r *router) publish(ev protocol.Event) {
	if ev.StreamID == "" {
		return
	}

	r.mu.Lock()
	subs := append([]chan protocol.Event{}, r.streams[ev.StreamID]...)
	if len(subs) == 0 {
		r.buffer[ev.StreamID] = append(r.buffer[ev.StreamID], ev)
	}
	r.mu.Unlock()

	if len(subs) > 0 {
		for _, ch := range subs {
			ch <- ev
		}
	}

	if ev.Event == "end" {
		r.mu.Lock()
		subs = r.streams[ev.StreamID]
		delete(r.streams, ev.StreamID)
		r.mu.Unlock()
		for _, ch := range subs {
			close(ch)
		}
	}
}
