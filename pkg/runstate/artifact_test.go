package runstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateArtifactDetectsChangedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service")
	if err := os.WriteFile(path, []byte("first"), 0o700); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	digest, size, _, err := InspectArtifact(path)
	if err != nil {
		t.Fatalf("inspect artifact: %v", err)
	}
	record := ArtifactRecord{ID: "service", Path: path, SHA256: digest, SizeBytes: size}
	if err := ValidateArtifact(record); err != nil {
		t.Fatalf("validate artifact: %v", err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o700); err != nil {
		t.Fatalf("replace artifact: %v", err)
	}
	if err := ValidateArtifact(record); err == nil {
		t.Fatal("changed artifact unexpectedly validated")
	}
}

func TestValidateArtifactRejectsNonExecutableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	digest, size, _, err := InspectArtifact(path)
	if err != nil {
		t.Fatalf("inspect artifact: %v", err)
	}
	if err := ValidateArtifact(ArtifactRecord{ID: "service", Path: path, SHA256: digest, SizeBytes: size}); err == nil {
		t.Fatal("non-executable artifact unexpectedly validated")
	}
}
