package tui

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/devctl/pkg/proc"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
)

type StateWatcher struct {
	RepoRoot string
	Interval time.Duration
	Pub      message.Publisher

	lastAlive  map[string]bool
	lastExists bool
	cpuTracker *proc.CPUTracker
}

func (w *StateWatcher) Run(ctx context.Context) error {
	if w.RepoRoot == "" {
		return errors.New("missing RepoRoot")
	}
	if w.Pub == nil {
		return errors.New("missing Publisher")
	}
	if w.Interval <= 0 {
		w.Interval = 1 * time.Second
	}

	// Initialize CPU tracker for calculating CPU percentages
	w.cpuTracker = proc.NewCPUTracker()

	t := time.NewTicker(w.Interval)
	defer t.Stop()

	for {
		if err := w.emitSnapshot(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (w *StateWatcher) emitSnapshot(ctx context.Context) error {
	_ = ctx
	path := state.StatePath(w.RepoRoot)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			w.lastAlive = nil
			w.lastExists = false
			return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: false})
		}
		w.lastAlive = nil
		w.lastExists = true
		return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, Error: errors.Wrap(err, "stat state").Error()})
	}

	st, err := state.Load(w.RepoRoot)
	if err != nil {
		w.lastAlive = nil
		w.lastExists = true
		return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, Error: errors.Wrap(err, "load state").Error()})
	}

	alive := map[string]bool{}
	for _, s := range st.Services {
		alive[s.Name] = state.ProcessAlive(s.PID)
	}

	if w.lastExists && w.lastAlive != nil {
		for _, svc := range st.Services {
			prev := w.lastAlive[svc.Name]
			now := alive[svc.Name]
			if prev && !now {
				if err := w.publishServiceExit(ServiceExitObserved{
					Name:   svc.Name,
					PID:    svc.PID,
					When:   time.Now(),
					Reason: "process not alive",
				}); err != nil {
					return err
				}
			}
		}
	}

	w.lastAlive = alive
	w.lastExists = true

	// Read process stats for all alive processes
	var pids []int
	for _, svc := range st.Services {
		if alive[svc.Name] {
			pids = append(pids, svc.PID)
		}
	}

	var processStats map[int]*proc.Stats
	if len(pids) > 0 {
		processStats, _ = proc.ReadAllStats(pids, w.cpuTracker)
		// Cleanup stale PIDs from the tracker
		w.cpuTracker.CleanupStale(pids)
	}

	// Check health for services with health config
	health := w.checkHealth(st.Services, alive)

	return w.publishSnapshot(StateSnapshot{
		RepoRoot:     w.RepoRoot,
		At:           time.Now(),
		Exists:       true,
		State:        st,
		Alive:        alive,
		ProcessStats: processStats,
		Health:       health,
	})
}

// checkHealth runs health checks for services with health config.
func (w *StateWatcher) checkHealth(services []state.ServiceRecord, alive map[string]bool) map[string]*HealthCheckResult {
	results := make(map[string]*HealthCheckResult)

	for _, svc := range services {
		// Skip if no health config
		if svc.HealthType == "" {
			continue
		}

		// Skip if process is dead
		if !alive[svc.Name] {
			results[svc.Name] = &HealthCheckResult{
				ServiceName: svc.Name,
				Status:      HealthUnhealthy,
				LastCheck:   time.Now(),
				CheckType:   svc.HealthType,
				Error:       "process not running",
			}
			continue
		}

		result := w.runHealthCheck(svc)
		results[svc.Name] = result
	}

	return results
}

// runHealthCheck performs a single health check for a service.
func (w *StateWatcher) runHealthCheck(svc state.ServiceRecord) *HealthCheckResult {
	result := &HealthCheckResult{
		ServiceName: svc.Name,
		CheckType:   svc.HealthType,
		LastCheck:   time.Now(),
	}

	start := time.Now()

	switch strings.ToLower(svc.HealthType) {
	case "tcp":
		result.Endpoint = svc.HealthAddress
		err := w.checkTCP(svc.HealthAddress)
		result.ResponseMs = time.Since(start).Milliseconds()
		if err != nil {
			result.Status = HealthUnhealthy
			result.Error = err.Error()
		} else {
			result.Status = HealthHealthy
		}

	case "http":
		url := svc.HealthURL
		if url == "" {
			url = svc.HealthAddress
		}
		result.Endpoint = url
		err := w.checkHTTP(url)
		result.ResponseMs = time.Since(start).Milliseconds()
		if err != nil {
			result.Status = HealthUnhealthy
			result.Error = err.Error()
		} else {
			result.Status = HealthHealthy
		}

	default:
		result.Status = HealthUnknown
		result.Error = "unknown health check type: " + svc.HealthType
	}

	return result
}

// checkTCP performs a TCP health check.
func (w *StateWatcher) checkTCP(address string) error {
	if address == "" {
		return errors.New("missing address")
	}
	conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// checkHTTP performs an HTTP health check.
func (w *StateWatcher) checkHTTP(url string) error {
	if url == "" {
		return errors.New("missing url")
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errors.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

func (w *StateWatcher) publishSnapshot(snap StateSnapshot) error {
	env, err := NewEnvelope(DomainTypeStateSnapshot, snap)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	_ = json.Valid(b)
	return w.Pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}

func (w *StateWatcher) publishServiceExit(ev ServiceExitObserved) error {
	env, err := NewEnvelope(DomainTypeServiceExit, ev)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	_ = json.Valid(b)
	return w.Pub.Publish(TopicDevctlEvents, message.NewMessage(watermill.NewUUID(), b))
}
