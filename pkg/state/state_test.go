package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceSpecRecordRoundTrip(t *testing.T) {
	rec := ServiceRecord{
		Name:      "api-server",
		PID:       12345,
		Command:   []string{"go", "run", "./cmd/api"},
		Cwd:       "/path/to/repo",
		Env:       map[string]string{"LOG_LEVEL": "[REDACTED]"},
		StdoutLog: ".devctl/logs/api.stdout.log",
		StderrLog: ".devctl/logs/api.stderr.log",
		StartedAt: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
		Spec: &ServiceSpecRecord{
			Name:    "api-server",
			Cwd:     "/path/to/repo",
			Command: []string{"go", "run", "./cmd/api"},
			Health: &HealthCheckRecord{
				Type: "http",
				URL:  "http://localhost:8080/health",
			},
		},
	}

	// Marshal to JSON
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Unmarshal back
	var rec2 ServiceRecord
	if err := json.Unmarshal(b, &rec2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify Spec was preserved
	if rec2.Spec == nil {
		t.Fatal("Spec is nil after round-trip")
	}
	if rec2.Spec.Name != "api-server" {
		t.Errorf("Spec.Name = %q, want %q", rec2.Spec.Name, "api-server")
	}
	if len(rec2.Spec.Command) != 3 || rec2.Spec.Command[0] != "go" {
		t.Errorf("Spec.Command = %v, want [go run ./cmd/api]", rec2.Spec.Command)
	}
	if rec2.Spec.Health == nil {
		t.Fatal("Spec.Health is nil")
	}
	if rec2.Spec.Health.Type != "http" {
		t.Errorf("Spec.Health.Type = %q, want %q", rec2.Spec.Health.Type, "http")
	}
	if rec2.Spec.Health.URL != "http://localhost:8080/health" {
		t.Errorf("Spec.Health.URL = %q, want health URL", rec2.Spec.Health.URL)
	}
}

func TestLoadStateWithoutSpec(t *testing.T) {
	// Verify backward compatibility: old state files without Spec field load successfully.
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".devctl", "state.json")

	oldState := `{
  "repo_root": "/path/to/repo",
  "created_at": "2026-05-04T10:00:00Z",
  "services": [
    {
      "name": "api-server",
      "pid": 12345,
      "command": ["go", "run", "./cmd/api"],
      "cwd": "/path/to/repo",
      "env": {"LOG_LEVEL": "[REDACTED]"},
      "stdout_log": ".devctl/logs/api.stdout.log",
      "stderr_log": ".devctl/logs/api.stderr.log"
    }
  ]
}`

	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(oldState), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(st.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(st.Services))
	}
	svc := st.Services[0]
	if svc.Name != "api-server" {
		t.Errorf("Name = %q, want %q", svc.Name, "api-server")
	}
	if svc.Spec != nil {
		t.Error("Spec should be nil for old state files, got non-nil")
	}
}

func TestSaveLoadWithSpec(t *testing.T) {
	dir := t.TempDir()

	st := &State{
		RepoRoot:  dir,
		Profile:   "development",
		CreatedAt: time.Now(),
		Services: []ServiceRecord{
			{
				Name:    "web",
				PID:     0,
				Command: []string{"npm", "run", "dev"},
				Cwd:     dir,
				Spec: &ServiceSpecRecord{
					Name:    "web",
					Command: []string{"npm", "run", "dev"},
				},
			},
		},
	}

	if err := Save(dir, st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Profile != "development" {
		t.Fatalf("Profile = %q, want development", loaded.Profile)
	}
	if len(loaded.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(loaded.Services))
	}
	svc := loaded.Services[0]
	if svc.Spec == nil {
		t.Fatal("Spec is nil after save/load")
	}
	if len(svc.Spec.Command) != 3 || svc.Spec.Command[0] != "npm" {
		t.Errorf("Spec.Command = %v, want [npm run dev]", svc.Spec.Command)
	}
}
