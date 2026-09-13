package plugincatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

const (
	SchemaVersion = 2
	cacheDirName  = "cache"
	cacheFileName = "plugin-catalog.json"
)

var (
	ErrCatalogMissing  = stderrors.New("plugin command catalog is missing")
	ErrCatalogStale    = stderrors.New("plugin command catalog is stale")
	ErrCatalogConflict = stderrors.New("plugin command catalog has conflicts")
)

var commandNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type CommandEntry struct {
	Name        string                `json:"name"`
	Help        string                `json:"help,omitempty"`
	ProviderID  string                `json:"provider_id"`
	PluginName  string                `json:"plugin_name,omitempty"`
	Args        []protocol.CommandArg `json:"args,omitempty"`
	Fingerprint string                `json:"fingerprint"`
}

type ProviderRecord struct {
	Source        string            `json:"source"`
	CatalogInputs map[string]string `json:"catalog_inputs,omitempty"`
}

type Catalog struct {
	Version           int                       `json:"version"`
	ConfigFingerprint string                    `json:"config_fingerprint"`
	Profile           string                    `json:"profile,omitempty"`
	GeneratedAt       time.Time                 `json:"generated_at"`
	Providers         map[string]ProviderRecord `json:"providers"`
	Commands          map[string]CommandEntry   `json:"commands"`
	Conflicts         map[string][]CommandEntry `json:"conflicts"`
}

type RefreshOptions struct {
	Factory  *runtime.Factory
	Reserved map[string]bool
}

func CachePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".devctl", cacheDirName, cacheFileName)
}

