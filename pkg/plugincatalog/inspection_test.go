package plugincatalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/devctl/pkg/config"
)

func TestInspectReportsMissingWithoutStartingProvider(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, config.DefaultConfigFilename)
	marker := filepath.Join(repoRoot, "started")
	content := "plugins:\n  - id: dynamic\n    path: /bin/sh\n    args: [-c, 'touch " + marker + "']\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	repo := loadCatalogRepo(t, repoRoot, configPath, "")
	inspection, err := Inspect(repo, nil)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspection.State != CatalogMissing {
		t.Fatalf("state = %q, want %q", inspection.State, CatalogMissing)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("inspection started provider; marker stat error = %v", err)
	}
}

func TestInspectReportsDeclaredSourceChangeAsStale(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, config.DefaultConfigFilename)
	scriptPath := filepath.Join(repoRoot, "plugin.py")
	content := `plugins:
  - id: plugin
    path: python3
    args: [plugin.py]
    commands: [{name: hello}]
    catalog_inputs: [plugin.py]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	repo := loadCatalogRepo(t, repoRoot, configPath, "")
	if _, err := Refresh(t.Context(), repo, RefreshOptions{}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("rewrite source: %v", err)
	}
	inspection, err := Inspect(repo, nil)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspection.State != CatalogStale {
		t.Fatalf("state = %q, want %q", inspection.State, CatalogStale)
	}
}
