package operator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

const LifecycleRecipeSchemaVersion = 1

type RecipePhase struct {
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	Steps      []string `json:"steps,omitempty"`
	Unresolved []string `json:"unresolved,omitempty"`
}

type LifecycleRecipe struct {
	Version               int            `json:"version"`
	ID                    string         `json:"id"`
	Operation             string         `json:"operation"`
	RepoRoot              string         `json:"repo_root"`
	RepositoryFingerprint string         `json:"repository_fingerprint"`
	ProfileName           string         `json:"profile,omitempty"`
	Selection             Selection      `json:"selection"`
	Phases                []RecipePhase  `json:"phases"`
	Policy                PipelinePolicy `json:"-"`
	ResolvedAt            time.Time      `json:"resolved_at"`
}

// PlanResult is retained as a compact fixture/input type for callers that
// already have fully prepared launch facts.
type PlanResult struct {
	Plan        engine.LaunchPlan
	ProfileName string
}

type PreparedLaunch struct {
	Version               int                   `json:"version"`
	Recipe                LifecycleRecipe       `json:"recipe"`
	RepositoryFingerprint string                `json:"repository_fingerprint"`
	Plan                  engine.LaunchPlan     `json:"plan"`
	Build                 *engine.BuildResult   `json:"build,omitempty"`
	Prepare               *engine.PrepareResult `json:"prepare,omitempty"`
	PreparedAt            time.Time             `json:"prepared_at"`
}

type Planner interface {
	ResolveRecipe(context.Context, string, UpRequest) (LifecycleRecipe, error)
	PrepareReplacement(context.Context, LifecycleRecipe) (PreparedLaunch, error)
	ValidatePrepared(context.Context, PreparedLaunch) error
}

type PipelinePlanner struct{}

var _ Planner = PipelinePlanner{}

func (PipelinePlanner) ResolveRecipe(
	ctx context.Context,
	operation string,
	request UpRequest,
) (LifecycleRecipe, error) {
	if err := ctx.Err(); err != nil {
		return LifecycleRecipe{}, err
	}
	repo, err := loadPlanningRepository(request)
	if err != nil {
		return LifecycleRecipe{}, errors.Wrap(err, "load repository for lifecycle recipe")
	}
	if len(repo.Specs) == 0 {
		return LifecycleRecipe{}, errors.New("lifecycle recipe: no plugins configured")
	}
	fingerprint, err := lifecycleRepositoryFingerprint(repo)
	if err != nil {
		return LifecycleRecipe{}, err
	}
	recipeID, err := runstate.NewRunID()
	if err != nil {
		return LifecycleRecipe{}, errors.Wrap(err, "generate lifecycle recipe ID")
	}
	request.RepoRoot = repo.Root
	if request.Policy.Cwd == "" {
		request.Policy.Cwd = repo.Root
	}
	return LifecycleRecipe{
		Version:               LifecycleRecipeSchemaVersion,
		ID:                    recipeID,
		Operation:             operation,
		RepoRoot:              repo.Root,
		RepositoryFingerprint: fingerprint,
		ProfileName:           repo.ProfileName,
		Selection:             request.Select,
		Policy:                request.Policy,
		Phases:                recipePhases(request.Policy),
		ResolvedAt:            time.Now().UTC(),
	}, nil
}

