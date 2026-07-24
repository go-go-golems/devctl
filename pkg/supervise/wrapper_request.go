package supervise

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/pkg/errors"
)

const (
	WrapperRequestVersion = 1
	HandshakeVersion      = 1
	WrapperRequestName    = "request.json"
	OwnerRecordName       = "owner.json"
	ReadyRecordName       = "ready.json"
	StdoutLogName         = "stdout.log"
	StderrLogName         = "stderr.log"
	ExitRecordName        = "exit.json"
)

var (
	ErrOwnerRecordMissing = stderrors.New("wrapper owner record missing")
	ErrReadyRecordMissing = stderrors.New("wrapper ready record missing")
)

type WrapperRequest struct {
	Version     int               `json:"version"`
	RunID       string            `json:"run_id"`
	Service     string            `json:"service"`
	RepoRoot    string            `json:"repo_root"`
	Cwd         string            `json:"cwd"`
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment,omitempty"`
	ArtifactDir string            `json:"artifact_dir"`
	TailLines   int               `json:"tail_lines"`
}

type OwnerRecord struct {
	Version   int                      `json:"version"`
	RunID     string                   `json:"run_id"`
	Service   string                   `json:"service"`
	Wrapper   runstate.ProcessIdentity `json:"wrapper"`
	WrittenAt time.Time                `json:"written_at"`
}

type ReadyRecord struct {
	Version   int                      `json:"version"`
	RunID     string                   `json:"run_id"`
	Service   string                   `json:"service"`
	Wrapper   runstate.ProcessIdentity `json:"wrapper"`
	Child     runstate.ProcessIdentity `json:"child"`
	ChildPGID int                      `json:"child_pgid"`
	WrittenAt time.Time                `json:"written_at"`
}

