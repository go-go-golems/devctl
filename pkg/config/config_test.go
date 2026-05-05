package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOptionalMissingReturnsEmptyConfig(t *testing.T) {
	cfg, err := LoadOptional(filepath.Join(t.TempDir(), ".devctl.yaml"))
	if err != nil {
		t.Fatalf("LoadOptional returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if got := cfg.ResolveProfile(""); got != "" {
		t.Fatalf("expected no active profile, got %q", got)
	}
	if len(cfg.Plugins) != 0 {
		t.Fatalf("expected no plugins, got %d", len(cfg.Plugins))
	}
}

func TestLoadStackedMissingOverrideKeepsBaseConfig(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, ".devctl.yaml")
	writeFile(t, basePath, `
profile:
  active: development
profiles:
  development:
    plugins: [api]
plugins:
  - id: api
    path: ./plugins/devctl-api
`)

	cfg, err := LoadStacked(basePath, filepath.Join(dir, ".devctl.override.yaml"))
	if err != nil {
		t.Fatalf("LoadStacked returned error: %v", err)
	}
	if got := cfg.ResolveProfile(""); got != "development" {
		t.Fatalf("expected development profile, got %q", got)
	}
	if err := cfg.ValidateProfile("development"); err != nil {
		t.Fatalf("expected valid development profile: %v", err)
	}
}

func TestLoadStackedMergesOverrideProfilesAndActiveProfile(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, ".devctl.yaml")
	overridePath := filepath.Join(dir, ".devctl.override.yaml")
	writeFile(t, basePath, `
profile:
  active: development
profiles:
  development:
    display_name: Development
    description: Shared development profile
    plugins: [api, web]
    env:
      LOG_LEVEL: debug
      SHARED: base
plugins:
  - id: api
    path: ./plugins/devctl-api
    env:
      API_MODE: base
  - id: web
    path: ./plugins/devctl-web
`)
	writeFile(t, overridePath, `
profile:
  active: manuel-debug
profiles:
  development:
    env:
      LOG_LEVEL: trace
      LOCAL_ONLY: "1"
  manuel-debug:
    display_name: Manuel Debug
    plugins: [api, local]
    env:
      LOG_LEVEL: trace
plugins:
  - id: api
    env:
      API_MODE: local
      API_TRACE: "1"
  - id: local
    path: ./plugins/devctl-local
`)

	cfg, err := LoadStacked(basePath, overridePath)
	if err != nil {
		t.Fatalf("LoadStacked returned error: %v", err)
	}
	if got := cfg.ResolveProfile(""); got != "manuel-debug" {
		t.Fatalf("expected override active profile, got %q", got)
	}
	if got := cfg.ResolveProfile("development"); got != "development" {
		t.Fatalf("expected explicit profile to win, got %q", got)
	}

	dev := cfg.GetProfile("development")
	if dev == nil {
		t.Fatal("expected development profile")
	}
	assertStringSlice(t, dev.Plugins, []string{"api", "web"})
	assertMapValue(t, dev.Env, "LOG_LEVEL", "trace")
	assertMapValue(t, dev.Env, "SHARED", "base")
	assertMapValue(t, dev.Env, "LOCAL_ONLY", "1")

	if err := cfg.ValidateProfile("manuel-debug"); err != nil {
		t.Fatalf("expected override-defined profile to validate: %v", err)
	}

	if len(cfg.Plugins) != 3 {
		t.Fatalf("expected 3 merged plugins, got %d", len(cfg.Plugins))
	}
	api := findPlugin(t, cfg, "api")
	assertMapValue(t, api.Env, "API_MODE", "local")
	assertMapValue(t, api.Env, "API_TRACE", "1")
	if api.Path != "./plugins/devctl-api" {
		t.Fatalf("expected base path to be preserved, got %q", api.Path)
	}
	local := findPlugin(t, cfg, "local")
	if local.Path != "./plugins/devctl-local" {
		t.Fatalf("expected local plugin path, got %q", local.Path)
	}
}

func TestDefaultProfileIsExplicitNotImplicit(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, ".devctl.yaml")
	writeFile(t, basePath, `
profiles:
  default:
    plugins: [api]
plugins:
  - id: api
    path: ./plugins/devctl-api
  - id: web
    path: ./plugins/devctl-web
`)

	cfg, err := LoadStacked(basePath, "")
	if err != nil {
		t.Fatalf("LoadStacked returned error: %v", err)
	}
	if got := cfg.ResolveProfile(""); got != "" {
		t.Fatalf("expected no implicit default profile, got %q", got)
	}
	if got := cfg.ResolveProfile("default"); got != "default" {
		t.Fatalf("expected explicit default profile, got %q", got)
	}
	if err := cfg.ValidateProfile("default"); err != nil {
		t.Fatalf("expected default profile to validate: %v", err)
	}
}

func TestValidateProfileRejectsUnknownProfileAndPlugin(t *testing.T) {
	cfg := &File{
		Profiles: map[string]*Profile{
			"bad": {Plugins: []string{"missing"}},
		},
		Plugins: []Plugin{{ID: "api", Path: "./plugins/devctl-api"}},
	}
	if err := cfg.ValidateProfile("missing-profile"); err == nil {
		t.Fatal("expected missing profile error")
	}
	if err := cfg.ValidateProfile("bad"); err == nil {
		t.Fatal("expected unknown plugin error")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findPlugin(t *testing.T, cfg *File, id string) Plugin {
	t.Helper()
	for _, plugin := range cfg.Plugins {
		if plugin.ID == id {
			return plugin
		}
	}
	t.Fatalf("plugin %q not found", id)
	return Plugin{}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func assertMapValue(t *testing.T, got map[string]string, key, want string) {
	t.Helper()
	if got[key] != want {
		t.Fatalf("expected %s=%q, got %q in %#v", key, want, got[key], got)
	}
}
