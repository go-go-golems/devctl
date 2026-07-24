package plugincatalog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/repository"
)

func TestStaticCatalogFingerprintIsStableAcrossPluginOrdering(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, config.DefaultConfigFilename)
	first := `plugins:
  - id: beta
    path: /bin/true
    commands: [{name: beta-command}]
  - id: alpha
    path: /bin/true
    commands: [{name: alpha-command}]
`
	if err := os.WriteFile(configPath, []byte(first), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	firstRepo := loadCatalogRepo(t, repoRoot, configPath, "")
	firstFingerprint, err := Fingerprint(firstRepo)
	if err != nil {
		t.Fatalf("first fingerprint: %v", err)
	}
	second := `plugins:
  - id: alpha
    path: /bin/true
    commands: [{name: alpha-command}]
  - id: beta
    path: /bin/true
    commands: [{name: beta-command}]
`
	if err := os.WriteFile(configPath, []byte(second), 0o600); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	secondRepo := loadCatalogRepo(t, repoRoot, configPath, "")
	secondFingerprint, err := Fingerprint(secondRepo)
	if err != nil {
		t.Fatalf("second fingerprint: %v", err)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("fingerprint changed with ordering: %s != %s", firstFingerprint, secondFingerprint)
	}
}

func TestRefreshRejectsDeterministicPluginCollision(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, config.DefaultConfigFilename)
	content := `plugins:
  - id: beta
    path: /bin/true
    commands: [{name: collide}]
  - id: alpha
    path: /bin/true
    commands: [{name: collide}]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	repo := loadCatalogRepo(t, repoRoot, configPath, "")
	_, err := Refresh(t.Context(), repo, RefreshOptions{})
	if !errors.Is(err, ErrCatalogConflict) {
		t.Fatalf("refresh error = %v, want conflict", err)
	}
	if _, statErr := os.Stat(CachePath(repoRoot)); !os.IsNotExist(statErr) {
		t.Fatalf("conflicting catalog was cached: %v", statErr)
	}
	staticCatalog, err := Static(repo, nil)
	if err != nil {
		t.Fatalf("static catalog: %v", err)
	}
	conflicts := staticCatalog.Conflicts["collide"]
	if len(conflicts) != 2 ||
		conflicts[0].ProviderID != "alpha" ||
		conflicts[1].ProviderID != "beta" {
		t.Fatalf("conflicts are not deterministic: %#v", conflicts)
	}
}

func TestRefreshRejectsReservedCommand(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, config.DefaultConfigFilename)
	content := `plugins:
  - id: plugin
    path: /bin/true
    commands: [{name: status}]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	repo := loadCatalogRepo(t, repoRoot, configPath, "")
	_, err := Refresh(t.Context(), repo, RefreshOptions{
		Reserved: map[string]bool{"status": true},
	})
	if !errors.Is(err, ErrCatalogConflict) {
		t.Fatalf("refresh error = %v, want reserved-name conflict", err)
	}
}

func TestCatalogCannotBeReusedAcrossProfiles(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, config.DefaultConfigFilename)
	content := `profiles:
  alpha: {plugins: [alpha]}
  beta: {plugins: [beta]}
plugins:
  - id: alpha
    path: /bin/true
    commands: [{name: alpha-command}]
  - id: beta
    path: /bin/true
    commands: [{name: beta-command}]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	alpha := loadCatalogRepo(t, repoRoot, configPath, "alpha")
	if _, err := Refresh(t.Context(), alpha, RefreshOptions{}); err != nil {
		t.Fatalf("refresh alpha: %v", err)
	}
	beta := loadCatalogRepo(t, repoRoot, configPath, "beta")
	if _, err := Load(beta, nil); !errors.Is(err, ErrCatalogStale) {
		t.Fatalf("load beta with alpha cache = %v, want stale", err)
	}
}

func loadCatalogRepo(t *testing.T, repoRoot, configPath, profile string) *repository.Repository {
	t.Helper()
	repo, err := repository.Load(repository.Options{
		RepoRoot: repoRoot, ConfigPath: configPath, ProfileName: profile, Cwd: repoRoot,
	})
	if err != nil {
		t.Fatalf("load repository: %v", err)
	}
	return repo
}
