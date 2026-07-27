package operator

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

func (c *controller) doctor(ctx context.Context, request DoctorRequest) (DoctorReport, error) {
	report := DoctorReport{
		Checks: []DoctorCheck{},
		Reconciliation: ReconciliationReport{
			Actions: []ReconciliationAction{}, UnindexedRuns: []string{},
		},
	}
	store, err := runstate.NewStore(request.RepoRoot)
	if err != nil {
		return report, newError(CodeUsage, "invalid repository root", err)
	}
	repo, configErr := repository.Load(repository.Options{
		RepoRoot: store.RepoRoot(), ConfigPath: request.ConfigPath,
		ProfileName: request.Profile, Cwd: request.Cwd,
	})
	if configErr != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Check: "configuration", Scope: "repository", Status: "error",
			Code: CodeConfigInvalid, Summary: configErr.Error(), Path: request.ConfigPath,
			Remediation: "fix the configuration or selected profile before lifecycle operations",
		})
	} else {
		report.Checks = append(report.Checks, DoctorCheck{
			Check: "configuration", Scope: "repository", Status: "ok",
			Summary: "configuration and selected profile are valid", Path: repo.ConfigAbs,
		})
		if request.Plugins {
			report.Checks = append(report.Checks, inspectDoctorPlugins(ctx, repo, request.Timeout)...)
		}
	}

	environment, stateErr := loadEnvironmentOptional(ctx, store)
	if stateErr != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Check: "state", Scope: "environment", Status: "error",
			Code: CodeStateCorrupt, Summary: stateErr.Error(), Path: store.StatePath(),
			Remediation: "inspect and restore the versioned state document; doctor will not rewrite it",
		})
		return report, nil
	}
	if environment == nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Check: "state", Scope: "environment", Status: "ok",
			Summary: "no environment state exists", Path: store.StatePath(),
		})
		addDoctorDiskCheck(&report, store, nil)
		return report, nil
	}
	report.Checks = append(report.Checks, DoctorCheck{
		Check: "state", Scope: "environment", Status: "ok",
		Summary: "environment state schema and index are valid", Path: store.StatePath(),
	})

	indexed := map[string]bool{}
	for _, slot := range environment.Services {
		if slot.CurrentRunID != "" {
			indexed[slot.CurrentRunID] = true
		}
		if slot.LastRunID != "" {
			indexed[slot.LastRunID] = true
		}
	}
	reader, readerErr := runlog.NewFileReader(store.RepoRoot())
	if readerErr != nil {
		return report, readerErr
	}
	for _, name := range sortedSlotNames(environment) {
		slot := environment.Services[name]
		runID := slot.CurrentRunID
		if runID == "" {
			runID = slot.LastRunID
		}
		if runID == "" {
			continue
		}
		run, loadErr := store.LoadRun(ctx, runID)
		if loadErr != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Check: "run", Scope: "service", Status: "error",
				Code: CodeStateCorrupt, Summary: loadErr.Error(),
				Service: name, RunID: runID,
			})
			continue
		}
		report.Checks = append(report.Checks, inspectDoctorRun(name, slot, run)...)
		_, logErr := reader.Query(ctx, runlog.Query{RunIDs: []string{runID}})
		if logErr == nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Check: "log-journal", Scope: "run", Status: "ok",
				Summary: "structured log journal is continuous", Service: name, RunID: runID,
			})
		} else {
			var readErr *runlog.ReadError
			status := "error"
			if stderrors.As(logErr, &readErr) && readErr.Code == runlog.CodeLogTrailingPartial {
				status = "warning"
			}
			report.Checks = append(report.Checks, DoctorCheck{
				Check: "log-journal", Scope: "run", Status: status,
				Code: CodeLogCorrupt, Summary: logErr.Error(),
				Service: name, RunID: runID,
			})
		}
	}
	report.Reconciliation.UnindexedRuns = doctorUnindexedRuns(store, indexed)
	for _, runID := range report.Reconciliation.UnindexedRuns {
		report.Checks = append(report.Checks, DoctorCheck{
			Check: "unindexed-run", Scope: "repository", Status: "warning",
			Summary:     "run directory is not referenced by the environment index",
			RunID:       runID,
			Remediation: "inspect the run artifacts; do not signal processes based on directory presence alone",
		})
	}
	addDoctorDiskCheck(&report, store, indexed)
	return report, nil
}

