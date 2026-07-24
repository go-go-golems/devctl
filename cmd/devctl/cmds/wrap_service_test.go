package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/stretchr/testify/require"
)

const wrapServiceHelperEnv = "DEVCTL_WRAP_SERVICE_TEST_HELPER"

func TestWrapServiceForwardsSIGHUPWithoutKillingWrapper(t *testing.T) {
	if os.Getenv(wrapServiceHelperEnv) == "1" {
		runWrapServiceTestHelper()
		return
	}

	outputDir := t.TempDir()
	readyPath := filepath.Join(outputDir, "service.ready")
	exitInfoPath := filepath.Join(outputDir, "service.exit.json")

	cmd := exec.Command(os.Args[0], "-test.run=^TestWrapServiceForwardsSIGHUPWithoutKillingWrapper$")
	cmd.Env = append(os.Environ(),
		wrapServiceHelperEnv+"=1",
		"DEVCTL_WRAP_SERVICE_TEST_DIR="+outputDir,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "wrapper did not report child start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.NoError(t, cmd.Process.Signal(syscall.SIGHUP))
	waitErr := cmd.Wait()
	require.Error(t, waitErr)

	var waitStatus syscall.WaitStatus
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		waitStatus, _ = exitErr.Sys().(syscall.WaitStatus)
	}
	require.False(t, waitStatus.Signaled(), "wrapper itself must not die from the forwarded signal")

	exitInfo, err := state.ReadExitInfo(exitInfoPath)
	require.NoError(t, err)
	require.Equal(t, "hangup", exitInfo.Signal)

	childPIDBytes, err := os.ReadFile(readyPath)
	require.NoError(t, err)
	childPID, err := strconv.Atoi(string(bytesTrimSpace(childPIDBytes)))
	require.NoError(t, err)
	require.False(t, state.ProcessAlive(childPID))
}

func runWrapServiceTestHelper() {
	outputDir := os.Getenv("DEVCTL_WRAP_SERVICE_TEST_DIR")
	command := newWrapServiceCmd()
	command.SetArgs([]string{
		"--service", "signal-fixture",
		"--cwd", outputDir,
		"--stdout-log", filepath.Join(outputDir, "service.stdout.log"),
		"--stderr-log", filepath.Join(outputDir, "service.stderr.log"),
		"--exit-info", filepath.Join(outputDir, "service.exit.json"),
		"--ready-file", filepath.Join(outputDir, "service.ready"),
		"--",
		"sleep", "30",
	})
	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func bytesTrimSpace(value []byte) string {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\n' || value[start] == '\t' || value[start] == '\r') {
		start++
	}
	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\t' || value[end-1] == '\r') {
		end--
	}
	return string(value[start:end])
}
