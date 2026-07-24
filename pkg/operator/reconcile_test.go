package operator

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
)

func TestReconcileFailsUnstartedPlannedRunAndClearsCurrent(t *testing.T) {
	store, runID := createReconcileFixture(t, runstate.RunPlanned)

	report, err := reconcile(context.Background(), store)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(report.Actions) != 1 || report.Actions[0].Action != "failed-unstarted-run" {
		t.Fatalf("unexpected report: %#v", report)
	}
	run, err := store.LoadRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Phase != runstate.RunFailed {
		t.Fatalf("phase = %q, want failed", run.Phase)
	}
	environment, err := store.LoadEnvironment(context.Background())
	if err != nil {
		t.Fatalf("load environment: %v", err)
	}
	slot := environment.Services["web"]
	if slot.CurrentRunID != "" || slot.LastRunID != runID {
		t.Fatalf("unexpected reconciled slot: %#v", slot)
	}
}

func TestReconcileRecordsExitAndReportsUnindexedRun(t *testing.T) {
	store, runID := createReconcileFixture(t, runstate.RunStarting)
	runDir, err := store.RunDir(runID)
	if err != nil {
		t.Fatalf("run dir: %v", err)
	}
	exitCode := 17
	if err := state.WriteExitInfo(filepath.Join(runDir, supervise.ExitRecordName), state.ExitInfo{
		Service: "web", ExitedAt: time.Now().UTC(), ExitCode: &exitCode,
	}); err != nil {
		t.Fatalf("write exit: %v", err)
	}
	orphanID := "018f0f65-6c1a-7abc-8def-0123456789ac"
	if err := store.CreateRun(context.Background(), runstate.RunRecord{
		RunID: orphanID, Service: "orphan", Phase: runstate.RunPlanned,
		Spec: runstate.ServiceSpecRecord{Name: "orphan", Command: []string{"serve"}},
	}); err != nil {
		t.Fatalf("create unindexed run: %v", err)
	}

	report, err := reconcile(context.Background(), store)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(report.UnindexedRuns) != 1 || report.UnindexedRuns[0] != orphanID {
		t.Fatalf("unexpected unindexed runs: %v", report.UnindexedRuns)
	}
	run, err := store.LoadRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Phase != runstate.RunExited || run.Exit == nil || run.Exit.ExitCode == nil || *run.Exit.ExitCode != exitCode {
		t.Fatalf("unexpected reconciled run: %#v", run)
	}
}

func TestSnapshotReconcilesExitedCurrentRun(t *testing.T) {
	store, runID := createReconcileFixture(t, runstate.RunReady)
	runDir, err := store.RunDir(runID)
	if err != nil {
		t.Fatalf("run dir: %v", err)
	}
	exitCode := 17
	if err := state.WriteExitInfo(filepath.Join(runDir, supervise.ExitRecordName), state.ExitInfo{
		Service: "web", ExitedAt: time.Now().UTC(), ExitCode: &exitCode,
	}); err != nil {
		t.Fatalf("write exit: %v", err)
	}
	controller, err := NewController(ControllerOptions{
		Planner: staticPlanner{},
		SupervisorFactory: func(string, time.Duration) serviceSupervisor {
			return &recordingSupervisor{t: t, repoRoot: store.RepoRoot()}
		},
	})
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}

	snapshot, err := controller.Snapshot(context.Background(), SnapshotRequest{
		RepoRoot: store.RepoRoot(), IncludeRuns: true,
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Services) != 1 {
		t.Fatalf("snapshot services = %d, want 1", len(snapshot.Services))
	}
	service := snapshot.Services[0]
	if service.Phase != runstate.RunExited || service.Exit == nil ||
		service.Exit.ExitCode == nil || *service.Exit.ExitCode != exitCode {
		t.Fatalf("snapshot did not expose reconciled exit: %#v", service)
	}
	environment, err := store.LoadEnvironment(context.Background())
	if err != nil {
		t.Fatalf("load environment: %v", err)
	}
	slot := environment.Services["web"]
	if slot.CurrentRunID != "" || slot.LastRunID != runID {
		t.Fatalf("snapshot did not reconcile environment slot: %#v", slot)
	}
}

func TestDoctorReportsWithoutReconcilingOrMutatingState(t *testing.T) {
	store, runID := createReconcileFixture(t, runstate.RunPlanned)
	controller, err := NewController(ControllerOptions{
		Planner: staticPlanner{},
		SupervisorFactory: func(string, time.Duration) serviceSupervisor {
			return &recordingSupervisor{t: t, repoRoot: store.RepoRoot()}
		},
	})
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	before, err := store.LoadEnvironment(context.Background())
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	report, err := controller.Doctor(context.Background(), DoctorRequest{RepoRoot: store.RepoRoot()})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if len(report.Checks) == 0 {
		t.Fatal("doctor returned no checks")
	}
	after, err := store.LoadEnvironment(context.Background())
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.Revision != before.Revision || after.Services["web"].CurrentRunID != runID {
		t.Fatalf("doctor mutated environment: before=%#v after=%#v", before, after)
	}
	run, err := store.LoadRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Phase != runstate.RunPlanned {
		t.Fatalf("doctor changed run phase to %q", run.Phase)
	}
}

func createReconcileFixture(t *testing.T, phase runstate.RunPhase) (*runstate.Store, string) {
	t.Helper()
	store, err := runstate.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	runID := "018f0f65-6c1a-7abc-8def-0123456789ab"
	if err := store.CreateRun(context.Background(), runstate.RunRecord{
		RunID: runID, Service: "web", Phase: phase,
		Spec: runstate.ServiceSpecRecord{Name: "web", Command: []string{"serve"}},
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := store.CreateEnvironment(context.Background(), runstate.EnvironmentState{
		Services: map[string]runstate.ServiceSlot{
			"web": {
				Name: "web", CurrentRunID: runID, Desired: runstate.DesiredRunning,
			},
		},
	}); err != nil {
		t.Fatalf("create environment: %v", err)
	}
	return store, runID
}
