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

		runID := watermill.NewUUID()
		runStart := time.Now()
		_ = publishPipelineRunStarted(bus.Publisher, PipelineRunStarted{
			RunID:    runID,
			Kind:     req.Kind,
			RepoRoot: opts.RepoRoot,
			At:       runStart,
			Phases:   phasesForAction(req.Kind),
		})

		_ = publishActionLog(bus.Publisher, "action start: "+string(req.Kind))
		var err error
		switch req.Kind {
		case ActionDown:
			err = runDown(ctx, opts, bus.Publisher, runID)
		case ActionUp:
			err = runUp(ctx, opts, bus.Publisher, runID)
		case ActionRestart:
			if err2 := runDown(ctx, opts, bus.Publisher, runID); err2 != nil {
				err = err2
				break
			}
			err = runUp(ctx, opts, bus.Publisher, runID)
		default:
			err = errors.Errorf("unknown action: %s", req.Kind)
		}

		if err != nil {
			_ = publishActionLog(bus.Publisher, "action failed: "+string(req.Kind)+": "+err.Error())
			_ = publishPipelineRunFinished(bus.Publisher, PipelineRunFinished{
				RunID:      runID,
				Kind:       req.Kind,
				RepoRoot:   opts.RepoRoot,
				At:         time.Now(),
				Ok:         false,
				DurationMs: time.Since(runStart).Milliseconds(),
				Error:      err.Error(),
			})
			return nil
		}
		_ = publishActionLog(bus.Publisher, "action ok: "+string(req.Kind))
		_ = publishPipelineRunFinished(bus.Publisher, PipelineRunFinished{
			RunID:      runID,
			Kind:       req.Kind,
			RepoRoot:   opts.RepoRoot,
			At:         time.Now(),
			Ok:         true,
			DurationMs: time.Since(runStart).Milliseconds(),
		})
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

