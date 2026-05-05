package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithoutProfileLoadsAllTopLevelPlugins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".devctl.yaml"), `
plugins:
  - id: api
    path: devctl-api
    priority: 20
  - id: web
    path: devctl-web
    priority: 10
`)

	repo, err := Load(Options{RepoRoot: dir})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if repo.ProfileName != "" {
		t.Fatalf("expected no profile, got %q", repo.ProfileName)
	}
	assertSpecIDs(t, repo, []string{"web", "api"})
}

func TestLoadWithExplicitDefaultProfileDoesNotMakeDefaultImplicit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".devctl.yaml"), `
profiles:
  default:
    plugins: [api]
plugins:
  - id: api
    path: devctl-api
    priority: 20
  - id: web
    path: devctl-web
    priority: 10
`)

	repo, err := Load(Options{RepoRoot: dir})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	assertSpecIDs(t, repo, []string{"web", "api"})

	repo, err = Load(Options{RepoRoot: dir, ProfileName: "default"})
	if err != nil {
		t.Fatalf("Load with default profile returned error: %v", err)
	}
	if repo.ProfileName != "default" {
		t.Fatalf("expected default profile, got %q", repo.ProfileName)
	}
	assertSpecIDs(t, repo, []string{"api"})
}

func TestLoadFiltersPluginsAndMergesProfileEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".devctl.yaml"), `
profile:
  active: backend
profiles:
  backend:
    plugins: [api, db]
    env:
      LOG_LEVEL: debug
      PORT: "8080"
plugins:
  - id: api
    path: devctl-api
    priority: 30
    env:
      PORT: "3000"
      API_ONLY: "1"
  - id: web
    path: devctl-web
    priority: 10
  - id: db
    path: devctl-db
    priority: 20
`)

	repo, err := Load(Options{RepoRoot: dir})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	assertSpecIDs(t, repo, []string{"db", "api"})
	api := repo.SpecByID["api"]
	assertMapValue(t, api.Env, "PORT", "8080")
	assertMapValue(t, api.Env, "API_ONLY", "1")
	assertMapValue(t, api.Env, "LOG_LEVEL", "debug")
}

func TestLoadUsesOverrideDefinedProfileAndPluginPatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".devctl.yaml"), `
plugins:
  - id: api
    path: devctl-api
    env:
      API_MODE: base
  - id: web
    path: devctl-web
`)
	writeFile(t, filepath.Join(dir, ".devctl.override.yaml"), `
profile:
  active: local
profiles:
  local:
    plugins: [api, local-helper]
    env:
      LOG_LEVEL: trace
plugins:
  - id: api
    env:
      API_MODE: local
  - id: local-helper
    path: devctl-local-helper
`)

	repo, err := Load(Options{RepoRoot: dir})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if repo.ProfileName != "local" {
		t.Fatalf("expected local profile, got %q", repo.ProfileName)
	}
	assertSpecIDs(t, repo, []string{"api", "local-helper"})
	api := repo.SpecByID["api"]
	assertMapValue(t, api.Env, "API_MODE", "local")
	assertMapValue(t, api.Env, "LOG_LEVEL", "trace")
}

func TestLoadProfileUnknownPluginReturnsClearError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".devctl.yaml"), `
profile:
  active: bad
profiles:
  bad:
    plugins: [missing]
plugins:
  - id: api
    path: devctl-api
`)

	_, err := Load(Options{RepoRoot: dir})
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), `profile "bad" references unknown plugin "missing"`; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertSpecIDs(t *testing.T, repo *Repository, want []string) {
	t.Helper()
	if len(repo.Specs) != len(want) {
		t.Fatalf("expected specs %v, got %#v", want, repo.Specs)
	}
	for i, id := range want {
		if repo.Specs[i].ID != id {
			t.Fatalf("expected specs %v, got %#v", want, repo.Specs)
		}
	}
}

func assertMapValue(t *testing.T, got map[string]string, key, want string) {
	t.Helper()
	if got[key] != want {
		t.Fatalf("expected %s=%q, got %q in %#v", key, want, got[key], got)
	}
}
