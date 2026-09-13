package operator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPipelinePlannerResolveDoesNotExecutePlugins(t *testing.T) {
	repoRoot := t.TempDir()
	marker := filepath.Join(repoRoot, "started.txt")
	plugin := filepath.Join(repoRoot, "plugin.py")
	code := `import json, pathlib, sys
pathlib.Path(sys.argv[1]).write_text("started", encoding="utf-8")
print(json.dumps({"type":"handshake","protocol_version":"v2","plugin_name":"resolve-test","capabilities":{"ops":[]}}), flush=True)
for _ in sys.stdin: pass
`
	if err := os.WriteFile(plugin, []byte(code), 0o600); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	config := "plugins:\n  - id: resolve-test\n    path: python3\n    args: [" + plugin + ", " + marker + "]\n"
	if err := os.WriteFile(filepath.Join(repoRoot, ".devctl.yaml"), []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	recipe, err := (PipelinePlanner{}).ResolveRecipe(t.Context(), "restart", UpRequest{
		RepoRoot: repoRoot,
		Select:   Selection{Services: []string{"api"}},
		Policy: PipelinePolicy{
			SkipPrepare: true,
			BuildSteps:  []string{"backend"},
		},
	})
	if err != nil {
		t.Fatalf("resolve recipe: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("plugin executed while resolving recipe: %v", err)
	}
	if recipe.Version != LifecycleRecipeSchemaVersion || recipe.Operation != "restart" {
		t.Fatalf("unexpected recipe identity: %#v", recipe)
	}
	if len(recipe.Phases) != 5 || recipe.Phases[2].Enabled {
		t.Fatalf("unexpected recipe phases: %#v", recipe.Phases)
	}
	if got := recipe.Phases[1].Steps; !reflect.DeepEqual(got, []string{"backend"}) {
		t.Fatalf("build steps = %v", got)
	}
	if len(recipe.Phases[4].Unresolved) == 0 {
		t.Fatalf("launch facts should remain unresolved: %#v", recipe.Phases[4])
	}
}

func TestPipelinePlannerRejectsPreparedRecipeAfterConfigChange(t *testing.T) {
	repoRoot := t.TempDir()
	journal := filepath.Join(repoRoot, "phases.txt")
	plugin := filepath.Join(repoRoot, "plugin.py")
	writePhaseMatrixPlugin(t, plugin)
	configPath := filepath.Join(repoRoot, ".devctl.yaml")
	writePhaseMatrixConfig(t, configPath, plugin, journal, "warn")

	planner := PipelinePlanner{}
	recipe, err := planner.ResolveRecipe(t.Context(), "up", UpRequest{
		RepoRoot: repoRoot, Policy: PipelinePolicy{Timeout: 2 * time.Second},
	})
	if err != nil {
		t.Fatalf("resolve recipe: %v", err)
	}
	prepared, err := planner.PrepareReplacement(t.Context(), recipe)
	if err != nil {
		t.Fatalf("prepare replacement: %v", err)
	}
	if prepared.Build == nil || prepared.Prepare == nil {
		t.Fatalf("phase results were discarded: %#v", prepared)
	}
	writePhaseMatrixConfig(t, configPath, plugin, journal, "error")

	err = planner.ValidatePrepared(t.Context(), prepared)
	var operatorErr *OperatorError
	if !errors.As(err, &operatorErr) || operatorErr.Code != CodeRecipeStale {
		t.Fatalf("validate prepared error = %v, want %s", err, CodeRecipeStale)
	}
}

func writePhaseMatrixPlugin(t *testing.T, path string) {
	t.Helper()
	code := `import json, sys
journal = sys.argv[1]
def emit(value):
    print(json.dumps(value), flush=True)
emit({"type":"handshake","protocol_version":"v2","plugin_name":"phase-matrix","capabilities":{"ops":["config.mutate","build.run","prepare.run","validate.run","launch.plan"]}})
for line in sys.stdin:
    request = json.loads(line)
    op = request["op"]
    with open(journal, "a", encoding="utf-8") as handle:
        handle.write(op + "\n")
    output = {}
    if op == "config.mutate": output = {"config_patch":{"set":{},"unset":[]}}
    elif op in ("build.run", "prepare.run"): output = {"steps":[],"artifacts":{"binary":"/tmp/example"}}
    elif op == "validate.run": output = {"valid":True,"errors":[],"warnings":[]}
    elif op == "launch.plan": output = {"services":[{"name":"service","command":["/bin/true"]}]}
    emit({"type":"response","request_id":request["request_id"],"ok":True,"output":output})
`
	if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
}

func writePhaseMatrixConfig(t *testing.T, path, plugin, journal, strictness string) {
	t.Helper()
	config := "strictness: " + strictness + "\nplugins:\n  - id: phases\n    path: python3\n    args: [" + plugin + ", " + journal + "]\n"
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestPipelinePlannerPhaseMatrix(t *testing.T) {
	tests := []struct {
		name   string
		policy PipelinePolicy
		want   []string
	}{
		{
			name:   "default lifecycle",
			policy: PipelinePolicy{Timeout: 2 * time.Second},
			want:   []string{"config.mutate", "build.run", "prepare.run", "validate.run", "launch.plan"},
		},
		{
			name: "skipped optional phases",
			policy: PipelinePolicy{Timeout: 2 * time.Second,
				SkipBuild: true, SkipPrepare: true, SkipValidate: true},
			want: []string{"config.mutate", "launch.plan"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			journal := filepath.Join(repoRoot, "phases.txt")
			plugin := filepath.Join(repoRoot, "plugin.py")
			code := `import json, sys
journal = sys.argv[1]
def emit(value):
    print(json.dumps(value), flush=True)
emit({"type":"handshake","protocol_version":"v2","plugin_name":"phase-matrix","capabilities":{"ops":["config.mutate","build.run","prepare.run","validate.run","launch.plan"]}})
for line in sys.stdin:
    request = json.loads(line)
    op = request["op"]
    with open(journal, "a", encoding="utf-8") as handle:
        handle.write(op + "\n")
    output = {}
    if op == "config.mutate": output = {"config_patch":{"set":{},"unset":[]}}
    elif op in ("build.run", "prepare.run"): output = {"steps":[],"artifacts":{}}
    elif op == "validate.run": output = {"valid":True,"errors":[],"warnings":[]}
    elif op == "launch.plan": output = {"services":[{"name":"service","command":["/bin/true"]}]}
    emit({"type":"response","request_id":request["request_id"],"ok":True,"output":output})
`
			if err := os.WriteFile(plugin, []byte(code), 0o600); err != nil {
				t.Fatalf("write plugin: %v", err)
			}
			config := "plugins:\n  - id: phases\n    path: python3\n    args: [" + plugin + ", " + journal + "]\n"
			if err := os.WriteFile(filepath.Join(repoRoot, ".devctl.yaml"), []byte(config), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			planner := PipelinePlanner{}
			recipe, err := planner.ResolveRecipe(t.Context(), "up", UpRequest{RepoRoot: repoRoot, Policy: test.policy})
			if err != nil {
				t.Fatalf("resolve recipe: %v", err)
			}
			if _, err := planner.PrepareReplacement(t.Context(), recipe); err != nil {
				t.Fatalf("prepare replacement: %v", err)
			}
			data, err := os.ReadFile(journal)
			if err != nil {
				t.Fatalf("read journal: %v", err)
			}
			got := strings.Fields(string(data))
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("phase order = %v, want %v", got, test.want)
			}
		})
	}
}
