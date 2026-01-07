# Investigation: Dashboard Not Rendering Pipeline Status or Plugins

**Date**: 2026-01-07  
**Ticket**: MO-009-TUI-COMPLETE-FEATURES  
**Severity**: High  
**Status**: Fixed

---

## 1. Symptom Description

User reported two issues when running `devctl tui`:

1. **Pipeline phases not showing**: When pressing `[u]` from the dashboard to start services, no pipeline status was displayed despite the pipeline running.

2. **Plugins view empty**: Even though `devctl plugins list` worked correctly from the command line, the TUI's Plugins view showed "No plugins configured."

---

## 2. Investigation Approach

### 2.1 Message Flow Tracing

First, I traced the message flow for pipeline events:

```
User presses [u]
    ↓
DashboardModel.Update() returns ActionRequestMsg{Kind: ActionUp}
    ↓
RootModel.Update() calls publishAction(req)
    ↓
PublishAction() publishes to TopicUIActions
    ↓
RegisterUIActionRunner handler receives message
    ↓
Publishes PipelineRunStarted to TopicDevctlEvents (IMMEDIATELY)
    ↓
RegisterDomainToUITransformer transforms to UITypePipelineRunStarted
    ↓
Publishes to TopicUIMessages
    ↓
RegisterUIForwarder sends PipelineRunStartedMsg to tea.Program
    ↓
RootModel.Update() receives PipelineRunStartedMsg
    ↓
Calls m.dashboard.WithPipelineStarted(v.Run)
```

**Finding**: The message flow was correct. Messages were being published and forwarded properly.

### 2.2 RootModel Pipeline Handling

Verified that `RootModel.Update()` correctly handles pipeline messages:

```go
// pkg/tui/models/root_model.go:172-186
case tui.PipelineRunStartedMsg:
    m.dashboard = m.dashboard.WithPipelineStarted(v.Run)  // ✓ Updates dashboard
    var cmd tea.Cmd
    m.pipeline, cmd = m.pipeline.Update(v)
    return m, cmd

case tui.PipelinePhaseStartedMsg:
    m.dashboard = m.dashboard.WithPipelinePhase(v.Event.Phase)  // ✓ Updates dashboard
    var cmd tea.Cmd
    m.pipeline, cmd = m.pipeline.Update(v)
    return m, cmd
```

**Finding**: Dashboard model was being updated correctly with pipeline state.

### 2.3 DashboardModel State Fields

Verified that `DashboardModel` has the correct fields to track pipeline state:

```go
// pkg/tui/models/dashboard_model.go:38-44
type DashboardModel struct {
    // ... other fields ...
    
    // Pipeline status
    pipelineRunning bool
    pipelineKind    tui.ActionKind
    pipelinePhase   tui.PipelinePhase
    pipelineStarted time.Time
    pipelineOk      *bool
}
```

**Finding**: Fields exist and are being set correctly by `WithPipelineStarted()`.

### 2.4 DashboardModel.View() Analysis - **ROOT CAUSE FOUND**

Examined the `View()` function flow:

```go
// pkg/tui/models/dashboard_model.go:185-201
func (m DashboardModel) View() string {
    theme := styles.DefaultTheme()

    if m.last == nil {
        return theme.TitleMuted.Render("Loading state...")  // Early return #1
    }

    s := m.last
    if !s.Exists {
        return m.renderStopped(theme)  // ← EARLY RETURN #2 - BUG HERE
    }
    if s.Error != "" {
        return m.renderError(theme, s.Error)  // ← EARLY RETURN #3 - BUG HERE
    }
    if s.State == nil {
        return theme.TitleMuted.Render("System: Unknown...")  // Early return #4
    }

    // Pipeline status only rendered here (line 283-286)
    if m.pipelineRunning {
        sections = append(sections, m.renderPipelineStatus(theme))
    }
    
    // Plugins summary only rendered here (line 298-300)
    if s.Plugins != nil && len(s.Plugins) > 0 {
        sections = append(sections, m.renderPluginsSummary(...))
    }
    // ...
}
```

