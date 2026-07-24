package testrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
)

// Repository is an isolated repository root for tests that exercise devctl
// configuration or runtime artifacts. It is always rooted under testing.TB's
// temporary directory and never resolves to a developer's working tree.
type Repository struct {
	Root       string
	DevctlDir  string
	ConfigPath string
}

// New creates an isolated repository root with a private .devctl directory.
func New(t testing.TB) *Repository {
	t.Helper()

	root := t.TempDir()
	devctlDir := filepath.Join(root, ".devctl")
	if err := os.MkdirAll(devctlDir, 0o700); err != nil {
		t.Fatalf("create isolated .devctl directory: %v", err)
	}

	return &Repository{
		Root:       root,
		DevctlDir:  devctlDir,
		ConfigPath: filepath.Join(root, ".devctl.yaml"),
	}
}

// Path joins trusted test path elements beneath the isolated repository root.
func (r *Repository) Path(elements ...string) string {
	parts := append([]string{r.Root}, elements...)
	return filepath.Join(parts...)
}

// WriteConfig writes the repository's base devctl configuration.
func (r *Repository) WriteConfig(contents []byte) error {
	if err := os.WriteFile(r.ConfigPath, contents, 0o600); err != nil {
		return errors.Wrap(err, "write isolated devctl config")
	}
	return nil
}
