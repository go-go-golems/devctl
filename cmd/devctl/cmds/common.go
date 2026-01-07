package cmds

import (
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	RepoRoot string
	Config   string
	Strict   bool
	DryRun   bool
	Timeout  time.Duration
}

func AddRootFlags(root *cobra.Command) {
	addRootFlags(root)
}

func addRootFlags(root *cobra.Command) {
	root.PersistentFlags().String("repo-root", "", "Repository root (defaults to current directory)")
	root.PersistentFlags().String("config", "", "Path to config file (defaults to .devctl.yaml under repo-root)")
	root.PersistentFlags().Bool("strict", false, "Treat merge collisions as errors")
	root.PersistentFlags().Bool("dry-run", false, "Do not perform destructive side effects (best-effort)")
	root.PersistentFlags().Duration("timeout", 30*time.Second, "Default timeout for plugin operations")
}

func getRootOptions(cmd *cobra.Command) (rootOptions, error) {
	repoRoot, err := cmd.Root().PersistentFlags().GetString("repo-root")
	if err != nil {
		return rootOptions{}, err
	}
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return rootOptions{}, err
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return rootOptions{}, err
	}

	cfgPath, err := cmd.Root().PersistentFlags().GetString("config")
	if err != nil {
		return rootOptions{}, err
	}
	if cfgPath == "" {
		cfgPath = config.DefaultPath(repoRoot)
	} else if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(repoRoot, cfgPath)
	}

	strict, err := cmd.Root().PersistentFlags().GetBool("strict")
	if err != nil {
		return rootOptions{}, err
	}
	dryRun, err := cmd.Root().PersistentFlags().GetBool("dry-run")
	if err != nil {
		return rootOptions{}, err
	}
	timeout, err := cmd.Root().PersistentFlags().GetDuration("timeout")
	if err != nil {
		return rootOptions{}, err
	}
	if timeout <= 0 {
		return rootOptions{}, errors.New("timeout must be > 0")
	}

	return rootOptions{
		RepoRoot: repoRoot,
		Config:   cfgPath,
		Strict:   strict,
		DryRun:   dryRun,
		Timeout:  timeout,
	}, nil
}

func requestMetaFromRootOptions(opts rootOptions) (runtime.RequestMeta, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return runtime.RequestMeta{}, err
	}
	return runtime.RequestMeta{
		RepoRoot: opts.RepoRoot,
		Cwd:      cwd,
		DryRun:   opts.DryRun,
	}, nil
}