func NewWrapperRequest(
	store *runstate.Store,
	runID string,
	service string,
	cwd string,
	command []string,
	environment map[string]string,
) (*WrapperRequest, error) {
	if store == nil {
		return nil, errors.New("create wrapper request: nil runstate store")
	}
	artifactDir, err := store.RunDir(runID)
	if err != nil {
		return nil, err
	}
	request := &WrapperRequest{
		Version:     WrapperRequestVersion,
		RunID:       runID,
		Service:     service,
		RepoRoot:    store.RepoRoot(),
		Cwd:         cwd,
		Command:     append([]string{}, command...),
		Environment: cloneStringMap(environment),
		ArtifactDir: artifactDir,
		TailLines:   25,
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return request, nil
}

func (r *WrapperRequest) Validate() error {
	if r == nil {
		return errors.New("validate wrapper request: nil request")
	}
	if r.Version != WrapperRequestVersion {
		return errors.Errorf("unsupported wrapper request version %d", r.Version)
	}
	store, err := runstate.NewStore(r.RepoRoot)
	if err != nil {
		return err
	}
	expectedArtifactDir, err := store.RunDir(r.RunID)
	if err != nil {
		return err
	}
	if filepath.Clean(r.ArtifactDir) != expectedArtifactDir {
		return errors.Errorf("wrapper artifact directory %q does not match run %q", r.ArtifactDir, r.RunID)
	}
	if r.Service == "" || strings.ContainsAny(r.Service, `/\`) || r.Service == "." || r.Service == ".." {
		return errors.Errorf("invalid wrapper service %q", r.Service)
	}
	if r.Cwd == "" || !filepath.IsAbs(r.Cwd) {
		return errors.Errorf("wrapper cwd must be absolute: %q", r.Cwd)
	}
	if len(r.Command) == 0 || r.Command[0] == "" {
		return errors.New("wrapper command is empty")
	}
	if r.TailLines <= 0 {
		r.TailLines = 25
	}
	return nil
}

func (r *WrapperRequest) RequestPath() string {
	return filepath.Join(r.ArtifactDir, WrapperRequestName)
}

func (r *WrapperRequest) OwnerPath() string {
	return filepath.Join(r.ArtifactDir, OwnerRecordName)
}

func (r *WrapperRequest) ReadyPath() string {
	return filepath.Join(r.ArtifactDir, ReadyRecordName)
}

func (r *WrapperRequest) StdoutPath() string {
	return filepath.Join(r.ArtifactDir, StdoutLogName)
}

func (r *WrapperRequest) StderrPath() string {
	return filepath.Join(r.ArtifactDir, StderrLogName)
}

func (r *WrapperRequest) ExitPath() string {
	return filepath.Join(r.ArtifactDir, ExitRecordName)
}

func WriteWrapperRequest(request *WrapperRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := runstate.WriteJSONAtomic(request.RequestPath(), request, 0o600); err != nil {
		return errors.Wrap(err, "write wrapper request")
	}
	return nil
}

func LoadWrapperRequest(path string) (*WrapperRequest, error) {
	if path == "" {
		return nil, errors.New("load wrapper request: empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read wrapper request")
	}
	var request WrapperRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, errors.Wrap(err, "parse wrapper request")
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if filepath.Clean(path) != request.RequestPath() {
		return nil, errors.Errorf("wrapper request path %q does not match %q", path, request.RequestPath())
	}
	return &request, nil
}

func WriteOwnerRecord(path string, record OwnerRecord) error {
	if err := runstate.WriteJSONAtomic(path, record, 0o600); err != nil {
		return errors.Wrap(err, "write wrapper owner record")
	}
	return nil
}

func WriteReadyRecord(path string, record ReadyRecord) error {
	if err := runstate.WriteJSONAtomic(path, record, 0o600); err != nil {
		return errors.Wrap(err, "write wrapper ready record")
	}
	return nil
}

func ReadOwnerRecord(path string) (*OwnerRecord, error) {
	var record OwnerRecord
	if err := readHandshakeJSON(path, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func ReadReadyRecord(path string) (*ReadyRecord, error) {
	var record ReadyRecord
	if err := readHandshakeJSON(path, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func ValidateOwnerRecord(
	ctx context.Context,
	request *WrapperRequest,
	expectedWrapper *runstate.ProcessIdentity,
	record *OwnerRecord,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if request == nil || expectedWrapper == nil || record == nil {
		return errors.New("validate owner record: missing input")
	}
	if record.Version != HandshakeVersion {
		return errors.Errorf("owner record has unsupported version %d", record.Version)
	}
	if record.RunID != request.RunID || record.Service != request.Service {
		return errors.New("owner record run or service does not match request")
	}
	if record.Wrapper != *expectedWrapper {
		return errors.New("owner record wrapper identity does not match launched wrapper")
	}
	matches, err := runstate.MatchesProcess(&record.Wrapper)
	if err != nil {
		return errors.Wrap(err, "validate owner process identity")
	}
	if !matches {
		return errors.New("owner record wrapper process identity is not live")
	}
	return nil
}

func ValidateReadyRecord(
	ctx context.Context,
	request *WrapperRequest,
	owner *OwnerRecord,
	record *ReadyRecord,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if request == nil || owner == nil || record == nil {
		return errors.New("validate ready record: missing input")
	}
	if record.Version != HandshakeVersion {
		return errors.Errorf("ready record has unsupported version %d", record.Version)
	}
	if record.RunID != request.RunID || record.Service != request.Service {
		return errors.New("ready record run or service does not match request")
	}
	if record.Wrapper != owner.Wrapper {
		return errors.New("ready record wrapper identity does not match owner record")
	}
	if record.Child.PID <= 0 || record.Child.StartToken == "" {
		return errors.New("ready record child identity is incomplete")
	}
	if record.ChildPGID != record.Child.PID {
		return errors.Errorf("ready record child PGID %d does not match isolated child PID %d", record.ChildPGID, record.Child.PID)
	}
	if err := validateChildProcessGroup(record.Child.PID, record.ChildPGID); err != nil {
		return err
	}
	matches, err := runstate.MatchesProcess(&record.Child)
	if err != nil {
		return errors.Wrap(err, "validate ready child process identity")
	}
	if !matches {
		return errors.New("ready record child process identity is not live")
	}
	return nil
}

func readHandshakeJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "read wrapper handshake record")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return errors.Wrap(err, "parse wrapper handshake record")
	}
	return nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
