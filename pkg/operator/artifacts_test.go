package operator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runstate"
)

func writeExecutableArtifact(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir artifact: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
}

func TestStageAndPublishReferencedArtifact(t *testing.T) {
	repoRoot := t.TempDir()
	source := filepath.Join(repoRoot, "build", "api")
	writeExecutableArtifact(t, source, "#!/bin/sh\necho api\n")
	recipe := LifecycleRecipe{
		Version:  LifecycleRecipeSchemaVersion,
		ID:       "018f0f65-6c1a-7abc-8def-0123456789ab",
		RepoRoot: repoRoot,
	}
	plan := engine.LaunchPlan{Services: []engine.ServiceSpec{{
		Name: "api", Executable: &engine.ExecutableRef{ArtifactID: "api", Args: []string{"--port", "8080"}},
	}}}
	records, err := stageReferencedArtifacts(
		t.Context(), recipe, &plan,
		&engine.BuildResult{Artifacts: map[string]string{"api": source}}, nil,
	)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	prepared := PreparedLaunch{Version: LifecycleRecipeSchemaVersion, Recipe: recipe, Plan: plan, Artifacts: records}
	if err := validatePreparedArtifacts(prepared); err != nil {
		t.Fatalf("validate staged: %v", err)
	}
	if err := publishPreparedArtifacts(&prepared); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(prepared.Artifacts) != 1 {
		t.Fatalf("artifacts = %#v", prepared.Artifacts)
	}
	record := prepared.Artifacts[0]
	wantDir := filepath.Join(repoRoot, ".devctl", "artifacts", "sha256", record.SHA256)
	if filepath.Dir(record.Path) != wantDir {
		t.Fatalf("published path = %q, want directory %q", record.Path, wantDir)
	}
	if got := prepared.Plan.Services[0].Command; len(got) != 3 || got[0] != record.Path {
		t.Fatalf("resolved command = %v", got)
	}
	if err := runstate.ValidateArtifact(record); err != nil {
		t.Fatalf("validate published: %v", err)
	}

	secondRecipe := recipe
	secondRecipe.ID = "018f0f65-6c1a-7abc-8def-0123456789ac"
	secondPlan := engine.LaunchPlan{Services: []engine.ServiceSpec{{
		Name: "worker", Executable: &engine.ExecutableRef{ArtifactID: "api"},
	}}}
	secondRecords, err := stageReferencedArtifacts(
		t.Context(), secondRecipe, &secondPlan,
		&engine.BuildResult{Artifacts: map[string]string{"api": source}}, nil,
	)
	if err != nil {
		t.Fatalf("stage identical artifact: %v", err)
	}
	second := PreparedLaunch{Version: LifecycleRecipeSchemaVersion, Recipe: secondRecipe, Plan: secondPlan, Artifacts: secondRecords}
	if err := publishPreparedArtifacts(&second); err != nil {
		t.Fatalf("publish identical artifact: %v", err)
	}
	if second.Artifacts[0].Path != record.Path {
		t.Fatalf("identical artifact path = %q, want reused %q", second.Artifacts[0].Path, record.Path)
	}
}

func TestStageRejectsAmbiguousAndMissingArtifactReferences(t *testing.T) {
	repoRoot := t.TempDir()
	source := filepath.Join(repoRoot, "api")
	writeExecutableArtifact(t, source, "api")
	recipe := LifecycleRecipe{ID: "recipe", RepoRoot: repoRoot}
	tests := []struct {
		name string
		spec engine.ServiceSpec
	}{
		{name: "both command forms", spec: engine.ServiceSpec{
			Name: "api", Command: []string{"api"}, Executable: &engine.ExecutableRef{ArtifactID: "api"},
		}},
		{name: "missing artifact", spec: engine.ServiceSpec{
			Name: "api", Executable: &engine.ExecutableRef{ArtifactID: "missing"},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := engine.LaunchPlan{Services: []engine.ServiceSpec{test.spec}}
			_, err := stageReferencedArtifacts(t.Context(), recipe, &plan,
				&engine.BuildResult{Artifacts: map[string]string{"api": source}}, nil)
			if err == nil {
				t.Fatal("stage unexpectedly succeeded")
			}
		})
	}
}

func TestCollectArtifactsProtectsCurrentAndLastRuns(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := runstate.NewStore(repoRoot)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	currentID := "018f0f65-6c1a-7abc-8def-0123456789ab"
	lastID := "018f0f65-6c1a-7abc-8def-0123456789ac"
	protected := []struct {
		runID  string
		digest string
	}{
		{currentID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{lastID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	for _, item := range protected {
		run := runstate.RunRecord{
			RunID: item.runID, Service: "api", Phase: runstate.RunExited,
			Spec: runstate.ServiceSpecRecord{Name: "api", Command: []string{"/bin/true"}},
			Artifact: &runstate.ArtifactRecord{
				ID: "api", Path: filepath.Join(repoRoot, ".devctl", "artifacts", "sha256", item.digest, "executable"),
				SHA256: item.digest, SizeBytes: 1,
			},
		}
		if err := store.CreateRun(context.Background(), run); err != nil {
			t.Fatalf("create run: %v", err)
		}
	}
	if err := store.CreateEnvironment(context.Background(), runstate.EnvironmentState{
		Services: map[string]runstate.ServiceSlot{"api": {
			Name: "api", CurrentRunID: currentID, LastRunID: lastID, Desired: runstate.DesiredRunning,
		}},
	}); err != nil {
		t.Fatalf("create environment: %v", err)
	}
	artifactRoot := filepath.Join(repoRoot, ".devctl", "artifacts", "sha256")
	orphan := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	for _, digest := range []string{protected[0].digest, protected[1].digest, orphan} {
		writeExecutableArtifact(t, filepath.Join(artifactRoot, digest, "api"), digest)
	}
	if err := collectArtifacts(t.Context(), store); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, item := range protected {
		if _, err := os.Stat(filepath.Join(artifactRoot, item.digest)); err != nil {
			t.Fatalf("protected digest %s removed: %v", item.digest, err)
		}
	}
	if _, err := os.Stat(filepath.Join(artifactRoot, orphan)); !os.IsNotExist(err) {
		t.Fatalf("orphan digest still exists: %v", err)
	}
}

func TestPreparedArtifactCorruptionIsRejected(t *testing.T) {
	repoRoot := t.TempDir()
	path := filepath.Join(repoRoot, "staged")
	writeExecutableArtifact(t, path, "first")
	digest, size, _, err := runstate.InspectArtifact(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	prepared := PreparedLaunch{Artifacts: []runstate.ArtifactRecord{{
		ID: "api", Path: path, SHA256: digest, SizeBytes: size,
	}}}
	writeExecutableArtifact(t, path, "changed")
	if err := validatePreparedArtifacts(prepared); err == nil {
		t.Fatal("corrupt staged artifact unexpectedly validated")
	}
}
