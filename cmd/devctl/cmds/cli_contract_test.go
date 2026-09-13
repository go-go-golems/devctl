package cmds

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/stretchr/testify/require"
)

func TestBuiltCLIContracts(t *testing.T) {
	binary := buildDevctlForCLIContractTest(t)

	t.Run("structured no-state status", func(t *testing.T) {
		repoRoot := t.TempDir()
		stdout, stderr, err := runCLI(binary,
			"status", "--repo-root", repoRoot, "--with-glaze-output", "--format", "json",
		)
		require.NoError(t, err, stderr)
		require.Empty(t, stderr)
		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
		require.Len(t, rows, 1)
		require.Equal(t, "stopped", rows[0]["environment"])
	})

	t.Run("human output is the default status mode", func(t *testing.T) {
		repoRoot := t.TempDir()
		stdout, stderr, err := runCLI(binary, "status", "--repo-root", repoRoot)
		require.NoError(t, err, stderr)
		require.Empty(t, stderr)
		require.Contains(t, stdout, "Environment stopped")
		require.NotContains(t, stdout, "environment\"")
	})

	t.Run("unknown status and log selectors are usage errors", func(t *testing.T) {
		for _, args := range [][]string{
			{"status", "--repo-root", t.TempDir(), "missing"},
			{"logs", "--repo-root", t.TempDir(), "missing"},
		} {
			_, stderr, err := runCLI(binary, args...)
			require.Error(t, err)
			require.Equal(t, 2, processExitCode(t, err), stderr)
			require.Contains(t, stderr, "E_SERVICE_UNKNOWN:")
			require.Contains(t, stderr, "missing")
		}
	})

	t.Run("removed lifecycle spelling is unknown", func(t *testing.T) {
		repoRoot := t.TempDir()
		_, stderr, err := runCLI(binary, "stop-service", "--repo-root", repoRoot, "web")
		require.Error(t, err)
		require.Equal(t, 2, processExitCode(t, err))
		require.Equal(t, 1, strings.Count(stderr, `unknown command "stop-service"`))
		require.Contains(t, stderr, "E_USAGE:")
		require.Contains(t, stderr, `unknown command "stop-service"`)
	})

	t.Run("logs flag conflicts are usage errors rendered once", func(t *testing.T) {
		repoRoot := t.TempDir()
		_, stderr, err := runCLI(binary,
			"logs", "--repo-root", repoRoot, "--follow", "--until", "1m",
		)
		require.Error(t, err)
		require.Equal(t, 2, processExitCode(t, err))
		require.Equal(t, 1, strings.Count(stderr, "--follow and --until"))
		require.Contains(t, stderr, "E_USAGE:")
	})

	t.Run("help and typo never start plugin", func(t *testing.T) {
		repoRoot := t.TempDir()
		configPath, markerPath := writeCountingCommandPlugin(t, repoRoot)
		_, stderr, err := runCLIInDir(binary, repoRoot, "--help")
		require.NoError(t, err, stderr)
		require.NoFileExists(t, markerPath)

		_, stderr, err = runCLIInDir(binary, repoRoot, "completion", "bash")
		require.NoError(t, err, stderr)
		require.NoFileExists(t, markerPath)

		_, stderr, err = runCLI(binary,
			"typo-command", "--repo-root", repoRoot, "--config", configPath,
		)
		require.Error(t, err)
		require.Equal(t, 2, processExitCode(t, err))
		require.Contains(t, stderr, "plugins refresh")
		require.NoFileExists(t, markerPath)
	})

	t.Run("raw help and command schema are machine readable", func(t *testing.T) {
		stdout, stderr, err := runCLI(binary,
			"help", "export", "--slug", "plugin-authoring", "--format", "json", "--output-fields", "content",
		)
		require.NoError(t, err, stderr)
		require.Contains(t, stdout, "devctl plugins let you take")

		stdout, stderr, err = runCLI(binary, "schema", "restart")
		require.NoError(t, err, stderr)
		var schema map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &schema))
		require.Equal(t, "devctl restart", schema["path"])
		require.Contains(t, schema["use"], "restart")
		require.NotEmpty(t, schema["flags"])

		stdout, stderr, err = runCLI(binary, "help", "artifact-provenance")
		require.NoError(t, err, stderr)
		require.Contains(t, stdout, "content-addressed")
		require.Contains(t, stdout, "Garbage collection")
	})

	t.Run("lifecycle explain resolves recipe without starting provider", func(t *testing.T) {
		repoRoot := t.TempDir()
		configPath, markerPath := writeCountingCommandPlugin(t, repoRoot)
		stdout, stderr, err := runCLI(binary,
			"restart", "api", "--explain", "--repo-root", repoRoot, "--config", configPath, "--format", "json",
		)
		require.NoError(t, err, stderr)
		require.NoFileExists(t, markerPath)
		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
		require.Len(t, rows, 5)
		require.Equal(t, "config.mutate", rows[0]["phase"])
		require.Equal(t, "launch.plan", rows[4]["phase"])
		require.NotEmpty(t, rows[4]["unresolved"])
	})

	t.Run("catalog inspection does not start provider", func(t *testing.T) {
		repoRoot := t.TempDir()
		configPath, markerPath := writeCountingCommandPlugin(t, repoRoot)
		stdout, stderr, err := runCLI(binary,
			"plugins", "catalog", "--repo-root", repoRoot, "--config", configPath,
			"--format", "json",
		)
		require.NoError(t, err, stderr)
		require.NoFileExists(t, markerPath)
		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
		require.Len(t, rows, 1)
		require.Equal(t, "missing", rows[0]["catalog_state"])
		require.Equal(t, "handshake", rows[0]["source"])
	})

	t.Run("catalog command starts exactly one provider", func(t *testing.T) {
		repoRoot := t.TempDir()
		configPath, markerPath := writeCountingCommandPlugin(t, repoRoot)
		_, stderr, err := runCLI(binary,
			"plugins", "refresh", "--repo-root", repoRoot, "--config", configPath,
			"--format", "json",
		)
		require.NoError(t, err, stderr)
		require.NoError(t, os.Remove(markerPath))

		stdout, stderr, err := runCLIInDir(binary, repoRoot, "--help")
		require.NoError(t, err, stderr)
		require.Contains(t, stdout, "echo")
		require.NoFileExists(t, markerPath)

		stdout, stderr, err = runCLIInDir(binary, repoRoot, "completion", "bash")
		require.NoError(t, err, stderr)
		require.Contains(t, stdout, "echo")
		require.NoFileExists(t, markerPath)

		_, stderr, err = runCLI(binary,
			"echo", "--repo-root", repoRoot, "--config", configPath, "hello",
		)
		require.NoError(t, err, stderr)
		data, readErr := os.ReadFile(markerPath)
		require.NoError(t, readErr)
		lines := strings.Fields(string(data))
		require.Len(t, lines, 1, "dynamic invocation started provider more than once")
	})

	t.Run("build artifact is published and linked to service run", func(t *testing.T) {
		repoRoot := t.TempDir()
		configPath := writeArtifactServicePlugin(t, repoRoot)
		stdout, stderr, err := runCLI(binary,
			"up", "--repo-root", repoRoot, "--config", configPath, "--format", "json",
		)
		if err != nil {
			logs, _ := filepath.Glob(filepath.Join(repoRoot, ".devctl", "runs", "*", "*"))
			for _, logPath := range logs {
				data, _ := os.ReadFile(logPath)
				stderr += "\n" + logPath + ":\n" + string(data)
			}
		}
		require.NoError(t, err, stderr+stdout)
		t.Cleanup(func() {
			_, _, _ = runCLI(binary, "down", "--repo-root", repoRoot)
		})

		store, err := runstate.NewStore(repoRoot)
		require.NoError(t, err)
		environment, err := store.LoadEnvironment(t.Context())
		require.NoError(t, err)
		run, err := store.LoadRun(t.Context(), environment.Services["artifact-service"].CurrentRunID)
		require.NoError(t, err)
		require.NotNil(t, run.Artifact)
		require.Equal(t, "service-bin", run.Artifact.ID)
		require.Equal(t, run.Artifact.Path, run.Spec.Command[0])
		require.Contains(t, run.Artifact.Path, filepath.Join(".devctl", "artifacts", "sha256", run.Artifact.SHA256))
		require.NoError(t, runstate.ValidateArtifact(*run.Artifact))

		statusOut, statusErr, err := runCLI(binary,
			"status", "--repo-root", repoRoot, "--with-glaze-output", "--format", "json",
		)
		require.NoError(t, err, statusErr)
		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(statusOut), &rows))
		require.Len(t, rows, 1)
		require.Equal(t, run.Artifact.SHA256, rows[0]["artifact_sha256"])
	})

	t.Run("provider-qualified run resolves a catalog collision", func(t *testing.T) {
		repoRoot := t.TempDir()
		configPath, alphaMarker, betaMarker := writeConflictingCommandPlugins(t, repoRoot)
		_, stderr, err := runCLI(binary,
			"plugins", "refresh", "--repo-root", repoRoot, "--config", configPath,
			"--format", "json",
		)
		require.Error(t, err)
		require.Contains(t, stderr, "plugin command catalog has conflicts")
		require.NoError(t, os.Remove(alphaMarker))
		require.NoError(t, os.Remove(betaMarker))

		stdout, stderr, err := runCLI(binary,
			"plugins", "inspect", "alpha", "--repo-root", repoRoot,
			"--config", configPath, "--format", "json",
		)
		require.NoError(t, err, stderr)
		var rows []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
		require.Len(t, rows, 1)
		require.Equal(t, "alpha", rows[0]["id"])
		require.Equal(t, "echo", rows[0]["command"])
		require.Equal(t, true, rows[0]["conflict"])
		require.NoFileExists(t, alphaMarker)
		require.NoFileExists(t, betaMarker)

		_, stderr, err = runCLI(binary,
			"plugins", "run", "alpha", "echo", "--repo-root", repoRoot,
			"--config", configPath, "--", "hello",
		)
		require.NoError(t, err, stderr)
		require.FileExists(t, alphaMarker)
		require.NoFileExists(t, betaMarker)
	})
}

func processExitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	return exitErr.ExitCode()
}

func buildDevctlForCLIContractTest(t *testing.T) string {
	t.Helper()
	repositoryRoot := findDevctlRootForTest(t)
	binary := filepath.Join(t.TempDir(), "devctl")
	command := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./cmd/devctl")
	command.Dir = repositoryRoot
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return binary
}

func runCLI(binary string, args ...string) (string, string, error) {
	return runCLIInDir(binary, "", args...)
}

func runCLIInDir(binary string, directory string, args ...string) (string, string, error) {
	command := exec.Command(binary, args...)
	command.Dir = directory
	var stdout strings.Builder
	var stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func writeArtifactServicePlugin(t *testing.T, repoRoot string) string {
	t.Helper()
	pluginPath := filepath.Join(repoRoot, "artifact-plugin.py")
	artifactPath := filepath.Join(repoRoot, "build", "service.sh")
	plugin := `#!/usr/bin/env python3
import json
import os
import pathlib
import sys

artifact = pathlib.Path(sys.argv[1])
def emit(value):
    print(json.dumps(value), flush=True)

emit({"type":"handshake","protocol_version":"v2","plugin_name":"artifact","capabilities":{"ops":["config.mutate","build.run","prepare.run","validate.run","launch.plan"]}})
for line in sys.stdin:
    request = json.loads(line)
    op = request["op"]
    output = {}
    if op == "config.mutate":
        output = {"config_patch":{"set":{},"unset":[]}}
    elif op == "build.run":
        artifact.parent.mkdir(parents=True, exist_ok=True)
        artifact.write_text("#!/bin/sh\nexec sleep 60\n", encoding="utf-8")
        os.chmod(artifact, 0o700)
        output = {"steps":[{"name":"service","ok":True}],"artifacts":{"service-bin":str(artifact)}}
    elif op == "prepare.run":
        output = {"steps":[],"artifacts":{}}
    elif op == "validate.run":
        output = {"valid":True,"errors":[],"warnings":[]}
    elif op == "launch.plan":
        output = {"services":[{"name":"artifact-service","executable":{"artifact_id":"service-bin"}}]}
    emit({"type":"response","request_id":request["request_id"],"ok":True,"output":output})
`
	require.NoError(t, os.WriteFile(pluginPath, []byte(plugin), 0o700))
	configPath := filepath.Join(repoRoot, ".devctl.yaml")
	config := "plugins:\n  - id: artifact\n    path: python3\n    args: [" + pluginPath + ", " + artifactPath + "]\n"
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))
	return configPath
}

