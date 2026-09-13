package runtime

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSupportedPythonRunner(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	command := exec.Command("python3", "-m", "unittest", "-v", "test_devctl_runner.py")
	command.Dir = filepath.Join(repoRoot, "sdk", "python")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("python runner tests: %v\n%s", err, output)
	}
}
