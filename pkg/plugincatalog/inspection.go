package plugincatalog

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"time"

	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/pkg/errors"
)

type CatalogState string

const (
	CatalogMissing    CatalogState = "missing"
	CatalogStale      CatalogState = "stale"
	CatalogValid      CatalogState = "valid"
	CatalogConflicted CatalogState = "conflicted"
)

type Inspection struct {
	State               CatalogState `json:"state"`
	ExpectedFingerprint string       `json:"expected_fingerprint"`
	StoredFingerprint   string       `json:"stored_fingerprint,omitempty"`
	GeneratedAt         time.Time    `json:"generated_at,omitempty"`
	Action              string       `json:"action"`
	Catalog             *Catalog     `json:"-"`
}

// Inspect reads catalog state without starting plugin processes.
func Inspect(repo *repository.Repository, reserved map[string]bool) (Inspection, error) {
	expected, err := Fingerprint(repo)
	if err != nil {
		return Inspection{}, err
	}
	result := Inspection{ExpectedFingerprint: expected}
	data, err := os.ReadFile(CachePath(repo.Root))
	if err != nil {
		if os.IsNotExist(err) {
			result.State = CatalogMissing
			result.Action = "run devctl plugins refresh for handshake providers"
			return result, nil
		}
		return Inspection{}, errors.Wrap(err, "read plugin command catalog")
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		result.State = CatalogStale
		result.Action = "run devctl plugins refresh"
		return result, nil
	}
	result.Catalog = &catalog
	result.StoredFingerprint = catalog.ConfigFingerprint
	result.GeneratedAt = catalog.GeneratedAt
	validationErr := Validate(&catalog, expected, reserved)
	switch {
	case validationErr == nil:
		result.State = CatalogValid
		result.Action = "none"
	case stderrors.Is(validationErr, ErrCatalogConflict):
		result.State = CatalogConflicted
		result.Action = "resolve command conflicts and refresh"
	case stderrors.Is(validationErr, ErrCatalogStale):
		result.State = CatalogStale
		result.Action = "run devctl plugins refresh"
	default:
		return Inspection{}, validationErr
	}
	return result, nil
}
