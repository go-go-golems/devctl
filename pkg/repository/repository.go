package repository

import (
	"context"
	"path/filepath"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

type Options struct {
	RepoRoot     string
	ConfigPath   string
	OverridePath string
	Cwd          string
	DryRun       bool
	ProfileName  string
}

type Repository struct {
	Root        string
	Config      *config.File
	Specs       []runtime.PluginSpec
	SpecByID    map[string]runtime.PluginSpec
	Request     runtime.RequestMeta
	ConfigAbs   string
	OverrideAbs string
	ProfileName string
	Profile     *config.Profile
}

func Load(opts Options) (*Repository, error) {
	if opts.RepoRoot == "" {
		return nil, errors.New("missing RepoRoot")
	}
	root, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return nil, err
	}
	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath(root)
	} else if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(root, cfgPath)
	}
	overridePath := opts.OverridePath
	if overridePath == "" {
		overridePath = config.DefaultOverridePath(root)
	} else if !filepath.IsAbs(overridePath) {
		overridePath = filepath.Join(root, overridePath)
	}

	cfg, err := config.LoadStacked(cfgPath, overridePath)
	if err != nil {
		return nil, err
	}
	specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: root})
	if err != nil {
		return nil, err
	}

	profileName := cfg.ResolveProfile(opts.ProfileName)
	var profile *config.Profile
	if profileName != "" {
		profile = cfg.GetProfile(profileName)
		if profile == nil {
			return nil, errors.Errorf("profile %q not found", profileName)
		}
		specs, err = filterSpecs(specs, profileName, profile)
		if err != nil {
			return nil, err
		}
	}

	specByID := make(map[string]runtime.PluginSpec, len(specs))
	for _, spec := range specs {
		if _, ok := specByID[spec.ID]; ok {
			continue
		}
		specByID[spec.ID] = spec
	}

	cwd := opts.Cwd
	if cwd == "" {
		cwd = root
	}

	return &Repository{
		Root:        root,
		Config:      cfg,
		Specs:       specs,
		SpecByID:    specByID,
		Request:     runtime.RequestMeta{RepoRoot: root, Cwd: cwd, DryRun: opts.DryRun},
		ConfigAbs:   cfgPath,
		OverrideAbs: overridePath,
		ProfileName: profileName,
		Profile:     profile,
	}, nil
}

func filterSpecs(specs []runtime.PluginSpec, profileName string, profile *config.Profile) ([]runtime.PluginSpec, error) {
	allowed := make(map[string]bool, len(profile.Plugins))
	for _, id := range profile.Plugins {
		allowed[id] = true
	}

	filtered := make([]runtime.PluginSpec, 0, len(profile.Plugins))
	seen := make(map[string]bool, len(profile.Plugins))
	for _, spec := range specs {
		if !allowed[spec.ID] {
			continue
		}
		merged := spec
		merged.Env = mergeEnvMaps(spec.Env, profile.Env)
		filtered = append(filtered, merged)
		seen[spec.ID] = true
	}

	for _, id := range profile.Plugins {
		if !seen[id] {
			return nil, errors.Errorf("profile %q references unknown plugin %q", profileName, id)
		}
	}
	return filtered, nil
}

func mergeEnvMaps(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func (r *Repository) StartClients(ctx context.Context, factory *runtime.Factory) ([]runtime.Client, error) {
	clients := make([]runtime.Client, 0, len(r.Specs))
	for _, spec := range r.Specs {
		c, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: r.Request})
		if err != nil {
			_ = CloseClients(context.Background(), clients)
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, nil
}

func CloseClients(ctx context.Context, clients []runtime.Client) error {
	var firstErr error
	for _, c := range clients {
		if err := c.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
