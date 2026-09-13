package operator

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/pkg/errors"
)

var (
	artifactIDPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	artifactDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type producedArtifact struct {
	path string
}

// stageReferencedArtifacts copies only executable artifacts selected by the
// launch plan into operation-owned staging. Published content-addressed files
// are created later while the lifecycle lock is held.
func stageReferencedArtifacts(
	ctx context.Context,
	recipe LifecycleRecipe,
	plan *engine.LaunchPlan,
	build *engine.BuildResult,
	prepare *engine.PrepareResult,
) ([]runstate.ArtifactRecord, error) {
	complete := false
	defer func() {
		if !recipe.Policy.DryRun && !complete {
			_ = os.RemoveAll(filepath.Join(
				recipe.RepoRoot, ".devctl", "artifacts", ".staging", recipe.ID,
			))
		}
	}()
	produced := map[string]producedArtifact{}
	if build != nil {
		for id, path := range build.Artifacts {
			produced[id] = producedArtifact{path: path}
		}
	}
	if prepare != nil {
		for id, path := range prepare.Artifacts {
			if previous, exists := produced[id]; exists && previous.path != path {
				return nil, errors.Errorf("artifact %q has conflicting build and prepare outputs", id)
			}
			produced[id] = producedArtifact{path: path}
		}
	}

	records := make([]runstate.ArtifactRecord, 0)
	staged := map[string]runstate.ArtifactRecord{}
	for index := range plan.Services {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		service := &plan.Services[index]
		if service.Executable == nil {
			if len(service.Command) == 0 {
				return nil, errors.Errorf("service %q has neither command nor executable artifact", service.Name)
			}
			continue
		}
		if len(service.Command) > 0 {
			return nil, errors.Errorf("service %q declares both command and executable artifact", service.Name)
		}
		id := service.Executable.ArtifactID
		if !artifactIDPattern.MatchString(id) {
			return nil, errors.Errorf("service %q has invalid executable artifact ID %q", service.Name, id)
		}
		output, exists := produced[id]
		if !exists {
			return nil, errors.Errorf("service %q references undeclared executable artifact %q", service.Name, id)
		}
		if output.path == "" {
			return nil, errors.Errorf("executable artifact %q declares an empty output path", id)
		}
		if recipe.Policy.DryRun {
			// Providers report intended outputs during dry runs; do not require or
			// copy bytes that correctly have not been produced.
			continue
		}
		record, exists := staged[id]
		if !exists {
			var err error
			record, err = stageExecutableArtifact(recipe, id, output.path)
			if err != nil {
				return nil, errors.Wrapf(err, "stage executable artifact %q", id)
			}
			staged[id] = record
			records = append(records, record)
		}
		service.Command = append([]string{record.Path}, service.Executable.Args...)
	}
	complete = true
	return records, nil
}

func stageExecutableArtifact(recipe LifecycleRecipe, id, source string) (runstate.ArtifactRecord, error) {
	if !filepath.IsAbs(source) {
		source = filepath.Join(recipe.RepoRoot, source)
	}
	digest, size, mode, err := runstate.InspectArtifact(source)
	if err != nil {
		return runstate.ArtifactRecord{}, err
	}
	if mode.Perm()&0o111 == 0 {
		return runstate.ArtifactRecord{}, errors.New("produced artifact is not executable")
	}
	stagingRoot := filepath.Join(recipe.RepoRoot, ".devctl", "artifacts", ".staging", recipe.ID)
	if err := os.MkdirAll(stagingRoot, 0o700); err != nil {
		return runstate.ArtifactRecord{}, errors.Wrap(err, "create artifact staging directory")
	}
	destination := filepath.Join(stagingRoot, id)
	if err := copyArtifactExclusive(source, destination, mode.Perm()); err != nil {
		return runstate.ArtifactRecord{}, err
	}
	record := runstate.ArtifactRecord{ID: id, Path: destination, SHA256: digest, SizeBytes: size}
	if err := runstate.ValidateArtifact(record); err != nil {
		return runstate.ArtifactRecord{}, errors.Wrap(err, "validate staged artifact")
	}
	return record, nil
}

// publishPreparedArtifacts moves staged files into the content-addressed store
// and rewrites service commands to select those immutable paths. The caller
// must hold the repository lifecycle lock.
func publishPreparedArtifacts(prepared *PreparedLaunch) error {
	published := make(map[string]runstate.ArtifactRecord, len(prepared.Artifacts))
	for index := range prepared.Artifacts {
		record := prepared.Artifacts[index]
		if err := runstate.ValidateArtifact(record); err != nil {
			return errors.Wrapf(err, "validate staged artifact %q", record.ID)
		}
		destinationDir := filepath.Join(
			prepared.Recipe.RepoRoot, ".devctl", "artifacts", "sha256", record.SHA256,
		)
		if err := os.MkdirAll(destinationDir, 0o700); err != nil {
			return errors.Wrap(err, "create content-addressed artifact directory")
		}
		destination := filepath.Join(destinationDir, "executable")
		if info, err := os.Lstat(destination); err == nil {
			if !info.Mode().IsRegular() {
				return errors.New("content-addressed artifact destination is not a regular file")
			}
			existing := record
			existing.Path = destination
			if err := runstate.ValidateArtifact(existing); err != nil {
				return errors.Wrap(err, "existing content-addressed artifact is corrupt")
			}
			if err := os.Remove(record.Path); err != nil {
				return errors.Wrap(err, "remove duplicate staged artifact")
			}
		} else if !os.IsNotExist(err) {
			return errors.Wrap(err, "inspect content-addressed artifact destination")
		} else if err := os.Rename(record.Path, destination); err != nil {
			return errors.Wrap(err, "publish content-addressed artifact")
		}
		if err := os.Chmod(destination, 0o500); err != nil {
			return errors.Wrap(err, "make content-addressed artifact read-only")
		}
		record.Path = destination
		if err := runstate.ValidateArtifact(record); err != nil {
			return errors.Wrapf(err, "validate published artifact %q", record.ID)
		}
		prepared.Artifacts[index] = record
		published[record.ID] = record
	}
	for index := range prepared.Plan.Services {
		service := &prepared.Plan.Services[index]
		if service.Executable == nil {
			continue
		}
		record, exists := published[service.Executable.ArtifactID]
		if !exists {
			return errors.Errorf("service %q prepared artifact is missing", service.Name)
		}
		service.Command = append([]string{record.Path}, service.Executable.Args...)
	}
	_ = os.RemoveAll(filepath.Join(prepared.Recipe.RepoRoot, ".devctl", "artifacts", ".staging", prepared.Recipe.ID))
	return nil
}

func copyArtifactExclusive(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source) // #nosec G304 -- source is a plugin-declared build output.
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) // #nosec G304 -- destination is under the repository artifact root.
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		_ = output.Close()
		if remove {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	remove = false
	return nil
}

