package cmds

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestPhaseCommands_BuildPassesStepsAndPrintsJSON(t *testing.T) {
	repoRoot, cfgPath := phaseCommandFixture(t, "")

	stdout, _, err := runDevctlCommand(t,
		"build",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--timeout", (2 * time.Second).String(),
		"--step", "backend",
		"--step", "frontend",
	)
	require.NoError(t, err)

	out := decodePhaseOutput(t, stdout)
	require.Equal(t, "default", nestedString(t, out, "config", "phase", "marker"))
	steps := nestedSlice(t, out, "build", "steps")
	require.Len(t, steps, 2)
	require.Equal(t, "backend", steps[0].(map[string]any)["name"])
	require.Equal(t, "frontend", steps[1].(map[string]any)["name"])
	require.Equal(t, "default-artifact", nestedString(t, out, "build", "artifacts", "marker"))
}

func TestPhaseCommands_PreparePassesStepsAndPrintsJSON(t *testing.T) {
	repoRoot, cfgPath := phaseCommandFixture(t, "")

	stdout, _, err := runDevctlCommand(t,
		"prepare",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--timeout", (2 * time.Second).String(),
		"--step", "pnpm-install",
	)
	require.NoError(t, err)

	out := decodePhaseOutput(t, stdout)
	steps := nestedSlice(t, out, "prepare", "steps")
	require.Len(t, steps, 1)
	require.Equal(t, "pnpm-install", steps[0].(map[string]any)["name"])
	require.Equal(t, "default-artifact", nestedString(t, out, "prepare", "artifacts", "marker"))
}

func TestPhaseCommands_ValidateSuccessAndFailure(t *testing.T) {
	repoRoot, cfgPath := phaseCommandFixture(t, "")

	stdout, _, err := runDevctlCommand(t,
		"validate",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--timeout", (2 * time.Second).String(),
	)
	require.NoError(t, err)
	out := decodePhaseOutput(t, stdout)
	require.Equal(t, true, nestedMap(t, out, "validate")["valid"])

	repoRoot, cfgPath = phaseCommandFixture(t, "invalid")
	stdout, _, err = runDevctlCommand(t,
		"validate",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--timeout", (2 * time.Second).String(),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation failed")
	out = decodePhaseOutput(t, stdout)
	require.Equal(t, false, nestedMap(t, out, "validate")["valid"])
	errs := nestedSlice(t, out, "validate", "errors")
	require.Len(t, errs, 1)
	require.Equal(t, "E_INVALID", errs[0].(map[string]any)["code"])
}

func TestPhaseCommands_RespectProfileSelection(t *testing.T) {
	repoRoot := t.TempDir()
	plugin := writePhasePlugin(t, repoRoot)
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	cfg := []byte("profiles:\n  selected:\n    plugins: [selected]\nplugins:\n  - id: default\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    env:\n      PHASE_MARKER: default\n    priority: 10\n  - id: selected\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    env:\n      PHASE_MARKER: selected\n    priority: 20\n")
	require.NoError(t, os.WriteFile(cfgPath, cfg, 0o644))

	stdout, _, err := runDevctlCommand(t,
		"build",
		"--repo-root", repoRoot,
		"--config", cfgPath,
		"--profile", "selected",
		"--timeout", (2 * time.Second).String(),
	)
	require.NoError(t, err)

	out := decodePhaseOutput(t, stdout)
	require.Equal(t, "selected", nestedString(t, out, "config", "phase", "marker"))
	steps := nestedSlice(t, out, "build", "steps")
	require.Len(t, steps, 1)
	require.Equal(t, "selected-build", steps[0].(map[string]any)["name"])
}

func phaseCommandFixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	repoRoot := t.TempDir()
	plugin := writePhasePlugin(t, repoRoot)
	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	cfg := "plugins:\n  - id: phase\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    env:\n      PHASE_MARKER: default\n"
	if mode == "invalid" {
		cfg += "      PHASE_INVALID: 'true'\n"
	}
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	return repoRoot, cfgPath
}

func writePhasePlugin(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "phase_plugin.py")
	code := `#!/usr/bin/env python3
import json
import os
import sys

marker = os.environ.get("PHASE_MARKER", "default")
invalid = os.environ.get("PHASE_INVALID", "").lower() == "true"

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "phase-plugin",
    "capabilities": {"ops": ["config.mutate", "build.run", "prepare.run", "validate.run"]},
})

for line in sys.stdin:
    if not line.strip():
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")
    inp = req.get("input", {}) or {}
    steps = inp.get("steps") or []
    if op == "config.mutate":
        emit({"type":"response","request_id":rid,"ok":True,"output":{"config_patch":{"set":{"phase.marker": marker},"unset":[]}}})
    elif op == "build.run":
        names = steps or [marker + "-build"]
        emit({"type":"response","request_id":rid,"ok":True,"output":{"steps":[{"name": name, "ok": True} for name in names],"artifacts":{"marker": marker + "-artifact"}}})
    elif op == "prepare.run":
        names = steps or [marker + "-prepare"]
        emit({"type":"response","request_id":rid,"ok":True,"output":{"steps":[{"name": name, "ok": True} for name in names],"artifacts":{"marker": marker + "-artifact"}}})
    elif op == "validate.run":
        if invalid:
            emit({"type":"response","request_id":rid,"ok":True,"output":{"valid":False,"errors":[{"code":"E_INVALID","message":"invalid fixture"}],"warnings":[]}})
        else:
            emit({"type":"response","request_id":rid,"ok":True,"output":{"valid":True,"errors":[],"warnings":[]}})
    else:
        emit({"type":"response","request_id":rid,"ok":False,"error":{"code":"E_UNSUPPORTED","message":"unsupported"}})
`
	require.NoError(t, os.WriteFile(path, []byte(code), 0o755))
	return path
}

func runDevctlCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := &cobra.Command{Use: "devctl", SilenceUsage: true, SilenceErrors: true}
	require.NoError(t, AddCommands(root))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func decodePhaseOutput(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out), stdout)
	return out
}

func nestedMap(t *testing.T, v map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := v
	for _, key := range keys {
		next, ok := cur[key].(map[string]any)
		require.Truef(t, ok, "missing map key %s in %#v", key, cur[key])
		cur = next
	}
	return cur
}

func nestedSlice(t *testing.T, v map[string]any, keys ...string) []any {
	t.Helper()
	parent := nestedMap(t, v, keys[:len(keys)-1]...)
	out, ok := parent[keys[len(keys)-1]].([]any)
	require.Truef(t, ok, "missing slice key %s in %#v", keys[len(keys)-1], parent[keys[len(keys)-1]])
	return out
}

func nestedString(t *testing.T, v map[string]any, keys ...string) string {
	t.Helper()
	parent := nestedMap(t, v, keys[:len(keys)-1]...)
	out, ok := parent[keys[len(keys)-1]].(string)
	require.Truef(t, ok, "missing string key %s in %#v", keys[len(keys)-1], parent[keys[len(keys)-1]])
	return out
}
