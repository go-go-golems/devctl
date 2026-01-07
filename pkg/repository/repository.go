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
	RepoRoot   string
	ConfigPath string
	Cwd        string
	DryRun     bool
}

type Repository struct {
	Root      string
	Config    *config.File
	Specs     []runtime.PluginSpec
	SpecByID  map[string]runtime.PluginSpec
	Request   runtime.RequestMeta
	ConfigAbs string
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

	cfg, err := config.LoadOptional(cfgPath)
	if err != nil {
		return nil, err
	}
	specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: root})
	if err != nil {
		return nil, err
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
		Root:      root,
		Config:    cfg,
		Specs:     specs,
		SpecByID:  specByID,
		Request:   runtime.RequestMeta{RepoRoot: root, Cwd: cwd, DryRun: opts.DryRun},
		ConfigAbs: cfgPath,
	}, nil
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