func cleanupPreparedArtifacts(prepared PreparedLaunch) {
	_ = os.RemoveAll(filepath.Join(
		prepared.Recipe.RepoRoot, ".devctl", "artifacts", ".staging", prepared.Recipe.ID,
	))
}

func artifactForService(service engine.ServiceSpec, records []runstate.ArtifactRecord) *runstate.ArtifactRecord {
	if service.Executable == nil {
		return nil
	}
	for _, record := range records {
		if record.ID == service.Executable.ArtifactID {
			selected := record
			return &selected
		}
	}
	return nil
}

func validatePreparedArtifacts(prepared PreparedLaunch) error {
	byID := make(map[string]runstate.ArtifactRecord, len(prepared.Artifacts))
	for _, record := range prepared.Artifacts {
		if _, exists := byID[record.ID]; exists {
			return errors.Errorf("prepared artifact %q is duplicated", record.ID)
		}
		if err := runstate.ValidateArtifact(record); err != nil {
			return errors.Wrapf(err, "prepared artifact %q", record.ID)
		}
		byID[record.ID] = record
	}
	for _, service := range prepared.Plan.Services {
		if service.Executable != nil {
			if _, exists := byID[service.Executable.ArtifactID]; !exists {
				return errors.Errorf("service %q prepared artifact is missing", service.Name)
			}
		}
	}
	return nil
}

// collectArtifacts removes content-addressed objects not selected by any
// current or last service run. Older run records retain digest evidence even
// after their executable bytes are collected. The caller must hold the
// lifecycle lock.
func collectArtifacts(ctx context.Context, store *runstate.Store) error {
	protected, err := protectedArtifactDigests(ctx, store)
	if err != nil {
		return err
	}
	root := filepath.Join(store.RepoRoot(), ".devctl", "artifacts", "sha256")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || protected[entry.Name()] {
			continue
		}
		if !artifactDigestPattern.MatchString(entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// protectedArtifactDigests validates every run referenced by artifact
// retention before lifecycle mutation. Legacy schemas remain a clean-cut
// migration error instead of surfacing after a new service has started.
func protectedArtifactDigests(ctx context.Context, store *runstate.Store) (map[string]bool, error) {
	protected := map[string]bool{}
	environment, err := loadEnvironmentOptional(ctx, store)
	if err != nil {
		return nil, err
	}
	if environment == nil {
		return protected, nil
	}
	for _, slot := range environment.Services {
		for _, runID := range []string{slot.CurrentRunID, slot.LastRunID} {
			if runID == "" {
				continue
			}
			run, err := store.LoadRun(ctx, runID)
			if err != nil {
				return nil, err
			}
			if run.Artifact != nil {
				protected[run.Artifact.SHA256] = true
			}
		}
	}
	return protected, nil
}