**ROOT CAUSE IDENTIFIED**:

When state doesn't exist (`!s.Exists`), the function returns early from `renderStopped()`, which:
- Does NOT check `m.pipelineRunning`
- Does NOT render pipeline status
- Does NOT render plugins summary

Same issue exists in `renderError()`.

### 2.5 renderStopped() Function Before Fix

```go
// pkg/tui/models/dashboard_model.go:384-397 (BEFORE FIX)
func (m DashboardModel) renderStopped(theme styles.Theme) string {
    icon := theme.StatusPending.Render(styles.IconSystem)
    status := theme.Title.Render("System: Stopped")
    hint := theme.TitleMuted.Render("Press [u] to start")

    box := widgets.NewBox("Dashboard").
        WithContent(lipgloss.JoinVertical(lipgloss.Left,
            lipgloss.JoinHorizontal(lipgloss.Center, icon, " ", status),
            "",
            hint,
        )).
        WithSize(m.width, 6)

    return box.Render()  // ← Returns ONLY the stopped box, nothing else
}
```

**Problem**: This function completely ignores:
1. `m.pipelineRunning` - The pipeline could be running!
2. `m.last.Plugins` - Plugins are available from config!

---

## 3. Secondary Issue: Plugins Not Read When State Missing

### 3.1 StateWatcher.emitSnapshot() Analysis

```go
// pkg/tui/state_watcher.go:62-74 (BEFORE FIX)
func (w *StateWatcher) emitSnapshot(ctx context.Context) error {
    path := state.StatePath(w.RepoRoot)
    _, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            // ↓ Plugins NOT included in this snapshot!
            return w.publishSnapshot(StateSnapshot{
                RepoRoot: w.RepoRoot, 
                At: time.Now(), 
                Exists: false,
            })
        }
        // ...
    }
    
    // Plugins only read here, after we know state exists
    plugins := w.readPlugins()  // ← Only reached if state exists!
}
```

**Problem**: Plugins were only read when state file existed. But plugins come from `.devctl.yaml`, not from state!

---

## 4. User Workflow Analysis

When user starts TUI with no services running:

```
1. User runs: devctl tui --repo-root /tmp/fixture
2. StateWatcher.emitSnapshot() runs
3. State file doesn't exist → publishSnapshot({Exists: false})  // No plugins!
4. Dashboard.View() sees !s.Exists → calls renderStopped()
5. renderStopped() shows "System: Stopped" with no plugins
6. User presses [u]
7. ActionRunner publishes PipelineRunStarted
8. Dashboard.WithPipelineStarted() sets m.pipelineRunning = true
9. Dashboard.View() still calls renderStopped() (state still doesn't exist)
10. renderStopped() ignores m.pipelineRunning → Shows nothing about pipeline!
```

**Result**: User sees "System: Stopped" with no indication that pipeline is running.

---

## 5. Timeline of Events During Startup

```
T+0.0s   User presses [u]
T+0.0s   DashboardModel sends ActionRequestMsg
T+0.0s   RootModel publishes action
T+0.0s   ActionRunner receives action
T+0.0s   ActionRunner publishes PipelineRunStarted (Kind: up, Phases: [...])
T+0.0s   Dashboard receives PipelineRunStartedMsg
T+0.0s   m.pipelineRunning = true, m.pipelineKind = "up"
T+0.0s   Dashboard.View() called → renderStopped() → NO PIPELINE SHOWN!

T+0.1s   ActionRunner: discovery.Discover() starts
T+0.1s   ActionRunner: runtime.Factory.Start() for each plugin
T+2.0s   Plugin handshakes complete (slow plugins add ~2s delay)
T+2.0s   ActionRunner publishes PipelinePhaseStarted{Phase: "mutate_config"}
T+2.0s   Dashboard receives phase update
T+2.0s   Dashboard.View() → renderStopped() → STILL NO PIPELINE SHOWN!

T+2.5s   State file created (services starting)
T+2.5s   StateWatcher sees state exists
T+2.5s   Dashboard.View() now takes main path → Shows pipeline status
```

