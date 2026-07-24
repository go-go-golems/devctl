package cmds

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/internal/testrepo"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/stretchr/testify/require"
)

const wrapServiceHelperEnv = "DEVCTL_WRAP_SERVICE_TEST_HELPER"
const wrapServiceRunID = "01890f47-6c4e-7c5a-bb6d-6f2e9f4c0002"

func TestWrapServiceForwardsSIGHUPWithoutKillingWrapper(t *testing.T) {
	if os.Getenv(wrapServiceHelperEnv) == "1" {
		runWrapServiceTestHelper()
		return
	}

	repo := testrepo.New(t)
	store, err := runstate.NewStore(repo.Root)
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(context.Background(), runstate.RunRecord{
		RunID:   wrapServiceRunID,
		Service: "signal-fixture",
		Phase:   runstate.RunPlanned,
		Spec: runstate.ServiceSpecRecord{
			Name:    "signal-fixture",
			Command: []string{"sleep", "30"},
		},
	}))
	request, err := supervise.NewWrapperRequest(
		store,
		wrapServiceRunID,
		"signal-fixture",
		repo.Root,
		[]string{"sleep", "30"},
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, supervise.WriteWrapperRequest(request))

	cmd := exec.Command(os.Args[0], "-test.run=^TestWrapServiceForwardsSIGHUPWithoutKillingWrapper$")
	cmd.Env = append(os.Environ(),
		wrapServiceHelperEnv+"=1",
		"DEVCTL_WRAP_SERVICE_TEST_DIR="+repo.Root,
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
		if _, err := os.Stat(request.ReadyPath()); err == nil {
			break
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "wrapper did not report child start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoFileExists(t, request.RequestPath())

	wrapperIdentity, err := runstate.ReadProcessIdentity(cmd.Process.Pid)
	require.NoError(t, err)
	owner, err := supervise.ReadOwnerRecord(request.OwnerPath())
	require.NoError(t, err)
	require.NoError(t, supervise.ValidateOwnerRecord(context.Background(), request, wrapperIdentity, owner))
	ready, err := supervise.ReadReadyRecord(request.ReadyPath())
	require.NoError(t, err)
	require.NoError(t, supervise.ValidateReadyRecord(context.Background(), request, owner, ready))
	tamperedReady := *ready
	tamperedReady.RunID = "01890f47-6c4e-7c5a-bb6d-6f2e9f4c0099"
	require.Error(t, supervise.ValidateReadyRecord(context.Background(), request, owner, &tamperedReady))
	tamperedReady = *ready
	tamperedReady.ChildPGID++
	require.Error(t, supervise.ValidateReadyRecord(context.Background(), request, owner, &tamperedReady))
	tamperedReady = *ready
	tamperedReady.Child.StartToken += "-wrong"
	require.Error(t, supervise.ValidateReadyRecord(context.Background(), request, owner, &tamperedReady))

	require.NoError(t, cmd.Process.Signal(syscall.SIGHUP))
	waitErr := cmd.Wait()
	require.Error(t, waitErr)

	var waitStatus syscall.WaitStatus
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		waitStatus, _ = exitErr.Sys().(syscall.WaitStatus)
	}
	require.False(t, waitStatus.Signaled(), "wrapper itself must not die from the forwarded signal")

	exitInfo, err := state.ReadExitInfo(request.ExitPath())
	require.NoError(t, err)
	require.Equal(t, "hangup", exitInfo.Signal)
	require.False(t, state.ProcessAlive(ready.Child.PID))
}

func runWrapServiceTestHelper() {
	outputDir := os.Getenv("DEVCTL_WRAP_SERVICE_TEST_DIR")
	store, err := runstate.NewStore(outputDir)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	runDir, err := store.RunDir(wrapServiceRunID)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	command := newWrapServiceCmd()
	command.SetArgs([]string{
		"--request", filepath.Join(runDir, supervise.WrapperRequestName),
	})
	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