func Fingerprint(repo *repository.Repository) (string, error) {
	if repo == nil {
		return "", errors.New("fingerprint plugin catalog: nil repository")
	}
	type executableIdentity struct {
		Path    string      `json:"path"`
		Size    int64       `json:"size,omitempty"`
		Mode    os.FileMode `json:"mode,omitempty"`
		ModTime int64       `json:"mod_time,omitempty"`
		Missing bool        `json:"missing,omitempty"`
	}
	type pluginMaterial struct {
		Spec          runtime.PluginSpec     `json:"spec"`
		Commands      []protocol.CommandSpec `json:"commands,omitempty"`
		Executable    executableIdentity     `json:"executable"`
		CatalogInputs map[string]string      `json:"catalog_inputs,omitempty"`
	}
	configByID := configPluginsByID(repo.Config)
	material := struct {
		Schema   int                      `json:"schema"`
		Protocol protocol.ProtocolVersion `json:"protocol"`
		Profile  string                   `json:"profile"`
		Plugins  []pluginMaterial         `json:"plugins"`
	}{
		Schema: SchemaVersion, Protocol: protocol.ProtocolV2, Profile: repo.ProfileName,
		Plugins: make([]pluginMaterial, 0, len(repo.Specs)),
	}
	specs := append([]runtime.PluginSpec{}, repo.Specs...)
	sort.Slice(specs, func(left, right int) bool { return specs[left].ID < specs[right].ID })
	for _, spec := range specs {
		resolvedPath, lookupErr := exec.LookPath(spec.Path)
		if lookupErr != nil {
			resolvedPath = spec.Path
		}
		identity := executableIdentity{Path: resolvedPath}
		if info, statErr := os.Stat(resolvedPath); statErr == nil {
			identity.Size = info.Size()
			identity.Mode = info.Mode()
			identity.ModTime = info.ModTime().UnixNano()
		} else {
			identity.Missing = true
		}
		inputs, inputErr := fingerprintCatalogInputs(repo.Root, configByID[spec.ID].CatalogInputs)
		if inputErr != nil {
			return "", errors.Wrapf(inputErr, "fingerprint plugin %q catalog inputs", spec.ID)
		}
		material.Plugins = append(material.Plugins, pluginMaterial{
			Spec: spec, Commands: configByID[spec.ID].Commands, Executable: identity,
			CatalogInputs: inputs,
		})
	}
	data, err := json.Marshal(material)
	if err != nil {
		return "", errors.Wrap(err, "marshal plugin catalog fingerprint")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func Static(repo *repository.Repository, reserved map[string]bool) (*Catalog, error) {
	fingerprint, err := Fingerprint(repo)
	if err != nil {
		return nil, err
	}
	catalog := newCatalog(repo, fingerprint)
	configByID := configPluginsByID(repo.Config)
	for _, spec := range repo.Specs {
		pluginConfig := configByID[spec.ID]
		inputs, inputErr := fingerprintCatalogInputs(repo.Root, pluginConfig.CatalogInputs)
		if inputErr != nil {
			return nil, errors.Wrapf(inputErr, "fingerprint plugin %q catalog inputs", spec.ID)
		}
		source := "handshake"
		if len(pluginConfig.Commands) > 0 {
			source = "static"
		}
		catalog.Providers[spec.ID] = ProviderRecord{Source: source, CatalogInputs: inputs}
		for _, command := range pluginConfig.Commands {
			addCommand(catalog, CommandEntry{
				Name: command.Name, Help: command.Help, ProviderID: spec.ID,
				Args:        append([]protocol.CommandArg{}, command.ArgsSpec...),
				Fingerprint: fingerprint,
			}, reserved)
		}
	}
	finalizeConflicts(catalog)
	if err := Validate(catalog, fingerprint, reserved); err != nil &&
		!stderrors.Is(err, ErrCatalogConflict) {
		return nil, err
	}
	return catalog, nil
}

func Refresh(
	ctx context.Context,
	repo *repository.Repository,
	options RefreshOptions,
) (*Catalog, error) {
	if repo == nil {
		return nil, errors.New("refresh plugin catalog: nil repository")
	}
	fingerprint, err := Fingerprint(repo)
	if err != nil {
		return nil, err
	}
	factory := options.Factory
	if factory == nil {
		factory = runtime.NewFactory(runtime.FactoryOptions{
			HandshakeTimeout: 2 * time.Second,
			ShutdownTimeout:  2 * time.Second,
		})
	}
	catalog := newCatalog(repo, fingerprint)
	configByID := configPluginsByID(repo.Config)
	for _, spec := range repo.Specs {
		pluginConfig := configByID[spec.ID]
		staticCommands := pluginConfig.Commands
		inputs, inputErr := fingerprintCatalogInputs(repo.Root, pluginConfig.CatalogInputs)
		if inputErr != nil {
			return nil, errors.Wrapf(inputErr, "fingerprint plugin %q catalog inputs", spec.ID)
		}
		catalog.Providers[spec.ID] = ProviderRecord{Source: "handshake", CatalogInputs: inputs}
		if len(staticCommands) > 0 {
			catalog.Providers[spec.ID] = ProviderRecord{Source: "static", CatalogInputs: inputs}
			for _, command := range staticCommands {
				addCommand(catalog, CommandEntry{
					Name: command.Name, Help: command.Help, ProviderID: spec.ID,
					Args:        append([]protocol.CommandArg{}, command.ArgsSpec...),
					Fingerprint: fingerprint,
				}, options.Reserved)
			}
			continue
		}
		client, startErr := factory.Start(ctx, spec, runtime.StartOptions{Meta: repo.Request})
		if startErr != nil {
			return nil, errors.Wrapf(startErr, "refresh command catalog provider %q", spec.ID)
		}
		handshake := client.Handshake()
		closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		closeErr := client.Close(closeContext)
		cancel()
		if closeErr != nil {
			return nil, errors.Wrapf(closeErr, "close command catalog provider %q", spec.ID)
		}
		if !contains(handshake.Capabilities.Ops, "command.run") {
			continue
		}
		for _, command := range handshake.Capabilities.Commands {
			addCommand(catalog, CommandEntry{
				Name: command.Name, Help: command.Help, ProviderID: spec.ID,
				PluginName:  handshake.PluginName,
				Args:        append([]protocol.CommandArg{}, command.ArgsSpec...),
				Fingerprint: fingerprint,
			}, options.Reserved)
		}
	}
	finalizeConflicts(catalog)
	validationErr := Validate(catalog, fingerprint, options.Reserved)
	if validationErr != nil && !stderrors.Is(validationErr, ErrCatalogConflict) {
		return nil, validationErr
	}
	if err := os.MkdirAll(filepath.Dir(CachePath(repo.Root)), 0o700); err != nil {
		return nil, errors.Wrap(err, "create plugin catalog cache directory")
	}
	if err := runstate.WriteJSONAtomic(CachePath(repo.Root), catalog, 0o600); err != nil {
		return nil, errors.Wrap(err, "write plugin command catalog")
	}
	return catalog, validationErr
}

func Load(repo *repository.Repository, reserved map[string]bool) (*Catalog, error) {
	fingerprint, err := Fingerprint(repo)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(CachePath(repo.Root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCatalogMissing
		}
		return nil, errors.Wrap(err, "read plugin command catalog")
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, errors.Wrap(ErrCatalogStale, "parse plugin command catalog")
	}
	if err := Validate(&catalog, fingerprint, reserved); err != nil {
		if stderrors.Is(err, ErrCatalogConflict) {
			return &catalog, err
		}
		return nil, err
	}
	return &catalog, nil
}

func Validate(catalog *Catalog, expectedFingerprint string, reserved map[string]bool) error {
	if catalog == nil || catalog.Version != SchemaVersion {
		return errors.Wrap(ErrCatalogStale, "unsupported catalog schema")
	}
	if catalog.ConfigFingerprint != expectedFingerprint {
		return errors.Wrap(ErrCatalogStale, "configuration fingerprint changed")
	}
	for providerID, provider := range catalog.Providers {
		if providerID == "" || provider.Source != "static" && provider.Source != "handshake" {
			return errors.Wrap(ErrCatalogStale, "catalog provider provenance is invalid")
		}
		for path, digest := range provider.CatalogInputs {
			if path == "" || len(digest) != sha256.Size*2 {
				return errors.Wrap(ErrCatalogStale, "catalog input provenance is invalid")
			}
		}
	}
	for name, entry := range catalog.Commands {
		if name != entry.Name || entry.ProviderID == "" || entry.Fingerprint != expectedFingerprint {
			return errors.Wrap(ErrCatalogStale, "catalog command identity is invalid")
		}
		if err := ValidateCommandName(name, reserved); err != nil {
			return err
		}
	}
	for name, entries := range catalog.Conflicts {
		if len(entries) == 0 {
			return errors.Wrap(ErrCatalogStale, "catalog conflict is empty")
		}
		previousProvider := ""
		for _, entry := range entries {
			if name != entry.Name || entry.ProviderID == "" || entry.Fingerprint != expectedFingerprint {
				return errors.Wrap(ErrCatalogStale, "catalog conflict identity is invalid")
			}
			if !commandNamePattern.MatchString(name) || len(name) > 64 {
				return errors.Wrap(ErrCatalogStale, "catalog conflict name is invalid")
			}
			if entry.ProviderID <= previousProvider {
				return errors.Wrap(ErrCatalogStale, "catalog conflict providers are not unique and sorted")
			}
			previousProvider = entry.ProviderID
		}
	}
	if len(catalog.Conflicts) > 0 {
		return errors.Wrap(ErrCatalogConflict, "ambiguous plugin command names")
	}
	return nil
}

func ValidateCommandName(name string, reserved map[string]bool) error {
	if !commandNamePattern.MatchString(name) || len(name) > 64 {
		return errors.Errorf("invalid plugin command name %q", name)
	}
	if reserved[name] {
		return errors.Errorf("plugin command %q collides with a reserved command", name)
	}
	return nil
}

func newCatalog(repo *repository.Repository, fingerprint string) *Catalog {
	return &Catalog{
		Version: SchemaVersion, ConfigFingerprint: fingerprint,
		Profile: repo.ProfileName, GeneratedAt: time.Now().UTC(),
		Providers: map[string]ProviderRecord{}, Commands: map[string]CommandEntry{}, Conflicts: map[string][]CommandEntry{},
	}
}

func addCommand(catalog *Catalog, entry CommandEntry, reserved map[string]bool) {
	if err := ValidateCommandName(entry.Name, reserved); err != nil {
		catalog.Conflicts[entry.Name] = append(catalog.Conflicts[entry.Name], entry)
		return
	}
	if existing, exists := catalog.Commands[entry.Name]; exists {
		delete(catalog.Commands, entry.Name)
		catalog.Conflicts[entry.Name] = append(catalog.Conflicts[entry.Name], existing, entry)
		return
	}
	if conflicts := catalog.Conflicts[entry.Name]; len(conflicts) > 0 {
		catalog.Conflicts[entry.Name] = append(conflicts, entry)
		return
	}
	catalog.Commands[entry.Name] = entry
}

func finalizeConflicts(catalog *Catalog) {
	for name := range catalog.Conflicts {
		sort.Slice(catalog.Conflicts[name], func(left, right int) bool {
			return catalog.Conflicts[name][left].ProviderID < catalog.Conflicts[name][right].ProviderID
		})
	}
}

func configPluginsByID(file *config.File) map[string]config.Plugin {
	result := map[string]config.Plugin{}
	if file == nil {
		return result
	}
	for _, plugin := range file.Plugins {
		result[plugin.ID] = plugin
	}
	return result
}

func fingerprintCatalogInputs(repoRoot string, inputs []string) (map[string]string, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	root, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return nil, errors.Wrap(err, "resolve repository root")
	}
	ordered := append([]string{}, inputs...)
	sort.Strings(ordered)
	fingerprints := make(map[string]string, len(ordered))
	for _, input := range ordered {
		clean := filepath.Clean(input)
		if input == "" || filepath.IsAbs(input) || clean == "." || clean == ".." || len(clean) >= 3 && clean[:3] == ".."+string(filepath.Separator) {
			return nil, errors.Errorf("catalog input %q must be a repository-relative file", input)
		}
		if _, exists := fingerprints[filepath.ToSlash(clean)]; exists {
			return nil, errors.Errorf("catalog input %q is duplicated", input)
		}
		path := filepath.Join(root, clean)
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return nil, errors.Wrapf(resolveErr, "resolve catalog input %q", input)
		}
		relative, relativeErr := filepath.Rel(root, resolved)
		if relativeErr != nil || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
			return nil, errors.Errorf("catalog input %q resolves outside the repository", input)
		}
		info, statErr := os.Stat(resolved)
		if statErr != nil {
			return nil, errors.Wrapf(statErr, "stat catalog input %q", input)
		}
		if !info.Mode().IsRegular() {
			return nil, errors.Errorf("catalog input %q is not a regular file", input)
		}
		data, readErr := os.ReadFile(resolved)
		if readErr != nil {
			return nil, errors.Wrapf(readErr, "read catalog input %q", input)
		}
		sum := sha256.Sum256(data)
		fingerprints[filepath.ToSlash(clean)] = hex.EncodeToString(sum[:])
	}
	return fingerprints, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
