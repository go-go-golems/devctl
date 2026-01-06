package state

import (
	"encoding/json"
	stderrors "errors"
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
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if stderrors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}
