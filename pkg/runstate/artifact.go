package runstate

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
)

func InspectArtifact(path string) (string, int64, os.FileMode, error) {
	file, err := os.Open(path) // #nosec G304 -- artifact paths come from validated repository plans.
	if err != nil {
		return "", 0, 0, errors.Wrap(err, "open artifact")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return "", 0, 0, errors.Wrap(err, "stat artifact")
	}
	if !info.Mode().IsRegular() {
		return "", 0, 0, errors.New("artifact is not a regular file")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", 0, 0, errors.Wrap(err, "hash artifact")
	}
	return hex.EncodeToString(hash.Sum(nil)), info.Size(), info.Mode(), nil
}

var artifactIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func ValidateArtifactRecord(record ArtifactRecord) error {
	if !artifactIDPattern.MatchString(record.ID) || !filepath.IsAbs(record.Path) {
		return errors.New("artifact identity is incomplete")
	}
	digest, err := hex.DecodeString(record.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return errors.New("artifact SHA-256 is invalid")
	}
	if record.SizeBytes < 0 {
		return errors.New("artifact size is invalid")
	}
	return nil
}

func ValidateArtifact(record ArtifactRecord) error {
	if err := ValidateArtifactRecord(record); err != nil {
		return err
	}
	digest, size, mode, err := InspectArtifact(record.Path)
	if err != nil {
		return err
	}
	if mode.Perm()&0o111 == 0 {
		return errors.New("artifact is not executable")
	}
	if digest != record.SHA256 || size != record.SizeBytes {
		return errors.New("artifact bytes do not match recorded identity")
	}
	return nil
}