func publishPipelineRunStarted(pub message.Publisher, ev PipelineRunStarted) error {
	env, err := NewEnvelope(DomainTypePipelineRunStarted, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelineRunFinished(pub message.Publisher, ev PipelineRunFinished) error {
	env, err := NewEnvelope(DomainTypePipelineRunFinished, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelinePhaseStarted(pub message.Publisher, ev PipelinePhaseStarted) error {
	env, err := NewEnvelope(DomainTypePipelinePhaseStarted, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelinePhaseFinished(pub message.Publisher, ev PipelinePhaseFinished) error {
	env, err := NewEnvelope(DomainTypePipelinePhaseFinished, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelineBuildResult(pub message.Publisher, ev PipelineBuildResult) error {
	env, err := NewEnvelope(DomainTypePipelineBuildResult, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelinePrepareResult(pub message.Publisher, ev PipelinePrepareResult) error {
	env, err := NewEnvelope(DomainTypePipelinePrepareResult, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelineValidateResult(pub message.Publisher, ev PipelineValidateResult) error {
	env, err := NewEnvelope(DomainTypePipelineValidateResult, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func publishPipelineLaunchPlan(pub message.Publisher, ev PipelineLaunchPlan) error {
	env, err := NewEnvelope(DomainTypePipelineLaunchPlan, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	return pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func phasesForAction(kind ActionKind) []PipelinePhase {
	switch kind {
	case ActionDown:
		return []PipelinePhase{PipelinePhaseStopSupervise, PipelinePhaseRemoveState}
	case ActionRestart:
		return []PipelinePhase{
			PipelinePhaseStopSupervise,
			PipelinePhaseRemoveState,
			PipelinePhaseMutateConfig,
			PipelinePhaseBuild,
			PipelinePhasePrepare,
			PipelinePhaseValidate,
			PipelinePhaseLaunchPlan,
			PipelinePhaseSupervise,
			PipelinePhaseStateSave,
		}
	default:
		return []PipelinePhase{
			PipelinePhaseMutateConfig,
			PipelinePhaseBuild,
			PipelinePhasePrepare,
			PipelinePhaseValidate,
			PipelinePhaseLaunchPlan,
			PipelinePhaseSupervise,
			PipelinePhaseStateSave,
		}
	}
}

func runDown(ctx context.Context, opts RootOptions, pub message.Publisher, runID string) error {
	if opts.RepoRoot == "" {
		return errors.New("missing RepoRoot")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.DryRun {
		return nil
	}

	stopStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseStopSupervise, At: stopStart})
	if _, err := os.Stat(state.StatePath(opts.RepoRoot)); err != nil {
		if os.IsNotExist(err) {
			_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
				RunID:      runID,
				Phase:      PipelinePhaseStopSupervise,
				At:         time.Now(),
				Ok:         true,
				DurationMs: time.Since(stopStart).Milliseconds(),
			})
			rmStart := time.Now()
			_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseRemoveState, At: rmStart})
			_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
				RunID:      runID,
				Phase:      PipelinePhaseRemoveState,
				At:         time.Now(),
				Ok:         true,
				DurationMs: time.Since(rmStart).Milliseconds(),
			})
			return nil
		}
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseStopSupervise,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(stopStart).Milliseconds(),
			Error:      errors.Wrap(err, "stat state").Error(),
		})
		return errors.Wrap(err, "stat state")
	}

	st, err := state.Load(opts.RepoRoot)
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseStopSupervise,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(stopStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	wrapperExe, _ := os.Executable()
	sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ShutdownTimeout: opts.Timeout, WrapperExe: wrapperExe})

	stopCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	_ = sup.Stop(stopCtx, st)
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseStopSupervise,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(stopStart).Milliseconds(),
	})

	rmStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseRemoveState, At: rmStart})
	err = state.Remove(opts.RepoRoot)
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseRemoveState,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(rmStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseRemoveState,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(rmStart).Milliseconds(),
	})
	return nil
}

func runUp(ctx context.Context, opts RootOptions, pub message.Publisher, runID string) error {
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

	mutateStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseMutateConfig, At: mutateStart})
	opCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	conf, err := p.MutateConfig(opCtx, patch.Config{})
	cancel()
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseMutateConfig,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(mutateStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseMutateConfig,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(mutateStart).Milliseconds(),
	})

	buildStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseBuild, At: buildStart})
	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	br, err := p.Build(opCtx, conf, nil)
	cancel()
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseBuild,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(buildStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	_ = publishPipelineBuildResult(pub, PipelineBuildResult{
		RunID: runID,
		At:    time.Now(),
		Steps: stepResultsFromEngine(br.Steps),
	})
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseBuild,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(buildStart).Milliseconds(),
	})

	prepStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhasePrepare, At: prepStart})
	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	pr, err := p.Prepare(opCtx, conf, nil)
	cancel()
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhasePrepare,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(prepStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	_ = publishPipelinePrepareResult(pub, PipelinePrepareResult{
		RunID: runID,
		At:    time.Now(),
		Steps: stepResultsFromEngine(pr.Steps),
	})
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhasePrepare,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(prepStart).Milliseconds(),
	})

	valStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseValidate, At: valStart})
	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	vr, err := p.Validate(opCtx, conf)
	cancel()
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseValidate,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(valStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	_ = publishPipelineValidateResult(pub, PipelineValidateResult{
		RunID:    runID,
		At:       time.Now(),
		Valid:    vr.Valid,
		Errors:   vr.Errors,
		Warnings: vr.Warnings,
	})
	if !vr.Valid {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseValidate,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(valStart).Milliseconds(),
			Error:      errors.Errorf("validation failed (%d errors, %d warnings)", len(vr.Errors), len(vr.Warnings)).Error(),
		})
		return errors.Errorf("validation failed (%d errors, %d warnings)", len(vr.Errors), len(vr.Warnings))
	}
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseValidate,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(valStart).Milliseconds(),
	})

	planStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseLaunchPlan, At: planStart})
	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	plan, err := p.LaunchPlan(opCtx, conf)
	cancel()
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseLaunchPlan,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(planStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	svcNames := make([]string, 0, len(plan.Services))
	for _, svc := range plan.Services {
		svcNames = append(svcNames, svc.Name)
	}
	_ = publishPipelineLaunchPlan(pub, PipelineLaunchPlan{
		RunID:    runID,
		At:       time.Now(),
		Services: svcNames,
	})
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseLaunchPlan,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(planStart).Milliseconds(),
	})

	if opts.DryRun {
		return nil
	}

	supStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseSupervise, At: supStart})
	wrapperExe, _ := os.Executable()
	sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ReadyTimeout: opts.Timeout, WrapperExe: wrapperExe})
	st, err := sup.Start(ctx, plan)
	if err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseSupervise,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(supStart).Milliseconds(),
			Error:      err.Error(),
		})
		return err
	}
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseSupervise,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(supStart).Milliseconds(),
	})

	saveStart := time.Now()
	_ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{RunID: runID, Phase: PipelinePhaseStateSave, At: saveStart})
	if err := state.Save(opts.RepoRoot, st); err != nil {
		_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
			RunID:      runID,
			Phase:      PipelinePhaseStateSave,
			At:         time.Now(),
			Ok:         false,
			DurationMs: time.Since(saveStart).Milliseconds(),
			Error:      err.Error(),
		})
		_ = sup.Stop(context.Background(), st)
		return err
	}
	_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
		RunID:      runID,
		Phase:      PipelinePhaseStateSave,
		At:         time.Now(),
		Ok:         true,
		DurationMs: time.Since(saveStart).Milliseconds(),
	})
	return nil
}

func stepResultsFromEngine(in []engine.StepResult) []PipelineStepResult {
	out := make([]PipelineStepResult, 0, len(in))
	for _, s := range in {
		out = append(out, PipelineStepResult{
			Name:       s.Name,
			Ok:         s.Ok,
			DurationMs: s.DurationMs,
		})
	}
	return out
}
