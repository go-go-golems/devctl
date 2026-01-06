package tui

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
)

func RegisterUIActionRunner(bus *Bus, opts RootOptions) {
	bus.AddHandler("devctl-ui-actions", TopicUIActions, func(msg *message.Message) error {
		defer msg.Ack()

		var env Envelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil {
			_ = publishActionLog(bus.Publisher, "action: bad envelope (unmarshal failed)")
			return nil
		}
		if env.Type != UITypeActionRequest {
			return nil
		}

		var req ActionRequest
		if err := json.Unmarshal(env.Payload, &req); err != nil {
			_ = publishActionLog(bus.Publisher, "action: bad request (unmarshal failed)")
			return nil
		}
		if req.Kind == "" {
			return nil
		}
		if req.At.IsZero() {
			req.At = time.Now()
		}

		ctx := msg.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		_ = publishActionLog(bus.Publisher, "action start: "+string(req.Kind))
		var err error
		switch req.Kind {
		case ActionDown:
			err = runDown(ctx, opts)
		case ActionUp:
			err = runUp(ctx, opts)
		case ActionRestart:
			if err2 := runDown(ctx, opts); err2 != nil {
				err = err2
				break
			}
			err = runUp(ctx, opts)
		default:
			err = errors.Errorf("unknown action: %s", req.Kind)
		}

		if err != nil {
			_ = publishActionLog(bus.Publisher, "action failed: "+string(req.Kind)+": "+err.Error())
			return nil
		}
		_ = publishActionLog(bus.Publisher, "action ok: "+string(req.Kind))
		return nil
	})
}

func publishActionLog(pub message.Publisher, text string) error {
	ev := ActionLog{At: time.Now(), Text: text}
	env, err := NewEnvelope(DomainTypeActionLog, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func runDown(ctx context.Context, opts RootOptions) error {
	if opts.RepoRoot == "" {
		return errors.New("missing RepoRoot")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.DryRun {
		return nil
	}

	if _, err := os.Stat(state.StatePath(opts.RepoRoot)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "stat state")
	}

	st, err := state.Load(opts.RepoRoot)
	if err != nil {
		return err
	}
	sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ShutdownTimeout: opts.Timeout})

	stopCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	_ = sup.Stop(stopCtx, st)
	return state.Remove(opts.RepoRoot)
}

func runUp(ctx context.Context, opts RootOptions) error {
	if opts.RepoRoot == "" {
		return errors.New("missing RepoRoot")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	if !opts.DryRun {
		if _, err := os.Stat(state.StatePath(opts.RepoRoot)); err == nil {
			return errors.New("state exists; run down first")
		}
	}

	cfg, err := config.LoadOptional(opts.Config)
	if err != nil {
		return err
	}
	if !opts.Strict && cfg.Strictness == "error" {
		opts.Strict = true
	}

	specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: opts.RepoRoot})
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return errors.New("no plugins configured (add .devctl.yaml)")
	}

	ctx = runtime.WithRepoRoot(ctx, opts.RepoRoot)
	ctx = runtime.WithDryRun(ctx, opts.DryRun)

	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})

	clients := make([]runtime.Client, 0, len(specs))
	for _, spec := range specs {
		c, err := factory.Start(ctx, spec)
		if err != nil {
			for _, cc := range clients {
				_ = cc.Close(ctx)
			}
			return err
		}
		clients = append(clients, c)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for _, c := range clients {
			_ = c.Close(closeCtx)
		}
	}()

	p := &engine.Pipeline{
		Clients: clients,
		Opts: engine.Options{
			Strict: opts.Strict,
			DryRun: opts.DryRun,
		},
	}

	opCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	conf, err := p.MutateConfig(opCtx, patch.Config{})
	cancel()
	if err != nil {
		return err
	}

	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	_, err = p.Build(opCtx, conf, nil)
	cancel()
	if err != nil {
		return err
	}

	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	_, err = p.Prepare(opCtx, conf, nil)
	cancel()
	if err != nil {
		return err
	}

	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	vr, err := p.Validate(opCtx, conf)
	cancel()
	if err != nil {
		return err
	}
	if !vr.Valid {
		return errors.New("validation failed")
	}

	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	plan, err := p.LaunchPlan(opCtx, conf)
	cancel()
	if err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ReadyTimeout: opts.Timeout})
	st, err := sup.Start(ctx, plan)
	if err != nil {
		return err
	}
	if err := state.Save(opts.RepoRoot, st); err != nil {
		_ = sup.Stop(context.Background(), st)
		return err
	}
	return nil
}
