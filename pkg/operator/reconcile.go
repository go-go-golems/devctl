package operator

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
)

func reconcile(ctx context.Context, store *runstate.Store) (ReconciliationReport, error) {
	report := ReconciliationReport{
		Actions:       []ReconciliationAction{},
		UnindexedRuns: []string{},
	}
	environment, err := loadEnvironmentOptional(ctx, store)
	if err != nil {
		return report, errors.Wrap(err, "load environment for reconciliation")
	}
	if environment == nil {
		return report, nil
	}

	indexed := map[string]struct{}{}
	for _, slot := range environment.Services {
		if slot.CurrentRunID != "" {
			indexed[slot.CurrentRunID] = struct{}{}
		}
		if slot.LastRunID != "" {
			indexed[slot.LastRunID] = struct{}{}
		}
	}
	for _, name := range sortedSlotNames(environment) {
		slot := environment.Services[name]
		if slot.CurrentRunID == "" {
			continue
		}
		action, terminal, reconcileErr := reconcileCurrentRun(ctx, store, name, slot.CurrentRunID)
		if action.Action != "" {
			report.Actions = append(report.Actions, action)
		}
		if reconcileErr != nil {
			return report, reconcileErr
		}
		if terminal {
			latest, loadErr := store.LoadEnvironment(ctx)
			if loadErr != nil {
				return report, errors.Wrap(loadErr, "reload reconciled environment")
			}
			if updateErr := store.Update(ctx, latest.Revision, func(document *runstate.EnvironmentState) error {
				current := document.Services[name]
				if current.CurrentRunID != slot.CurrentRunID {
					return errors.Errorf("service %q current run changed during reconciliation", name)
				}
				current.LastRunID = current.CurrentRunID
				current.CurrentRunID = ""
				document.Services[name] = current
				return nil
			}); updateErr != nil {
				return report, errors.Wrap(updateErr, "clear terminal reconciled run")
			}
		}
	}

	entries, err := os.ReadDir(store.RunsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, errors.Wrap(err, "scan run directories")
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, exists := indexed[entry.Name()]; !exists {
			report.UnindexedRuns = append(report.UnindexedRuns, entry.Name())
		}
	}
	sort.Strings(report.UnindexedRuns)
	return report, nil
}

