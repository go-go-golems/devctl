package operator

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/pkg/errors"
)

func (c *controller) newOperation(kind string) (OperationResult, error) {
	operationID, err := c.newOperationID()
	if err != nil {
		return OperationResult{}, errors.Wrap(err, "generate operator operation ID")
	}
	return OperationResult{
		OperationID: operationID,
		Kind:        kind,
		StartedAt:   c.now(),
		Outcomes:    []ServiceOutcome{},
	}, nil
}

func (c *controller) finishFailed(
	result OperationResult,
	code string,
	message string,
	cause error,
) (OperationResult, error) {
	operatorErr := newError(code, message, cause)
	result.Status = "failed"
	result.FinishedAt = c.now()
	return result, operatorErr
}

func (c *controller) finishWithOperatorError(
	result OperationResult,
	operatorErr *OperatorError,
) (OperationResult, error) {
	result.Status = "failed"
	result.FinishedAt = c.now()
	return result, operatorErr
}

func (c *controller) finishCanceled(result OperationResult, cause error) (OperationResult, error) {
	result.Status = "canceled"
	result.FinishedAt = c.now()
	return result, newError(CodeCanceled, "operation canceled", cause)
}

func (c *controller) finishFromOutcomes(
	ctx context.Context,
	sink EventSink,
	result OperationResult,
) (OperationResult, error) {
	failures := 0
	for _, outcome := range result.Outcomes {
		if outcome.Error != nil {
			failures++
		}
	}
	result.FinishedAt = c.now()
	switch {
	case failures == 0:
		result.Status = "succeeded"
	case failures == len(result.Outcomes):
		result.Status = "failed"
	default:
		result.Status = "partial"
	}
	c.emit(ctx, sink, result.OperationID, EventOperationFinished, "", result.Status, "operation finished", nil)
	if failures > 0 {
		return result, newError(CodePartialFailure, "one or more service operations failed", nil)
	}
	return result, nil
}

func (c *controller) emit(
	ctx context.Context,
	sink EventSink,
	operationID string,
	kind EventKind,
	service string,
	status string,
	message string,
	operatorErr *OperatorError,
) {
	_ = sink.Send(ctx, OperatorEvent{
		Version:     EventVersion,
		OperationID: operationID,
		At:          c.now(),
		Kind:        kind,
		Service:     service,
		Status:      status,
		Message:     message,
		Error:       operatorErr,
	})
}

func normalizeSink(sink EventSink) EventSink {
	if sink == nil {
		return NopEventSink{}
	}
	return sink
}

func serviceError(code, message, service, runID string, cause error) *OperatorError {
	operatorErr := newError(code, message, cause)
	operatorErr.Service = service
	operatorErr.RunID = runID
	return operatorErr
}

func asOperatorError(err error) *OperatorError {
	var operatorErr *OperatorError
	if stderrors.As(err, &operatorErr) {
		return operatorErr
	}
	return nil
}

func loadEnvironmentOptional(ctx context.Context, store *runstate.Store) (*runstate.EnvironmentState, error) {
	environment, err := store.LoadEnvironment(ctx)
	if err == nil {
		return environment, nil
	}
	if os.IsNotExist(errors.Cause(err)) {
		return nil, nil
	}
	return nil, err
}

func selectPlannedServices(
	plan []engine.ServiceSpec,
	selection Selection,
) ([]engine.ServiceSpec, *OperatorError) {
	byName := make(map[string]engine.ServiceSpec, len(plan))
	for _, service := range plan {
		if service.Name == "" {
			return nil, newError(CodeConfigInvalid, "launch plan contains an unnamed service", nil)
		}
		if _, exists := byName[service.Name]; exists {
			return nil, serviceError(CodeConfigInvalid, "launch plan contains duplicate service", service.Name, "", nil)
		}
		byName[service.Name] = service
	}

	names := selection.Services
	if len(names) == 0 {
		names = make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
	} else {
		names = append([]string{}, names...)
	}
	sort.Strings(names)
	selected := make([]engine.ServiceSpec, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		service, exists := byName[name]
		if !exists {
			return nil, serviceError(CodeServiceUnknown, "service is not present in launch plan", name, "", nil)
		}
		selected = append(selected, service)
	}
	return selected, nil
}

func selectStateServices(
	environment *runstate.EnvironmentState,
	selection Selection,
) ([]string, *OperatorError) {
	if environment == nil {
		return []string{}, nil
	}
	names := selection.Services
	if len(names) == 0 {
		names = sortedSlotNames(environment)
	} else {
		names = append([]string{}, names...)
		sort.Strings(names)
	}
	selected := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		if _, exists := environment.Services[name]; !exists {
			return nil, serviceError(CodeServiceUnknown, "service is not present in environment state", name, "", nil)
		}
		selected = append(selected, name)
	}
	return selected, nil
}

func sortedSlotNames(environment *runstate.EnvironmentState) []string {
	names := make([]string, 0, len(environment.Services))
	for name := range environment.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func rejectRunningSelections(
	ctx context.Context,
	store *runstate.Store,
	environment *runstate.EnvironmentState,
	services []engine.ServiceSpec,
) *OperatorError {
	if environment == nil {
		return nil
	}
	for _, service := range services {
		slot, exists := environment.Services[service.Name]
		if !exists || slot.CurrentRunID == "" {
			continue
		}
		run, err := store.LoadRun(ctx, slot.CurrentRunID)
		if err != nil {
			return serviceError(CodeStateCorrupt, "load current service run", service.Name, slot.CurrentRunID, err)
		}
		for _, identity := range []*runstate.ProcessIdentity{run.Wrapper, run.Child} {
			if identity == nil {
				continue
			}
			status, inspectErr := runstate.InspectProcess(identity)
			if inspectErr != nil {
				return serviceError(CodeProcessIdentityUnsupported, "inspect current service process", service.Name, slot.CurrentRunID, inspectErr)
			}
			if status == runstate.ProcessMismatch {
				return serviceError(CodeProcessIdentityMismatch, "current service PID belongs to another process", service.Name, slot.CurrentRunID, nil)
			}
			if status == runstate.ProcessMatches {
				return serviceError(CodeServiceAlreadyRunning, "service already has a matching live run", service.Name, slot.CurrentRunID, nil)
			}
		}
	}
	return nil
}

func resolveServiceCwd(repoRoot string, configured string) string {
	if configured == "" {
		return repoRoot
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	return filepath.Join(repoRoot, configured)
}

func healthRecord(health *engine.HealthCheck) *runstate.HealthCheckRecord {
	if health == nil {
		return nil
	}
	return &runstate.HealthCheckRecord{
		Type:      health.Type,
		Address:   health.Address,
		URL:       health.URL,
		TimeoutMs: health.TimeoutMs,
	}
}

func normalizedTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return 30 * time.Second
	}
	return timeout
}
