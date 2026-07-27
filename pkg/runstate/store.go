package runstate

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	stateDirName  = ".devctl"
	stateFileName = "state.json"
	runsDirName   = "runs"
	runFileName   = "run.json"
)

var (
	ErrRevisionConflict = stderrors.New("runstate revision conflict")
	ErrInvalidPath      = stderrors.New("invalid runstate path")
	ErrInvalidState     = stderrors.New("invalid runstate document")
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Store struct {
	repoRoot string
	now      func() time.Time
	mu       sync.Mutex
	hooks    atomicWriteHooks
}

type StoreOption func(*Store)

func WithClock(now func() time.Time) StoreOption {
	return func(store *Store) {
		if now != nil {
			store.now = now
		}
	}
}

func NewStore(repoRoot string, options ...StoreOption) (*Store, error) {
	if repoRoot == "" {
		return nil, errors.Wrap(ErrInvalidPath, "repository root is empty")
	}
	absolute, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, errors.Wrap(err, "resolve repository root")
	}
	absolute = filepath.Clean(absolute)

	store := &Store{
		repoRoot: absolute,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		option(store)
	}
	return store, nil
}

func (s *Store) RepoRoot() string {
	return s.repoRoot
}

func (s *Store) StatePath() string {
	return filepath.Join(s.repoRoot, stateDirName, stateFileName)
}

func (s *Store) RunsDir() string {
	return filepath.Join(s.repoRoot, stateDirName, runsDirName)
}

func (s *Store) RunDir(runID string) (string, error) {
	if err := validateRunID(runID); err != nil {
		return "", err
	}
	return safeJoin(s.RunsDir(), runID)
}

func (s *Store) RunPath(runID string) (string, error) {
	dir, err := s.RunDir(runID)
	if err != nil {
		return "", err
	}
	return safeJoin(dir, runFileName)
}

func (s *Store) EnsureLayout() error {
	if err := os.MkdirAll(s.RunsDir(), 0o700); err != nil {
		return errors.Wrap(err, "create runstate layout")
	}
	return nil
}

func (s *Store) CreateEnvironment(ctx context.Context, state EnvironmentState) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "create environment state")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.StatePath()); err == nil {
		return errors.Wrap(os.ErrExist, "create environment state")
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "inspect environment state")
	}

	now := s.now()
	if state.Version == 0 {
		state.Version = StateSchemaVersion
	}
	if state.RepoRoot == "" {
		state.RepoRoot = s.repoRoot
	}
	if state.Revision == 0 {
		state.Revision = 1
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = state.CreatedAt
	}
	if state.Services == nil {
		state.Services = map[string]ServiceSlot{}
	}
	if err := s.validateEnvironment(&state); err != nil {
		return err
	}
	return writeJSONAtomic(s.StatePath(), &state, 0o600, s.hooks)
}

func (s *Store) LoadEnvironment(ctx context.Context) (*EnvironmentState, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "load environment state")
	}
	var state EnvironmentState
	if err := readJSON(s.StatePath(), &state); err != nil {
		return nil, err
	}
	if err := s.validateEnvironment(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) Update(
	ctx context.Context,
	expectedRevision uint64,
	fn func(*EnvironmentState) error,
) error {
	if fn == nil {
		return errors.New("update environment state: nil mutation")
	}
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "update environment state")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var current EnvironmentState
	if err := readJSON(s.StatePath(), &current); err != nil {
		return err
	}
	if err := s.validateEnvironment(&current); err != nil {
		return err
	}
	if current.Revision != expectedRevision {
		return errors.Wrapf(
			ErrRevisionConflict,
			"expected revision %d, found %d",
			expectedRevision,
			current.Revision,
		)
	}

	next := cloneEnvironment(current)
	if err := fn(&next); err != nil {
		return errors.Wrap(err, "mutate environment state")
	}
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "update environment state")
	}
	next.Revision++
	next.UpdatedAt = s.now()
	if err := s.validateEnvironment(&next); err != nil {
		return err
	}
	return writeJSONAtomic(s.StatePath(), &next, 0o600, s.hooks)
}

func (s *Store) CreateRun(ctx context.Context, run RunRecord) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "create run record")
	}
	if run.Version == 0 {
		run.Version = RunSchemaVersion
	}
	now := s.now()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	dir, err := s.RunDir(run.RunID)
	if err != nil {
		return err
	}
	if run.ArtifactDir == "" {
		run.ArtifactDir = filepath.ToSlash(filepath.Join(stateDirName, runsDirName, run.RunID))
	}
	if err := s.validateRun(&run); err != nil {
		return err
	}
	if err := os.MkdirAll(s.RunsDir(), 0o700); err != nil {
		return errors.Wrap(err, "create runs directory")
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return errors.Wrap(err, "create run directory")
	}
	path, err := s.RunPath(run.RunID)
	if err != nil {
		return err
	}
	return writeJSONAtomic(path, &run, 0o600, s.hooks)
}