func reconcileCurrentRun(
	ctx context.Context,
	store *runstate.Store,
	service string,
	runID string,
) (ReconciliationAction, bool, error) {
	action := ReconciliationAction{Service: service, RunID: runID}
	run, err := store.LoadRun(ctx, runID)
	if err != nil {
		action.Action = "contradiction"
		action.Error = serviceError(CodeStateCorrupt, "current run record is missing or invalid", service, runID, err)
		return action, false, nil
	}
	action.Before = run.Phase
	if run.Service != service {
		return markReconciledUnknown(ctx, store, action, "RUN_SERVICE_MISMATCH", "run service does not match environment slot")
	}

	runDir, err := store.RunDir(runID)
	if err != nil {
		return action, false, err
	}
	exitInfo, exitErr := state.ReadExitInfo(filepath.Join(runDir, supervise.ExitRecordName))
	hasExit := exitErr == nil
	if exitErr != nil && !os.IsNotExist(errors.Cause(exitErr)) {
		return markReconciledUnknown(ctx, store, action, "EXIT_ARTIFACT_INVALID", exitErr.Error())
	}

	wrapperStatus, err := inspectOptionalIdentity(run.Wrapper)
	if err != nil {
		return action, false, errors.Wrap(err, "inspect reconciled wrapper")
	}
	childStatus, err := inspectOptionalIdentity(run.Child)
	if err != nil {
		return action, false, errors.Wrap(err, "inspect reconciled child")
	}
	if wrapperStatus == runstate.ProcessMismatch || childStatus == runstate.ProcessMismatch {
		return markReconciledUnknown(ctx, store, action, "PROCESS_IDENTITY_MISMATCH", "stored process start token no longer matches PID")
	}

	owner, ownerErr := supervise.ReadOwnerRecord(filepath.Join(runDir, supervise.OwnerRecordName))
	ready, readyErr := supervise.ReadReadyRecord(filepath.Join(runDir, supervise.ReadyRecordName))
	hasOwner := ownerErr == nil
	hasReady := readyErr == nil
	if ownerErr != nil && !os.IsNotExist(errors.Cause(ownerErr)) {
		return markReconciledUnknown(ctx, store, action, "OWNER_ARTIFACT_INVALID", ownerErr.Error())
	}
	if readyErr != nil && !os.IsNotExist(errors.Cause(readyErr)) {
		return markReconciledUnknown(ctx, store, action, "READY_ARTIFACT_INVALID", readyErr.Error())
	}
	if contradiction := validateReconciliationArtifacts(run, owner, ready, hasOwner, hasReady); contradiction != "" {
		return markReconciledUnknown(ctx, store, action, "HANDSHAKE_CONTRADICTION", contradiction)
	}

	if hasExit {
		if wrapperStatus != runstate.ProcessAbsent || childStatus != runstate.ProcessAbsent {
			return markReconciledUnknown(ctx, store, action, "EXIT_WITH_LIVE_PROCESS", "exit artifact exists while an owned process remains live")
		}
		phase := runstate.RunExited
		if exitInfo.Error != "" && (run.Phase == runstate.RunPlanned || run.Phase == runstate.RunStarting || run.Phase == runstate.RunFailed) {
			phase = runstate.RunFailed
		}
		if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
			record.Phase = phase
			record.Exit = &runstate.ExitSummary{
				ExitedAt: exitInfo.ExitedAt,
				ExitCode: exitInfo.ExitCode,
				Signal:   exitInfo.Signal,
				Error:    exitInfo.Error,
			}
			return nil
		}); err != nil {
			return action, false, errors.Wrap(err, "persist reconciled exit")
		}
		action.After = phase
		action.Action = "recorded-exit"
		return action, true, nil
	}

	if hasReady {
		ownerStatus, inspectErr := runstate.InspectProcess(&owner.Wrapper)
		if inspectErr != nil {
			return action, false, inspectErr
		}
		childReadyStatus, inspectErr := runstate.InspectProcess(&ready.Child)
		if inspectErr != nil {
			return action, false, inspectErr
		}
		if ownerStatus == runstate.ProcessMismatch || childReadyStatus == runstate.ProcessMismatch {
			return markReconciledUnknown(ctx, store, action, "PROCESS_IDENTITY_MISMATCH", "handshake identity no longer matches PID")
		}
		if ownerStatus == runstate.ProcessMatches && childReadyStatus == runstate.ProcessMatches {
			pgid, pgidErr := runstate.ReadProcessGroupID(ready.Child.PID)
			if pgidErr != nil {
				return action, false, pgidErr
			}
			if pgid != ready.ChildPGID {
				return markReconciledUnknown(ctx, store, action, "PROCESS_GROUP_MISMATCH", "child process group does not match ready artifact")
			}
			phase := runstate.RunStarting
			if run.Spec.Health == nil || (run.Phase == runstate.RunReady && run.Health != nil && run.Health.Healthy) {
				phase = runstate.RunReady
			}
			if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
				record.Wrapper = &owner.Wrapper
				record.Child = &ready.Child
				record.ChildPGID = ready.ChildPGID
				record.Phase = phase
				return nil
			}); err != nil {
				return action, false, errors.Wrap(err, "persist reconciled handshake")
			}
			action.After = phase
			action.Action = "adopted-handshake"
			return action, false, nil
		}
		return markReconciledUnknown(ctx, store, action, "MISSING_EXIT_ARTIFACT", "owned handshake processes exited without an exit artifact")
	}

	if hasOwner {
		ownerStatus, inspectErr := runstate.InspectProcess(&owner.Wrapper)
		if inspectErr != nil {
			return action, false, inspectErr
		}
		if ownerStatus == runstate.ProcessMatches {
			if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
				record.Wrapper = &owner.Wrapper
				record.Phase = runstate.RunStarting
				return nil
			}); err != nil {
				return action, false, errors.Wrap(err, "persist reconciled owner")
			}
			action.After = runstate.RunStarting
			action.Action = "adopted-owner"
			return action, false, nil
		}
		return markReconciledUnknown(ctx, store, action, "OWNER_EXIT_UNPROVEN", "wrapper owner disappeared without ready or exit evidence")
	}

	if run.Phase == runstate.RunPlanned && run.Wrapper == nil && run.Child == nil {
		if err := store.UpdateRun(ctx, runID, func(record *runstate.RunRecord) error {
			record.Phase = runstate.RunFailed
			record.LastError = &runstate.ErrorRecord{
				Code:    "START_NOT_OBSERVED",
				Message: "controller stopped before wrapper ownership was established",
			}
			return nil
		}); err != nil {
			return action, false, errors.Wrap(err, "mark unstarted planned run failed")
		}
		action.After = runstate.RunFailed
		action.Action = "failed-unstarted-run"
		return action, true, nil
	}
	if run.Phase == runstate.RunFailed || run.Phase == runstate.RunExited {
		action.After = run.Phase
		action.Action = "cleared-terminal-run"
		return action, true, nil
	}
	return markReconciledUnknown(ctx, store, action, "OWNERSHIP_EVIDENCE_MISSING", "non-terminal run has no durable ownership or exit evidence")
}

func validateReconciliationArtifacts(
	run *runstate.RunRecord,
	owner *supervise.OwnerRecord,
	ready *supervise.ReadyRecord,
	hasOwner bool,
	hasReady bool,
) string {
	if hasReady && !hasOwner {
		return "ready artifact exists without owner artifact"
	}
	if hasOwner && (owner.Version != supervise.HandshakeVersion || owner.RunID != run.RunID || owner.Service != run.Service) {
		return "owner artifact does not match run record"
	}
	if hasReady {
		if ready.Version != supervise.HandshakeVersion || ready.RunID != run.RunID || ready.Service != run.Service {
			return "ready artifact does not match run record"
		}
		if ready.Wrapper != owner.Wrapper || ready.ChildPGID <= 0 || ready.ChildPGID != ready.Child.PID {
			return "ready artifact identities or child process group are inconsistent"
		}
	}
	return ""
}

func inspectOptionalIdentity(identity *runstate.ProcessIdentity) (runstate.ProcessStatus, error) {
	if identity == nil {
		return runstate.ProcessAbsent, nil
	}
	return runstate.InspectProcess(identity)
}

func markReconciledUnknown(
	ctx context.Context,
	store *runstate.Store,
	action ReconciliationAction,
	code string,
	message string,
) (ReconciliationAction, bool, error) {
	operatorErr := serviceError(CodeProcessIdentityMismatch, message, action.Service, action.RunID, nil)
	operatorErr.Details = map[string]any{"reconciliation_code": code}
	if err := store.UpdateRun(ctx, action.RunID, func(record *runstate.RunRecord) error {
		record.Phase = runstate.RunUnknown
		record.LastError = &runstate.ErrorRecord{Code: code, Message: message}
		return nil
	}); err != nil {
		return action, false, errors.Wrap(err, "mark reconciled run unknown")
	}
	action.After = runstate.RunUnknown
	action.Action = "marked-unknown"
	action.Error = operatorErr
	return action, false, nil
}
