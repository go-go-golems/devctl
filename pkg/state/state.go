package state

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

const (
	StateDirName  = ".devctl"
	StateFilename = "state.json"
	LogsDirName   = "logs"
)

type State struct {
	RepoRoot  string          `json:"repo_root"`
	CreatedAt time.Time       `json:"created_at"`
	Services  []ServiceRecord `json:"services"`
}

type ServiceRecord struct {
	Name      string            `json:"name"`
	PID       int               `json:"pid"`
	Command   []string          `json:"command"`
	Cwd       string            `json:"cwd"`
	Env       map[string]string `json:"env,omitempty"`
	StdoutLog string            `json:"stdout_log"`
	StderrLog string            `json:"stderr_log"`
	ExitInfo  string            `json:"exit_info,omitempty"`
	StartedAt time.Time         `json:"started_at,omitempty"` // When the process was started

	// Health check configuration (if any)
	HealthType    string `json:"health_type,omitempty"`    // "tcp"|"http"
	HealthAddress string `json:"health_address,omitempty"` // For TCP checks
	HealthURL     string `json:"health_url,omitempty"`     // For HTTP checks
}

func StatePath(repoRoot string) string {
	return filepath.Join(repoRoot, StateDirName, StateFilename)
}

func LogsDir(repoRoot string) string {
	return filepath.Join(repoRoot, StateDirName, LogsDirName)
}

func Load(repoRoot string) (*State, error) {
	path := StatePath(repoRoot)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read state")
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, errors.Wrap(err, "parse state json")
	}
	return &s, nil
}

func Save(repoRoot string, s *State) error {
	if s == nil {
		return errors.New("nil state")
	}
	dir := filepath.Dir(StatePath(repoRoot))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errors.Wrap(err, "mkdir state dir")
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal state")
	}
	if err := os.WriteFile(StatePath(repoRoot), b, 0o644); err != nil {
		return errors.Wrap(err, "write state")
	}
	return nil
}

func Remove(repoRoot string) error {
	path := StatePath(repoRoot)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "remove state")
	}
	return nil
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if isZombie(pid) {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if stderrors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}

func isZombie(pid int) bool {
	path := fmt.Sprintf("/proc/%d/stat", pid)
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Format: pid (comm) state ...
	// We want the state character after the closing ')'.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return false
	}
	rest := bytes.TrimSpace(b[i+1:])
	fields := bytes.Fields(rest)
	if len(fields) < 1 || len(fields[0]) < 1 {
		return false
	}
	return fields[0][0] == 'Z'
}