func (s *Store) LoadRun(ctx context.Context, runID string) (*RunRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "load run record")
	}
	path, err := s.RunPath(runID)
	if err != nil {
		return nil, err
	}
	var run RunRecord
	if err := readJSON(path, &run); err != nil {
		return nil, err
	}
	if err := s.validateRun(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) UpdateRun(ctx context.Context, runID string, fn func(*RunRecord) error) error {
	if fn == nil {
		return errors.New("update run record: nil mutation")
	}
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "update run record")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.RunPath(runID)
	if err != nil {
		return err
	}
	var run RunRecord
	if err := readJSON(path, &run); err != nil {
		return err
	}
	if err := s.validateRun(&run); err != nil {
		return err
	}
	if err := fn(&run); err != nil {
		return errors.Wrap(err, "mutate run record")
	}
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "update run record")
	}
	run.UpdatedAt = s.now()
	if err := s.validateRun(&run); err != nil {
		return err
	}
	return writeJSONAtomic(path, &run, 0o600, s.hooks)
}

func (s *Store) validateEnvironment(state *EnvironmentState) error {
	if state.Version != StateSchemaVersion {
		return errors.Wrapf(ErrInvalidState, "unsupported state version %d", state.Version)
	}
	stateRoot, err := filepath.Abs(state.RepoRoot)
	if err != nil {
		return errors.Wrap(err, "validate environment repository root")
	}
	if filepath.Clean(stateRoot) != s.repoRoot {
		return errors.Wrapf(ErrInvalidState, "state repository root %q does not match %q", state.RepoRoot, s.repoRoot)
	}
	if state.Revision == 0 {
		return errors.Wrap(ErrInvalidState, "state revision must be positive")
	}
	if state.CreatedAt.IsZero() || state.UpdatedAt.IsZero() {
		return errors.Wrap(ErrInvalidState, "state timestamps are required")
	}
	if state.Services == nil {
		return errors.Wrap(ErrInvalidState, "state services map is required")
	}
	for key, slot := range state.Services {
		if err := validateServiceName(key); err != nil {
			return err
		}
		if slot.Name != key {
			return errors.Wrapf(ErrInvalidState, "service slot %q has name %q", key, slot.Name)
		}
		if !validDesiredState(slot.Desired) {
			return errors.Wrapf(ErrInvalidState, "service %q has invalid desired state %q", key, slot.Desired)
		}
		if slot.CurrentRunID != "" {
			if err := validateRunID(slot.CurrentRunID); err != nil {
				return err
			}
		}
		if slot.LastRunID != "" {
			if err := validateRunID(slot.LastRunID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) validateRun(run *RunRecord) error {
	if run.Version != RunSchemaVersion {
		return errors.Wrapf(ErrInvalidState, "unsupported run version %d", run.Version)
	}
	if err := validateRunID(run.RunID); err != nil {
		return err
	}
	if err := validateServiceName(run.Service); err != nil {
		return err
	}
	if run.Spec.Name != run.Service {
		return errors.Wrapf(ErrInvalidState, "run service %q does not match spec name %q", run.Service, run.Spec.Name)
	}
	if len(run.Spec.Command) == 0 {
		return errors.Wrapf(ErrInvalidState, "run %q has an empty command", run.RunID)
	}
	if !validRunPhase(run.Phase) {
		return errors.Wrapf(ErrInvalidState, "run %q has invalid phase %q", run.RunID, run.Phase)
	}
	if run.CreatedAt.IsZero() || run.UpdatedAt.IsZero() {
		return errors.Wrapf(ErrInvalidState, "run %q timestamps are required", run.RunID)
	}
	expectedArtifact := filepath.ToSlash(filepath.Join(stateDirName, runsDirName, run.RunID))
	if run.ArtifactDir != expectedArtifact {
		return errors.Wrapf(ErrInvalidPath, "run %q artifact directory %q does not match %q", run.RunID, run.ArtifactDir, expectedArtifact)
	}
	return nil
}

func readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "read runstate JSON")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return errors.Wrap(err, "parse runstate JSON")
	}
	return nil
}

func safeJoin(base string, elements ...string) (string, error) {
	path := filepath.Join(append([]string{base}, elements...)...)
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return "", errors.Wrap(err, "validate runstate path")
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.Wrapf(ErrInvalidPath, "path %q escapes %q", path, base)
	}
	return path, nil
}

func validateRunID(runID string) error {
	parsed, err := uuid.Parse(runID)
	if err != nil || parsed.Version() != 7 || parsed.String() != strings.ToLower(runID) {
		return errors.Wrapf(ErrInvalidPath, "invalid UUIDv7 run ID %q", runID)
	}
	return nil
}

func validateServiceName(name string) error {
	if !serviceNamePattern.MatchString(name) || name == "." || name == ".." {
		return errors.Wrapf(ErrInvalidPath, "invalid service name %q", name)
	}
	return nil
}

func cloneEnvironment(state EnvironmentState) EnvironmentState {
	clone := state
	clone.Services = make(map[string]ServiceSlot, len(state.Services))
	for name, slot := range state.Services {
		clone.Services[name] = slot
	}
	return clone
}