func writeCountingCommandPlugin(t *testing.T, repoRoot string) (string, string) {
	t.Helper()
	markerPath := filepath.Join(repoRoot, "starts.txt")
	pluginPath := filepath.Join(repoRoot, "plugin.py")
	plugin := `#!/usr/bin/env python3
import json
import sys

marker = sys.argv[1]
with open(marker, "a", encoding="utf-8") as handle:
    handle.write("start\n")

def emit(value):
    sys.stdout.write(json.dumps(value) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "counting-command",
    "capabilities": {
        "ops": ["command.run"],
        "commands": [{"name": "echo", "help": "Echo", "args_spec": []}],
    },
})

for line in sys.stdin:
    request = json.loads(line)
    if request.get("op") == "command.run":
        emit({"type": "response", "request_id": request["request_id"], "ok": True, "output": {"exit_code": 0}})
    else:
        emit({"type": "response", "request_id": request["request_id"], "ok": False, "error": {"code": "E_UNSUPPORTED", "message": "unsupported"}})
`
	require.NoError(t, os.WriteFile(pluginPath, []byte(plugin), 0o700))
	configPath := filepath.Join(repoRoot, ".devctl.yaml")
	configBody := "plugins:\n  - id: counting\n    path: " + pluginPath +
		"\n    args:\n      - " + markerPath + "\n"
	require.NoError(t, os.WriteFile(configPath, []byte(configBody), 0o600))
	return configPath, markerPath
}

func writeConflictingCommandPlugins(t *testing.T, repoRoot string) (string, string, string) {
	t.Helper()
	configPath, alphaMarker := writeCountingCommandPlugin(t, repoRoot)
	pluginPath := filepath.Join(repoRoot, "plugin.py")
	betaMarker := filepath.Join(repoRoot, "beta-starts.txt")
	configBody := "plugins:\n" +
		"  - id: alpha\n    path: " + pluginPath + "\n    args: [" + alphaMarker + "]\n" +
		"  - id: beta\n    path: " + pluginPath + "\n    args: [" + betaMarker + "]\n"
	require.NoError(t, os.WriteFile(configPath, []byte(configBody), 0o600))
	return configPath, alphaMarker, betaMarker
}
