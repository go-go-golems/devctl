//go:build linux

package runstate

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
)

const bootIDPath = "/proc/sys/kernel/random/boot_id"

func ReadProcessIdentity(pid int) (*ProcessIdentity, error) {
	if pid <= 0 {
		return nil, errors.Wrapf(ErrInvalidState, "invalid process PID %d", pid)
	}
	bootID, err := os.ReadFile(bootIDPath)
	if err != nil {
		return nil, errors.Wrap(err, "read Linux boot ID")
	}
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	stat, err := os.ReadFile(statPath)
	if err != nil {
		return nil, errors.Wrap(err, "read Linux process stat")
	}
	startTime, err := parseProcStatStartTime(stat)
	if err != nil {
		return nil, errors.Wrapf(err, "parse Linux process stat for PID %d", pid)
	}
	return &ProcessIdentity{
		PID:        pid,
		StartToken: strings.TrimSpace(string(bootID)) + ":" + startTime,
	}, nil
}

func MatchesProcess(identity *ProcessIdentity) (bool, error) {
	status, err := InspectProcess(identity)
	if err != nil {
		return false, err
	}
	return status == ProcessMatches, nil
}

func InspectProcess(identity *ProcessIdentity) (ProcessStatus, error) {
	if identity == nil || identity.PID <= 0 || identity.StartToken == "" {
		return ProcessAbsent, errors.Wrap(ErrInvalidState, "incomplete process identity")
	}
	current, err := ReadProcessIdentity(identity.PID)
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			return ProcessAbsent, nil
		}
		return ProcessAbsent, err
	}
	if current.StartToken != identity.StartToken {
		return ProcessMismatch, nil
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", identity.PID))
	if err != nil {
		if os.IsNotExist(err) {
			return ProcessAbsent, nil
		}
		return ProcessAbsent, errors.Wrap(err, "read Linux process state")
	}
	state, err := parseProcStatState(stat)
	if err != nil {
		return ProcessAbsent, err
	}
	if state == 'Z' {
		return ProcessAbsent, nil
	}
	return ProcessMatches, nil
}

func parseProcStatStartTime(stat []byte) (string, error) {
	// /proc/<pid>/stat field 2 is parenthesized comm and may itself contain
	// spaces or ')'. Locate its final ')' before indexing fields 3 through 22.
	closeIndex := bytes.LastIndexByte(stat, ')')
	if closeIndex < 0 {
		return "", errors.New("process stat is missing command terminator")
	}
	fields := bytes.Fields(stat[closeIndex+1:])
	const startTimeIndexAfterCommand = 19 // field 22 minus first remaining field 3
	if len(fields) <= startTimeIndexAfterCommand {
		return "", errors.Errorf("process stat has %d trailing fields, need at least %d", len(fields), startTimeIndexAfterCommand+1)
	}
	startTime := string(fields[startTimeIndexAfterCommand])
	if startTime == "" {
		return "", errors.New("process stat start time is empty")
	}
	return startTime, nil
}

func parseProcStatState(stat []byte) (byte, error) {
	closeIndex := bytes.LastIndexByte(stat, ')')
	if closeIndex < 0 {
		return 0, errors.New("process stat is missing command terminator")
	}
	fields := bytes.Fields(stat[closeIndex+1:])
	if len(fields) == 0 || len(fields[0]) != 1 {
		return 0, errors.New("process stat state is missing")
	}
	return fields[0][0], nil
}
