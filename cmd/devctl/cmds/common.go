package cmds

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const repoLayerSlug = "repo"

type RepoSettings struct {
	RepoRoot string `glazed:"repo-root"`
	Config   string `glazed:"config"`
	Profile  string `glazed:"profile"`
	Strict   bool   `glazed:"strict"`
	DryRun   bool   `glazed:"dry-run"`
	Timeout  string `glazed:"timeout"` // duration string, e.g. "30s"
}

type RepoContext struct {
	RepoRoot   string
	ConfigPath string
	Cwd        string
	Profile    string
	Strict     bool
	DryRun     bool
	Timeout    time.Duration
}

func (rc RepoContext) RequestMeta() runtime.RequestMeta {
	return runtime.RequestMeta{
		RepoRoot: rc.RepoRoot,
		Cwd:      rc.Cwd,
		DryRun:   rc.DryRun,
	}
}

func repoContextFromSettings(settings RepoSettings, cwd string) (RepoContext, error) {
	repoRoot := settings.RepoRoot
	if repoRoot == "" {
		repoRoot = cwd
	}
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return RepoContext{}, err
	}

	cfgPath := settings.Config
	if cfgPath == "" {
		cfgPath = config.DefaultPath(repoRoot)
	} else if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(repoRoot, cfgPath)
	}

	timeoutStr := settings.Timeout
	if timeoutStr == "" {
		timeoutStr = "30s"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return RepoContext{}, errors.Wrap(err, "parse --timeout (expected duration like 30s)")
	}
	if timeout <= 0 {
		return RepoContext{}, errors.New("timeout must be > 0")
	}

	return RepoContext{
		RepoRoot:   repoRoot,
		ConfigPath: cfgPath,
		Cwd:        cwd,
		Profile:    settings.Profile,
		Strict:     settings.Strict,
		DryRun:     settings.DryRun,
		Timeout:    timeout,
	}, nil
}

func RepoContextFromParsedLayers(vals *values.Values) (RepoContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return RepoContext{}, err
	}
	settings := RepoSettings{}
	if err := vals.DecodeSectionInto(repoLayerSlug, &settings); err != nil {
		return RepoContext{}, err
	}
	return repoContextFromSettings(settings, cwd)
}

func RepoContextFromCobra(cmd *cobra.Command) (RepoContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return RepoContext{}, err
	}

	repoRoot, err := cmd.Flags().GetString("repo-root")
	if err != nil {
		return RepoContext{}, err
	}
	cfgPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return RepoContext{}, err
	}
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return RepoContext{}, err
	}
	profile, err := cmd.Flags().GetString("profile")
	if err != nil {
		return RepoContext{}, err
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return RepoContext{}, err
	}
	timeoutStr, err := cmd.Flags().GetString("timeout")
	if err != nil {
		return RepoContext{}, err
	}

	return repoContextFromSettings(RepoSettings{
		RepoRoot: repoRoot,
		Config:   cfgPath,
		Profile:  profile,
		Strict:   strict,
		DryRun:   dryRun,
		Timeout:  timeoutStr,
	}, cwd)
}

var (
	repoLayerOnce sync.Once
	repoLayerInst *schema.SectionImpl
	repoLayerErr  error
)

func getRepoLayer() (*schema.SectionImpl, error) {
	repoLayerOnce.Do(func() {
		section, err := schema.NewSection(repoLayerSlug, "Repository",
			schema.WithDescription("Repository context shared by most devctl commands"),
			schema.WithFields(
				fields.New(
					"repo-root",
					fields.TypeString,
					fields.WithDefault(""),
					fields.WithHelp("Repository root (defaults to current directory)"),
				),
				fields.New(
					"config",
					fields.TypeString,
					fields.WithDefault(""),
					fields.WithHelp("Path to config file (defaults to .devctl.yaml under repo-root)"),
				),
				fields.New(
					"profile",
					fields.TypeString,
					fields.WithDefault(""),
					fields.WithHelp("Active profile name (overrides profile.active in config)"),
				),
				fields.New(
					"strict",
					fields.TypeBool,
					fields.WithDefault(false),
					fields.WithHelp("Treat merge collisions as errors"),
				),
				fields.New(
					"dry-run",
					fields.TypeBool,
					fields.WithDefault(false),
					fields.WithHelp("Do not perform destructive side effects (best-effort)"),
				),
				fields.New(
					"timeout",
					fields.TypeString,
					fields.WithDefault("30s"),
					fields.WithHelp("Default timeout for plugin operations (duration like 30s)"),
				),
			),
		)
		if err != nil {
			repoLayerErr = err
			return
		}

		repoLayerInst = section
	})
	return repoLayerInst, repoLayerErr
}

func AddRepoFlags(cmd *cobra.Command) {
	section, err := getRepoLayer()
	cobra.CheckErr(err)
	cobra.CheckErr(section.AddSectionToCobraCommand(cmd))
}

type rootOptions struct {
	RepoRoot string
	Config   string
	Profile  string
	Strict   bool
	DryRun   bool
	Timeout  time.Duration
}

func getRootOptions(cmd *cobra.Command) (rootOptions, error) {
	rc, err := RepoContextFromCobra(cmd)
	if err != nil {
		return rootOptions{}, err
	}
	return rootOptions{
		RepoRoot: rc.RepoRoot,
		Config:   rc.ConfigPath,
		Profile:  rc.Profile,
		Strict:   rc.Strict,
		DryRun:   rc.DryRun,
		Timeout:  rc.Timeout,
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