func (PipelinePlanner) PrepareReplacement(
	ctx context.Context,
	recipe LifecycleRecipe,
) (PreparedLaunch, error) {
	if recipe.Version != LifecycleRecipeSchemaVersion {
		return PreparedLaunch{}, errors.Errorf("unsupported lifecycle recipe schema %d", recipe.Version)
	}
	request := UpRequest{
		RepoRoot: recipe.RepoRoot,
		Profile:  recipe.ProfileName,
		Select:   recipe.Selection,
		Policy:   recipe.Policy,
	}
	repo, err := loadPlanningRepository(request)
	if err != nil {
		return PreparedLaunch{}, errors.Wrap(err, "reload repository for replacement preparation")
	}
	fingerprint, err := lifecycleRepositoryFingerprint(repo)
	if err != nil {
		return PreparedLaunch{}, err
	}
	if fingerprint != recipe.RepositoryFingerprint {
		return PreparedLaunch{}, staleRecipeError(recipe.ID)
	}

	timeout := normalizedTimeout(recipe.Policy.Timeout)
	strict := recipe.Policy.Strict || repo.Config.Strictness == "error"
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})
	clients, err := repo.StartClients(ctx, factory)
	if err != nil {
		return PreparedLaunch{}, errors.Wrap(err, "start replacement preparation plugins")
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = repository.CloseClients(closeContext, clients)
	}()

	pipeline := &engine.Pipeline{
		Clients: clients,
		Opts:    engine.Options{Strict: strict, DryRun: recipe.Policy.DryRun},
	}
	phaseContext, cancel := context.WithTimeout(ctx, timeout)
	configuration, err := pipeline.MutateConfig(phaseContext, patch.Config{})
	cancel()
	if err != nil {
		return PreparedLaunch{}, errors.Wrap(err, "mutate replacement configuration")
	}

	prepared := PreparedLaunch{Version: LifecycleRecipeSchemaVersion, Recipe: recipe}
	if !recipe.Policy.SkipBuild {
		phaseContext, cancel = context.WithTimeout(ctx, timeout)
		result, buildErr := pipeline.Build(phaseContext, configuration, recipe.Policy.BuildSteps)
		cancel()
		if buildErr != nil {
			return PreparedLaunch{}, errors.Wrap(buildErr, "run replacement build phase")
		}
		prepared.Build = &result
	}
	if !recipe.Policy.SkipPrepare {
		phaseContext, cancel = context.WithTimeout(ctx, timeout)
		result, prepareErr := pipeline.Prepare(phaseContext, configuration, recipe.Policy.PrepareSteps)
		cancel()
		if prepareErr != nil {
			return PreparedLaunch{}, errors.Wrap(prepareErr, "run replacement prepare phase")
		}
		prepared.Prepare = &result
	}
	if !recipe.Policy.SkipValidate {
		phaseContext, cancel = context.WithTimeout(ctx, timeout)
		validation, validationErr := pipeline.Validate(phaseContext, configuration)
		cancel()
		if validationErr != nil {
			return PreparedLaunch{}, errors.Wrap(validationErr, "run replacement validation phase")
		}
		if !validation.Valid {
			return PreparedLaunch{}, errors.Errorf("replacement validation failed with %d error(s)", len(validation.Errors))
		}
	}
	phaseContext, cancel = context.WithTimeout(ctx, timeout)
	prepared.Plan, err = pipeline.LaunchPlan(phaseContext, configuration)
	cancel()
	if err != nil {
		return PreparedLaunch{}, errors.Wrap(err, "resolve prepared launch plan")
	}
	prepared.RepositoryFingerprint = fingerprint
	prepared.PreparedAt = time.Now().UTC()
	return prepared, nil
}

func (PipelinePlanner) ValidatePrepared(ctx context.Context, prepared PreparedLaunch) error {
	if prepared.Version != LifecycleRecipeSchemaVersion || prepared.Recipe.Version != LifecycleRecipeSchemaVersion {
		return errors.New("unsupported prepared lifecycle schema")
	}
	request := UpRequest{
		RepoRoot: prepared.Recipe.RepoRoot,
		Profile:  prepared.Recipe.ProfileName,
		Policy:   prepared.Recipe.Policy,
	}
	repo, err := loadPlanningRepository(request)
	if err != nil {
		return errors.Wrap(err, "reload repository before lifecycle apply")
	}
	fingerprint, err := lifecycleRepositoryFingerprint(repo)
	if err != nil {
		return err
	}
	if fingerprint != prepared.RepositoryFingerprint || fingerprint != prepared.Recipe.RepositoryFingerprint {
		return staleRecipeError(prepared.Recipe.ID)
	}
	return ctx.Err()
}

func loadPlanningRepository(request UpRequest) (*repository.Repository, error) {
	cwd := request.Policy.Cwd
	if cwd == "" {
		cwd = request.RepoRoot
	}
	return repository.Load(repository.Options{
		RepoRoot: request.RepoRoot, ConfigPath: request.Policy.ConfigPath,
		ProfileName: request.Profile, Cwd: cwd, DryRun: request.Policy.DryRun,
	})
}

func lifecycleRepositoryFingerprint(repo *repository.Repository) (string, error) {
	material := struct {
		Schema  int    `json:"schema"`
		Profile string `json:"profile"`
		Config  any    `json:"config"`
	}{Schema: LifecycleRecipeSchemaVersion, Profile: repo.ProfileName, Config: repo.Config}
	data, err := json.Marshal(material)
	if err != nil {
		return "", errors.Wrap(err, "encode lifecycle repository fingerprint")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func recipePhases(policy PipelinePolicy) []RecipePhase {
	return []RecipePhase{
		{Name: "config.mutate", Enabled: true},
		{Name: "build.run", Enabled: !policy.SkipBuild, Steps: append([]string{}, policy.BuildSteps...)},
		{Name: "prepare.run", Enabled: !policy.SkipPrepare, Steps: append([]string{}, policy.PrepareSteps...)},
		{Name: "validate.run", Enabled: !policy.SkipValidate},
		{Name: "launch.plan", Enabled: true, Unresolved: []string{"services", "commands", "health"}},
	}
}

func staleRecipeError(recipeID string) error {
	return &OperatorError{
		Code:    CodeRecipeStale,
		Message: "lifecycle recipe is stale; resolve and prepare again",
		Details: map[string]any{"recipe_id": recipeID},
	}
}
