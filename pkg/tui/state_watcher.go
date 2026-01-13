package tui

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/proc"
	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
)

type pluginIntrospection struct {
	Status     string
	Err        string
	StartedAt  time.Time
	FinishedAt time.Time
	Handshake  protocol.Handshake
}

type StateWatcher struct {
	RepoRoot string
	Interval time.Duration
	Pub      message.Publisher

	lastAlive  map[string]bool
	lastExists bool
	cpuTracker *proc.CPUTracker

	introspectCh chan struct{}
	capsMu       sync.RWMutex
	capsByID     map[string]pluginIntrospection
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
	w.ensureIntrospectionState()
	go w.introspectionLoop(ctx)
	w.requestIntrospection()

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

func (w *StateWatcher) ensureIntrospectionState() {
	if w.introspectCh == nil {
		w.introspectCh = make(chan struct{}, 1)
	}
	if w.capsByID == nil {
		w.capsByID = make(map[string]pluginIntrospection)
	}
}

func (w *StateWatcher) requestIntrospection() {
	if w.introspectCh == nil {
		return
	}
	select {
	case w.introspectCh <- struct{}{}:
	default:
	}
}

func (w *StateWatcher) RequestIntrospection() {
	w.ensureIntrospectionState()
	w.requestIntrospection()
}

func (w *StateWatcher) introspectionLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.introspectCh:
			w.runIntrospection(ctx)
		}
	}
}

func (w *StateWatcher) runIntrospection(ctx context.Context) {
	if w.RepoRoot == "" {
		return
	}

	repo, err := repository.Load(repository.Options{RepoRoot: w.RepoRoot, ConfigPath: "", Cwd: w.RepoRoot})
	if err != nil {
		return
	}

	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  2 * time.Second,
	})

	for _, spec := range repo.Specs {
		startedAt := time.Now()
		w.updateIntrospection(spec.ID, pluginIntrospection{
			Status:    "introspecting",
			StartedAt: startedAt,
		})

		c, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: repo.Request})
		if err != nil {
			w.updateIntrospection(spec.ID, pluginIntrospection{
				Status:     "error",
				Err:        err.Error(),
				StartedAt:  startedAt,
				FinishedAt: time.Now(),
			})
			continue
		}

		hs := c.Handshake()
		_ = c.Close(ctx)

		w.updateIntrospection(spec.ID, pluginIntrospection{
			Status:     "ok",
			StartedAt:  startedAt,
			FinishedAt: time.Now(),
			Handshake:  hs,
		})
	}
}

func (w *StateWatcher) updateIntrospection(id string, info pluginIntrospection) {
	w.capsMu.Lock()
	defer w.capsMu.Unlock()
	w.capsByID[id] = info
}

func (w *StateWatcher) emitSnapshot(ctx context.Context) error {
	_ = ctx
	// Always read plugins from config, regardless of state existence
	plugins := w.readPlugins()

	path := state.StatePath(w.RepoRoot)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			w.lastAlive = nil
			w.lastExists = false
			return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: false, Plugins: plugins})
		}
		w.lastAlive = nil
		w.lastExists = true
		return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, Error: errors.Wrap(err, "stat state").Error(), Plugins: plugins})
	}

	st, err := state.Load(w.RepoRoot)
	if err != nil {
		w.lastAlive = nil
		w.lastExists = true
		return w.publishSnapshot(StateSnapshot{RepoRoot: w.RepoRoot, At: time.Now(), Exists: true, Error: errors.Wrap(err, "load state").Error(), Plugins: plugins})
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
		Plugins:      plugins,
	})
}

// readPlugins reads plugin info from the devctl config file.
func (w *StateWatcher) readPlugins() []PluginSummary {
	cfgPath := config.DefaultPath(w.RepoRoot)
	cfg, err := config.LoadOptional(cfgPath)
	if err != nil || cfg == nil {
		return nil
	}

	plugins := make([]PluginSummary, 0, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		status := "active"

		// Check if plugin path/command is available
		pluginPath := p.Path
		if pluginPath != "" {
			if isCommandPath(pluginPath) {
				// It's a command name (no slashes), check if it exists in PATH
				if _, err := exec.LookPath(pluginPath); err != nil {
					status = "error"
				}
			} else {
				// It's a file path
				if !filepath.IsAbs(pluginPath) {
					pluginPath = filepath.Join(w.RepoRoot, pluginPath)
				}
				if _, err := os.Stat(pluginPath); err != nil {
					status = "error"
				}
			}
		}

		summary := PluginSummary{
			ID:        p.ID,
			Path:      p.Path,
			Priority:  p.Priority,
			Status:    status,
			CapStatus: "unknown",
		}

		if capInfo, ok := w.lookupIntrospection(p.ID); ok {
			summary.CapStatus = capInfo.Status
			summary.CapError = capInfo.Err
			summary.CapStart = capInfo.StartedAt
			summary.CapEnd = capInfo.FinishedAt
			if capInfo.Status == "ok" {
				summary.Protocol = string(capInfo.Handshake.ProtocolVersion)
				summary.Ops = capInfo.Handshake.Capabilities.Ops
				summary.Streams = capInfo.Handshake.Capabilities.Streams
				for _, cmd := range capInfo.Handshake.Capabilities.Commands {
					if cmd.Name != "" {
						summary.Commands = append(summary.Commands, cmd.Name)
					}
				}
			}
		}

		plugins = append(plugins, summary)
	}

	return plugins
}

func (w *StateWatcher) lookupIntrospection(id string) (pluginIntrospection, bool) {
	w.capsMu.RLock()
	defer w.capsMu.RUnlock()
	info, ok := w.capsByID[id]
	return info, ok
}

// isCommandPath returns true if the path looks like a command name (no slashes).
func isCommandPath(path string) bool {
	return !strings.Contains(path, "/")
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
