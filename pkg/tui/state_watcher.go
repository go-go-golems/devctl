package tui

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
)

type StateWatcher struct {
	RepoRoot string
	Interval time.Duration
	Pub      message.Publisher

	lastAlive  map[string]bool
	lastExists bool
}

func (w *StateWatcher) Run(ctx context.Context) error {
	if w.RepoRoot == "" {
		return errors.New("missing RepoRoot")
	}
	if w.Pub == nil {
		return errors.New("missing Publisher")
	}
	if w.Interval <= 0 {
		w.Interval = 1 * time.Second
	}

	t := time.NewTicker(w.Interval)
	defer t.Stop()

	for {
		if err := w.emitSnapshot(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (w *StateWatcher) emitSnapshot(ctx context.Context) error {
	_ = ctx
	path := state.StatePath(w.RepoRoot)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			w.lastAlive = nil
			w.lastExists = false
			return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: false})
		}
		w.lastAlive = nil
		w.lastExists = true
		return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, Error: errors.Wrap(err, "stat state").Error()})
	}

	st, err := state.Load(w.RepoRoot)
	if err != nil {
		w.lastAlive = nil
		w.lastExists = true
		return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, Error: errors.Wrap(err, "load state").Error()})
	}

	alive := map[string]bool{}
	for _, s := range st.Services {
		alive[s.Name] = state.ProcessAlive(s.PID)
	}

	if w.lastExists && w.lastAlive != nil {
		for _, svc := range st.Services {
			prev := w.lastAlive[svc.Name]
			now := alive[svc.Name]
			if prev && !now {
				if err := w.publishServiceExit(ServiceExitObserved{
					Name:   svc.Name,
					PID:    svc.PID,
					When:   time.Now(),
					Reason: "process not alive",
				}); err != nil {
					return err
				}
			}
		}
	}

	w.lastAlive = alive
	w.lastExists = true

	return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, State: st, Alive: alive})
}

func (w *StateWatcher) publishSnapshot(snap StateSnapshot) error {
	env, err := NewEnvelope(DomainTypeStateSnapshot, snap)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	_ = json.Valid(b)
	return w.Pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func (w *StateWatcher) publishServiceExit(ev ServiceExitObserved) error {
	env, err := NewEnvelope(DomainTypeServiceExit, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	_ = json.Valid(b)
	return w.Pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}
