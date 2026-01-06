package engine

import (
	"context"
	"sort"

	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

type Options struct {
	Strict  bool
	DryRun  bool
	Timeout int64 // reserved
}

type Pipeline struct {
	Clients []runtime.Client
	Opts    Options
}

func (p *Pipeline) MutateConfig(ctx context.Context, cfg patch.Config) (patch.Config, error) {
	ordered := clientsInOrder(p.Clients)

	current := cfg
	for _, c := range ordered {
		if !c.SupportsOp("config.mutate") {
			continue
		}
		var out struct {
			ConfigPatch patch.ConfigPatch `json:"config_patch"`
		}
		if err := c.Call(ctx, "config.mutate", map[string]any{"config": current}, &out); err != nil {
			return nil, err
		}
		var err error
		current, err = patch.Apply(current, out.ConfigPatch)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func (p *Pipeline) LaunchPlan(ctx context.Context, cfg patch.Config) (LaunchPlan, error) {
	ordered := clientsInOrder(p.Clients)

	var merged LaunchPlan
	seen := map[string]int{}
	for _, c := range ordered {
		if !c.SupportsOp("launch.plan") {
			continue
		}
		var out LaunchPlan
		if err := c.Call(ctx, "launch.plan", map[string]any{"config": cfg}, &out); err != nil {
			return LaunchPlan{}, err
		}
		for _, svc := range out.Services {
			if svc.Name == "" {
				return LaunchPlan{}, errors.New("launch.plan returned service with empty name")
			}
			if idx, ok := seen[svc.Name]; ok {
				if p.Opts.Strict {
					return LaunchPlan{}, errors.Errorf("service name collision: %s", svc.Name)
				}
				merged.Services[idx] = svc
				continue
			}
			seen[svc.Name] = len(merged.Services)
			merged.Services = append(merged.Services, svc)
		}
	}
	if merged.Services == nil {
		merged.Services = []ServiceSpec{}
	}
	return merged, nil
}

func (p *Pipeline) Validate(ctx context.Context, cfg patch.Config) (ValidateResult, error) {
	ordered := clientsInOrder(p.Clients)

	merged := ValidateResult{Valid: true, Errors: []protocol.Error{}, Warnings: []protocol.Error{}}
	for _, c := range ordered {
		if !c.SupportsOp("validate.run") {
			continue
		}
		var out ValidateResult
		if err := c.Call(ctx, "validate.run", map[string]any{"config": cfg}, &out); err != nil {
			return ValidateResult{}, err
		}
		if !out.Valid {
			merged.Valid = false
		}
		merged.Errors = append(merged.Errors, out.Errors...)
		merged.Warnings = append(merged.Warnings, out.Warnings...)
	}
	return merged, nil
}

func (p *Pipeline) Build(ctx context.Context, cfg patch.Config, steps []string) (BuildResult, error) {
	ordered := clientsInOrder(p.Clients)

	var merged BuildResult
	merged.Artifacts = map[string]string{}
	stepIndex := map[string]int{}
	for _, c := range ordered {
		if !c.SupportsOp("build.run") {
			continue
		}
		var out BuildResult
		if err := c.Call(ctx, "build.run", map[string]any{"config": cfg, "steps": steps}, &out); err != nil {
			return BuildResult{}, err
		}
		for k, v := range out.Artifacts {
			merged.Artifacts[k] = v
		}
		for _, sr := range out.Steps {
			if sr.Name == "" {
				return BuildResult{}, errors.New("build.run returned step with empty name")
			}
			if idx, ok := stepIndex[sr.Name]; ok {
				if p.Opts.Strict {
					return BuildResult{}, errors.Errorf("build step collision: %s", sr.Name)
				}
				merged.Steps[idx] = sr
				continue
			}
			stepIndex[sr.Name] = len(merged.Steps)
			merged.Steps = append(merged.Steps, sr)
		}
	}
	return merged, nil
}

func (p *Pipeline) Prepare(ctx context.Context, cfg patch.Config, steps []string) (PrepareResult, error) {
	ordered := clientsInOrder(p.Clients)

	var merged PrepareResult
	merged.Artifacts = map[string]string{}
	stepIndex := map[string]int{}
	for _, c := range ordered {
		if !c.SupportsOp("prepare.run") {
			continue
		}
		var out PrepareResult
		if err := c.Call(ctx, "prepare.run", map[string]any{"config": cfg, "steps": steps}, &out); err != nil {
			return PrepareResult{}, err
		}
		for k, v := range out.Artifacts {
			merged.Artifacts[k] = v
		}
		for _, sr := range out.Steps {
			if sr.Name == "" {
				return PrepareResult{}, errors.New("prepare.run returned step with empty name")
			}
			if idx, ok := stepIndex[sr.Name]; ok {
				if p.Opts.Strict {
					return PrepareResult{}, errors.Errorf("prepare step collision: %s", sr.Name)
				}
				merged.Steps[idx] = sr
				continue
			}
			stepIndex[sr.Name] = len(merged.Steps)
			merged.Steps = append(merged.Steps, sr)
		}
	}
	return merged, nil
}

func clientsInOrder(clients []runtime.Client) []runtime.Client {
	out := append([]runtime.Client{}, clients...)
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].Spec()
		b := out[j].Spec()
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.ID < b.ID
	})
	return out
}