func inspectDoctorPlugins(
	ctx context.Context,
	repo *repository.Repository,
	timeout time.Duration,
) []DoctorCheck {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: min(timeout, 2*time.Second),
		ShutdownTimeout:  min(timeout, 2*time.Second),
	})
	checks := make([]DoctorCheck, 0, len(repo.Specs))
	for _, spec := range repo.Specs {
		pluginContext, cancel := context.WithTimeout(ctx, timeout)
		client, err := factory.Start(pluginContext, spec, runtime.StartOptions{Meta: repo.Request})
		cancel()
		if err != nil {
			checks = append(checks, DoctorCheck{
				Check: "plugin-handshake", Scope: "plugin", Status: "error",
				Code: CodeConfigInvalid, Summary: err.Error(), Service: spec.ID,
			})
			continue
		}
		closeContext, closeCancel := context.WithTimeout(context.Background(), min(timeout, 2*time.Second))
		closeErr := client.Close(closeContext)
		closeCancel()
		status := "ok"
		summary := "plugin handshake and shutdown succeeded"
		if closeErr != nil {
			status = "warning"
			summary = closeErr.Error()
		}
		checks = append(checks, DoctorCheck{
			Check: "plugin-handshake", Scope: "plugin", Status: status,
			Summary: summary, Service: spec.ID,
		})
	}
	return checks
}

func inspectDoctorRun(
	service string,
	slot runstate.ServiceSlot,
	run *runstate.RunRecord,
) []DoctorCheck {
	checks := []DoctorCheck{{
		Check: "run", Scope: "service", Status: "ok",
		Summary: "run schema and service index are valid",
		Service: service, RunID: run.RunID,
	}}
	if run.Service != service {
		checks[0].Status = "error"
		checks[0].Code = CodeStateCorrupt
		checks[0].Summary = "run service does not match environment slot"
	}
	for label, identity := range map[string]*runstate.ProcessIdentity{
		"wrapper-identity": run.Wrapper,
		"child-identity":   run.Child,
	} {
		if identity == nil {
			continue
		}
		status, err := runstate.InspectProcess(identity)
		check := DoctorCheck{
			Check: label, Scope: "run", Status: "ok",
			Summary: "process identity matches", Service: service, RunID: run.RunID,
		}
		if err != nil {
			check.Status = "error"
			check.Code = CodeProcessIdentityUnsupported
			check.Summary = err.Error()
		} else if status == runstate.ProcessMismatch {
			check.Status = "error"
			check.Code = CodeProcessIdentityMismatch
			check.Summary = "PID start token belongs to another process"
		} else if status == runstate.ProcessAbsent && slot.CurrentRunID == run.RunID &&
			run.Phase != runstate.RunExited && run.Phase != runstate.RunFailed {
			check.Status = "warning"
			check.Code = CodeProcessIdentityMismatch
			check.Summary = "current non-terminal run process is absent"
		}
		checks = append(checks, check)
	}
	if run.Child != nil && run.ChildPGID > 0 {
		pgid, err := runstate.ReadProcessGroupID(run.Child.PID)
		check := DoctorCheck{
			Check: "process-group", Scope: "run", Status: "ok",
			Summary: "child process group matches durable ready evidence",
			Service: service, RunID: run.RunID,
		}
		if err != nil && !os.IsNotExist(errors.Cause(err)) {
			check.Status = "warning"
			check.Summary = err.Error()
		} else if err == nil && pgid != run.ChildPGID {
			check.Status = "error"
			check.Code = CodeProcessIdentityMismatch
			check.Summary = "child process group differs from durable ready evidence"
		}
		checks = append(checks, check)
	}
	if run.Spec.Health != nil {
		status := "ok"
		summary := "health configuration is valid"
		if run.Spec.Health.Type != "tcp" && run.Spec.Health.Type != "http" {
			status = "error"
			summary = "health type must be tcp or http"
		}
		checks = append(checks, DoctorCheck{
			Check: "health-config", Scope: "run", Status: status,
			Summary: summary, Service: service, RunID: run.RunID,
		})
	}
	return checks
}

func doctorUnindexedRuns(store *runstate.Store, indexed map[string]bool) []string {
	entries, err := os.ReadDir(store.RunsDir())
	if err != nil {
		return nil
	}
	result := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && !indexed[entry.Name()] {
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result
}

func addDoctorDiskCheck(
	report *DoctorReport,
	store *runstate.Store,
	indexed map[string]bool,
) {
	var bytes int64
	var files int
	_ = filepath.WalkDir(store.RunsDir(), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr == nil {
			bytes += info.Size()
			files++
		}
		return nil
	})
	report.Checks = append(report.Checks, DoctorCheck{
		Check: "run-storage", Scope: "repository", Status: "ok",
		Summary: fmt.Sprintf("%d bytes across %d artifact files and %d indexed runs", bytes, files, len(indexed)),
		Path:    store.RunsDir(),
	})
}