**Key Insight**: For the first ~2.5 seconds of the pipeline, user sees nothing because `renderStopped()` was ignoring pipeline state.

---

## 6. Fix Applied

### 6.1 Fix for state_watcher.go

Read plugins at the start of `emitSnapshot()`, include in ALL snapshot types:

```go
func (w *StateWatcher) emitSnapshot(ctx context.Context) error {
    // Always read plugins from config, regardless of state existence
    plugins := w.readPlugins()  // ← Moved to top

    path := state.StatePath(w.RepoRoot)
    _, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            return w.publishSnapshot(StateSnapshot{
                RepoRoot: w.RepoRoot, 
                At: time.Now(), 
                Exists: false,
                Plugins: plugins,  // ← Now included!
            })
        }
        // ... also include plugins in error case
    }
    // ...
}
```

### 6.2 Fix for renderStopped()

```go
func (m DashboardModel) renderStopped(theme styles.Theme) string {
    var sections []string

    // Original stopped box
    box := widgets.NewBox("Dashboard").WithContent(...)
    sections = append(sections, box.Render())

    // NEW: Show pipeline status if running
    if m.pipelineRunning {
        sections = append(sections, "")
        sections = append(sections, m.renderPipelineStatus(theme))
    }

    // NEW: Show plugins summary if available
    if m.last != nil && len(m.last.Plugins) > 0 {
        sections = append(sections, "")
        sections = append(sections, m.renderPluginsSummary(theme, m.last.Plugins))
    }

    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
```

### 6.3 Fix for renderError()

Same pattern applied to show pipeline and plugins in error state.

---

## 7. Additional Issue: readPlugins() Command Path Handling

Also fixed in this session: `readPlugins()` was incorrectly treating command names like `python3` as file paths:

```go
// BEFORE (broken):
pluginPath := p.Path
if pluginPath != "" && pluginPath[0] != '/' {
    pluginPath = w.RepoRoot + "/" + pluginPath  // /tmp/fixture/python3 ← Wrong!
}
if _, err := os.Stat(pluginPath); err != nil {
    status = "error"  // Always fails for commands!
}

// AFTER (fixed):
if isCommandPath(pluginPath) {
    // It's a command name, check with exec.LookPath
    if _, err := exec.LookPath(pluginPath); err != nil {
        status = "error"
    }
} else {
    // It's a file path, use os.Stat
    if !filepath.IsAbs(pluginPath) {
        pluginPath = filepath.Join(w.RepoRoot, pluginPath)
    }
    if _, err := os.Stat(pluginPath); err != nil {
        status = "error"
    }
}

func isCommandPath(path string) bool {
    return !strings.Contains(path, "/")
}
```

---

## 8. Lessons Learned

### 8.1 Design Pattern Issue

The `View()` function had multiple early-return paths that bypassed important rendering logic. This is a **fragmented rendering** anti-pattern.

**Better approach**: 
- Collect all sections that should be rendered
- Have early returns only for truly terminal states (loading, fatal error)
- Always check cross-cutting concerns (pipeline status, global state) in all render paths

### 8.2 State vs Config Confusion

Plugins are defined in `.devctl.yaml` (config), not in the state file. The code incorrectly assumed plugins were only relevant when state existed.

**Rule**: Config-derived data should be available regardless of runtime state.

### 8.3 Testing Gap

No test verified that pipeline status was visible when state didn't exist. This is a common edge case: "what shows during the transition period?"

---

## 9. Files Changed

1. `pkg/tui/state_watcher.go` - Read plugins at top of `emitSnapshot()`, include in all snapshots
2. `pkg/tui/models/dashboard_model.go` - Update `renderStopped()` and `renderError()` to show pipeline and plugins

---

## 10. Verification Steps

1. Run `devctl tui --repo-root /tmp/fixture` with no state
2. Verify plugins are shown on dashboard
3. Press `[u]` to start services
4. Verify "Pipeline: up Running" appears immediately
5. Verify phase updates appear as pipeline progresses
6. Verify plugins remain visible throughout

