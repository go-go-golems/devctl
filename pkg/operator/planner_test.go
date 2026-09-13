package operator

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

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
			if _, err := (PipelinePlanner{}).Plan(t.Context(), UpRequest{RepoRoot: repoRoot, Policy: test.policy}); err != nil {
				t.Fatalf("plan: %v", err)
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
