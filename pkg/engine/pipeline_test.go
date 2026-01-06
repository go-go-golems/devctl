package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	spec runtime.PluginSpec
	ops  map[string]func(input any) (any, error)
}

var _ runtime.Client = (*fakeClient)(nil)

func (f *fakeClient) Spec() runtime.PluginSpec        { return f.spec }
func (f *fakeClient) Handshake() protocol.Handshake   { return protocol.Handshake{} }
func (f *fakeClient) SupportsOp(op string) bool       { _, ok := f.ops[op]; return ok }
func (f *fakeClient) Close(ctx context.Context) error { return nil }
func (f *fakeClient) StartStream(ctx context.Context, op string, input any) (string, <-chan protocol.Event, error) {
	return "", nil, errors.New("not supported")
}

func (f *fakeClient) Call(ctx context.Context, op string, input any, output any) error {
	fn, ok := f.ops[op]
	if !ok {
		return errors.New("unsupported op")
	}
	out, err := fn(input)
	if err != nil {
		return err
	}
	if output == nil {
		return nil
	}

	b, err := json.Marshal(out)
	if err != nil {
		return errors.Wrap(err, "marshal out")
	}
	if err := json.Unmarshal(b, output); err != nil {
		return errors.Wrap(err, "unmarshal into output")
	}
	return nil
}

func TestPipeline_MutateConfig_OrdersByPriorityThenID(t *testing.T) {
	var calls []string
	p := &Pipeline{
		Clients: []runtime.Client{
			&fakeClient{
				spec: runtime.PluginSpec{ID: "b", Priority: 10},
				ops: map[string]func(input any) (any, error){
					"config.mutate": func(input any) (any, error) {
						calls = append(calls, "b")
						return map[string]any{
							"config_patch": patch.ConfigPatch{Set: map[string]any{"x": "b"}},
						}, nil
					},
				},
			},
			&fakeClient{
				spec: runtime.PluginSpec{ID: "a", Priority: 10},
				ops: map[string]func(input any) (any, error){
					"config.mutate": func(input any) (any, error) {
						calls = append(calls, "a")
						return map[string]any{
							"config_patch": patch.ConfigPatch{Set: map[string]any{"x": "a"}},
						}, nil
					},
				},
			},
			&fakeClient{
				spec: runtime.PluginSpec{ID: "c", Priority: 5},
				ops: map[string]func(input any) (any, error){
					"config.mutate": func(input any) (any, error) {
						calls = append(calls, "c")
						return map[string]any{
							"config_patch": patch.ConfigPatch{Set: map[string]any{"x": "c"}},
						}, nil
					},
				},
			},
		},
	}

	cfg, err := p.MutateConfig(context.Background(), patch.Config{})
	require.NoError(t, err)
	require.Equal(t, []string{"c", "a", "b"}, calls)
	require.Equal(t, "b", cfg["x"])
}

func TestPipeline_LaunchPlan_CollisionStrict(t *testing.T) {
	p := &Pipeline{
		Opts: Options{Strict: true},
		Clients: []runtime.Client{
			&fakeClient{
				spec: runtime.PluginSpec{ID: "p1", Priority: 1},
				ops: map[string]func(input any) (any, error){
					"launch.plan": func(input any) (any, error) {
						return LaunchPlan{Services: []ServiceSpec{{Name: "svc", Command: []string{"a"}}}}, nil
					},
				},
			},
			&fakeClient{
				spec: runtime.PluginSpec{ID: "p2", Priority: 2},
				ops: map[string]func(input any) (any, error){
					"launch.plan": func(input any) (any, error) {
						return LaunchPlan{Services: []ServiceSpec{{Name: "svc", Command: []string{"b"}}}}, nil
					},
				},
			},
		},
	}
	_, err := p.LaunchPlan(context.Background(), patch.Config{})
	require.Error(t, err)
}

func TestPipeline_Validate_MergesErrorsAndANDsValid(t *testing.T) {
	p := &Pipeline{
		Clients: []runtime.Client{
			&fakeClient{
				spec: runtime.PluginSpec{ID: "v1", Priority: 1},
				ops: map[string]func(input any) (any, error){
					"validate.run": func(input any) (any, error) {
						return ValidateResult{
							Valid: true,
							Warnings: []protocol.Error{
								{Code: "W1", Message: "warn"},
							},
						}, nil
					},
				},
			},
			&fakeClient{
				spec: runtime.PluginSpec{ID: "v2", Priority: 2},
				ops: map[string]func(input any) (any, error){
					"validate.run": func(input any) (any, error) {
						return ValidateResult{
							Valid: false,
							Errors: []protocol.Error{
								{Code: "E1", Message: "err"},
							},
						}, nil
					},
				},
			},
		},
	}

	out, err := p.Validate(context.Background(), patch.Config{})
	require.NoError(t, err)
	require.False(t, out.Valid)
	require.Len(t, out.Errors, 1)
	require.Len(t, out.Warnings, 1)
}

func TestPipeline_Build_StepCollisionStrict(t *testing.T) {
	p := &Pipeline{
		Opts: Options{Strict: true},
		Clients: []runtime.Client{
			&fakeClient{
				spec: runtime.PluginSpec{ID: "b1", Priority: 1},
				ops: map[string]func(input any) (any, error){
					"build.run": func(input any) (any, error) {
						return BuildResult{Steps: []StepResult{{Name: "backend", Ok: true}}}, nil
					},
				},
			},
			&fakeClient{
				spec: runtime.PluginSpec{ID: "b2", Priority: 2},
				ops: map[string]func(input any) (any, error){
					"build.run": func(input any) (any, error) {
						return BuildResult{Steps: []StepResult{{Name: "backend", Ok: true}}}, nil
					},
				},
			},
		},
	}
	_, err := p.Build(context.Background(), patch.Config{}, []string{"backend"})
	require.Error(t, err)
}
