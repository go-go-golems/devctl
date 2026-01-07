---
Title: Implementation Plan
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - backend
    - ui-components
DocType: design
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Implementation plan for completing all missing devctl TUI features"
LastUpdated: 2026-01-07T02:50:20-05:00
WhatFor: "Provide an ordered plan and task breakdown for MO-009"
WhenToUse: "When implementing or reviewing MO-009 tasks"
---

# MO-009: Complete TUI Features Implementation Plan

## Overview

This document provides a comprehensive implementation plan for completing all missing TUI features identified in the MO-008 gap analysis. The goal is to bring the `devctl tui` implementation to full parity with the MO-006 design specification.

## Reference Documents

- **Original Design**: `MO-006-DEVCTL-TUI/.../01-devctl-tui-layout.md`
- **Gap Analysis**: `MO-008-IMPROVE-TUI-LOOKS/.../03-gap-analysis-vs-design.md`
- **Current Implementation**: `pkg/tui/models/*.go`

---

# Phase 1: Data Layer Enhancements

Before implementing UI features, we need to ensure the data layer provides all required information.

## 1.1 Enhance StateSnapshot with Process Stats

**Goal**: Add CPU/Memory stats to service records

### Tasks

- [ ] **1.1.1** Add process stats fields to `state.ServiceRecord`
  ```go
  type ServiceRecord struct {
      // existing fields...
      CPUPercent  float64   `json:"cpu_percent,omitempty"`
      MemoryMB    int64     `json:"memory_mb,omitempty"`
      Command     string    `json:"command,omitempty"`
      WorkingDir  string    `json:"working_dir,omitempty"`
      StartedAt   time.Time `json:"started_at,omitempty"`
  }
  ```

- [ ] **1.1.2** Create `pkg/proc/stats.go` for reading process stats
  ```go
  type ProcessStats struct {
      PID        int
      CPUPercent float64
      MemoryMB   int64
      Command    string
      Cwd        string
  }
  
  func ReadProcessStats(pid int) (*ProcessStats, error)
  func ReadAllProcessStats(pids []int) (map[int]*ProcessStats, error)
  ```

- [ ] **1.1.3** Integrate stats reading into supervisor state polling
  - Read from `/proc/[pid]/stat` for CPU/MEM on Linux
  - Fallback to `ps` command for macOS
  - Poll every 2-5 seconds

- [ ] **1.1.4** Update `tui.StateSnapshot` to include stats
  ```go
  type StateSnapshot struct {
      // existing...
      ProcessStats map[string]*ProcessStats `json:"process_stats,omitempty"`
  }
  ```

**Files to modify**:
- `pkg/state/types.go`
- `pkg/state/reader.go`
- NEW: `pkg/proc/stats.go`
- `pkg/tui/domain.go`

---

## 1.2 Add Health Check Data

**Goal**: Expose health check results to TUI

### Tasks

- [ ] **1.2.1** Define health check result structure
  ```go
  type HealthCheckResult struct {
      ServiceName string
      Status      HealthStatus // UNKNOWN | HEALTHY | UNHEALTHY
      LastCheck   time.Time
      CheckType   string // "tcp" | "http" | "exec"
      Endpoint    string // e.g., "http://localhost:8080/health"
      Error       string
      ResponseMs  int64
  }
  
  type HealthStatus string
  const (
      HealthUnknown   HealthStatus = "unknown"
      HealthHealthy   HealthStatus = "healthy"
      HealthUnhealthy HealthStatus = "unhealthy"
  )
  ```

- [ ] **1.2.2** Add health check polling to supervisor
  - Read health endpoints from service config
  - Poll every 5 seconds
  - Store results in state or separate file

- [ ] **1.2.3** Update `StateSnapshot` with health data
  ```go
  type StateSnapshot struct {
      // existing...
      Health map[string]*HealthCheckResult `json:"health,omitempty"`
  }
  ```

- [ ] **1.2.4** Create health check icons/styles
  ```go
  // styles/icons.go
  const (
      IconHealthy   = "●"  // green
      IconUnhealthy = "●"  // red
      IconUnknown   = "○"  // gray
  )
  ```

**Files to modify**:
- `pkg/state/types.go`
- `pkg/supervisor/health.go` (new or existing)
- `pkg/tui/domain.go`
- `pkg/tui/styles/icons.go`

---

## 1.3 Add Environment Variables to Service State

**Goal**: Store and display service environment variables

### Tasks

- [ ] **1.3.1** Capture environment at launch time
  - Store sanitized env vars (redact secrets)
  - Save to `state.json` or separate file

- [ ] **1.3.2** Add env to ServiceRecord
  ```go
  type ServiceRecord struct {
      // existing...
      Environment map[string]string `json:"environment,omitempty"`
  }
  ```

- [ ] **1.3.3** Create env sanitization helper
  ```go
  func SanitizeEnv(env map[string]string) map[string]string {
      // Redact keys containing: PASSWORD, SECRET, TOKEN, KEY, CREDENTIAL
  }
  ```

**Files to modify**:
- `pkg/state/types.go`
- `pkg/supervisor/launch.go`
- NEW: `pkg/state/sanitize.go`

---

# Phase 2: Dashboard Enhancements

## 2.1 Add Health/CPU/MEM Columns to Services Table

**Goal**: Show service health and resource usage at a glance

### Tasks

- [ ] **2.1.1** Update `DashboardModel.View()` to include new columns
  ```go
  serviceColumns := []widgets.TableColumn{
      {Header: "Name", Width: 16},
      {Header: "Status", Width: 10},
      {Header: "Health", Width: 8},
      {Header: "PID", Width: 8},
      {Header: "CPU", Width: 6},
      {Header: "MEM", Width: 8},
  }
  ```

- [ ] **2.1.2** Create formatters for CPU/MEM display
  ```go
  func formatCPU(pct float64) string  // "12.3%"
  func formatMem(mb int64) string     // "245MB"
  ```

- [ ] **2.1.3** Add health icon to service row
  ```go
  healthIcon := styles.HealthIcon(health.Status)
  rows[i] = widgets.TableRow{
      Icon:     statusIcon,
      Cells:    []string{name, status, healthIcon, pid, cpu, mem},
      Selected: i == m.selected,
  }
  ```

- [ ] **2.1.4** Handle missing data gracefully
  - Show "-" for unavailable CPU/MEM
  - Show "○" (unknown) for services without health checks

**Files to modify**:
- `pkg/tui/models/dashboard_model.go`
- `pkg/tui/styles/icons.go` (add HealthIcon)
- `pkg/tui/widgets/table.go` (may need dynamic columns)

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Services (3)                                     [l] logs  [r] restart  [x] kill│
│> ✓ backend    Running  ● Healthy  12847  12.3%  245MB                          │
│  ✓ frontend   Running  ● Healthy  12849   3.1%   89MB                          │
│  ✓ postgres   Running  ● Healthy  12843   1.0%  156MB                          │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 2.2 Add Recent Events Preview Box

**Goal**: Show last 5 events on dashboard without switching views

### Tasks

- [ ] **2.2.1** Add events preview to DashboardModel
  ```go
  type DashboardModel struct {
      // existing...
      recentEvents []tui.EventLogEntry // last 5
  }
  ```

- [ ] **2.2.2** Subscribe dashboard to event log updates
  ```go
  case tui.EventLogAppendMsg:
      // Add to recent events, keep last 5
      m.recentEvents = append(m.recentEvents, v.Entry)
      if len(m.recentEvents) > 5 {
          m.recentEvents = m.recentEvents[len(m.recentEvents)-5:]
      }
  ```

- [ ] **2.2.3** Render events preview box in View()
  ```go
  eventsBox := widgets.NewBox("Recent Events (5)").
      WithTitleRight("[e] all events").
      WithContent(renderRecentEvents(m.recentEvents)).
      WithSize(m.width, 7)
  ```

- [ ] **2.2.4** Format event lines compactly
  ```go
  func renderRecentEvents(events []tui.EventLogEntry) string {
      // 14:23:45  backend  ℹ  Request completed in 234ms
  }
  ```

**Files to modify**:
- `pkg/tui/models/dashboard_model.go`
- `pkg/tui/models/root_model.go` (route events to dashboard)

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Recent Events (5)                                              [e] all events  │
│ 14:23:45  backend   ℹ  Request completed in 234ms                             │
│ 14:23:42  frontend  ℹ  Asset compilation complete                             │
│ 14:23:40  backend   ℹ  Connected to database                                  │
│ 14:23:38  postgres  ℹ  Database ready for connections                         │
│ 14:23:35  system    ✓  All health checks passed                               │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 2.3 Add Plugins Summary Section

**Goal**: Show active plugins on dashboard

### Tasks

- [ ] **2.3.1** Add plugin data to StateSnapshot
  ```go
  type PluginSummary struct {
      Name     string
      Priority int
      OpsCount int
      CmdsCount int
      Status   string // "active" | "disabled"
  }
  
  type StateSnapshot struct {
      // existing...
      Plugins []PluginSummary `json:"plugins,omitempty"`
  }
  ```

- [ ] **2.3.2** Read plugin info from devctl config
  - Parse `.devctl.yaml` for plugin definitions
  - Count ops/commands per plugin

- [ ] **2.3.3** Render plugins summary in dashboard
  ```go
  pluginsBox := widgets.NewBox(fmt.Sprintf("Plugins (%d active)", len(plugins))).
      WithTitleRight("[p] details").
      WithContent(renderPluginsSummary(plugins)).
      WithSize(m.width, len(plugins)+3)
  ```

**Files to modify**:
- `pkg/tui/domain.go`
- `pkg/tui/models/dashboard_model.go`
- `pkg/config/plugins.go` (or similar)

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Plugins (3 active)                                               [p] details   │
│ • moments-config   (priority: 10)   ops: 2   commands: 0                      │
│ • moments-build    (priority: 15)   ops: 3   commands: 1                      │
│ • moments-launch   (priority: 20)   ops: 4   commands: 2                      │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

# Phase 3: Service Detail Enhancements

## 3.1 Add Process Info Section

**Goal**: Show detailed process information

### Tasks

- [ ] **3.1.1** Add process info box to ServiceModel.View()
  ```go
  processInfo := lipgloss.JoinVertical(lipgloss.Left,
      theme.TitleMuted.Render(fmt.Sprintf("PID:         %d", rec.PID)),
      theme.TitleMuted.Render(fmt.Sprintf("Command:     %s", stats.Command)),
      theme.TitleMuted.Render(fmt.Sprintf("Working Dir: %s", stats.Cwd)),
      theme.TitleMuted.Render(fmt.Sprintf("CPU:         %.1f%%", stats.CPUPercent)),
      theme.TitleMuted.Render(fmt.Sprintf("Memory:      %d MB", stats.MemoryMB)),
  )
  
  processBox := widgets.NewBox("Process Info").
      WithContent(processInfo).
      WithSize(m.width, 7)
  ```

- [ ] **3.1.2** Show started time/uptime
  ```go
  startedAgo := humanize.Time(rec.StartedAt) // "2h 34m ago"
  ```

- [ ] **3.1.3** Adjust layout to fit all sections
  - Process info: 7 lines
  - Health info: 3 lines (optional)
  - Env vars: 3 lines (collapsed)
  - Log viewport: remaining

**Files to modify**:
- `pkg/tui/models/service_model.go`
- Consider adding `github.com/dustin/go-humanize` dependency

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Process Info                                                                   │
│ PID:         12847                                                            │
│ Command:     go run ./cmd/moments-server serve                                │
│ Working Dir: /home/user/moments/backend                                       │
│ CPU:         12.3%                                                            │
│ Memory:      245.8 MB                                                         │
│ Started:     2h 34m ago                                                       │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 3.2 Add Health Check Info

**Goal**: Show health check status and endpoint

### Tasks

- [ ] **3.2.1** Add health section to ServiceModel.View()
  ```go
  if health != nil {
      healthIcon := styles.HealthIcon(health.Status)
      healthContent := lipgloss.JoinVertical(lipgloss.Left,
          lipgloss.JoinHorizontal(lipgloss.Center,
              theme.StatusRunning.Render(healthIcon),
              " ",
              theme.Title.Render(string(health.Status)),
          ),
          theme.TitleMuted.Render(fmt.Sprintf("Endpoint:   %s", health.Endpoint)),
          theme.TitleMuted.Render(fmt.Sprintf("Last check: %s (%dms)", 
              humanize.Time(health.LastCheck), health.ResponseMs)),
      )
      healthBox := widgets.NewBox("Health").
          WithContent(healthContent).
          WithSize(m.width, 5)
      sections = append(sections, healthBox.Render())
  }
  ```

**Files to modify**:
- `pkg/tui/models/service_model.go`

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Health                                                                         │
│ ● Healthy                                                                     │
│ Endpoint:   http://localhost:8083/rpc/v1/health                               │
│ Last check: 2s ago (45ms)                                                     │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 3.3 Add Environment Variables Section

**Goal**: Show service environment (sanitized)

### Tasks

- [ ] **3.3.1** Add env section to ServiceModel.View()
  ```go
  if len(rec.Environment) > 0 {
      envLines := formatEnvVars(rec.Environment, m.width-8)
      envBox := widgets.NewBox("Environment").
          WithTitleRight("[expand]").
          WithContent(strings.Join(envLines, "\n")).
          WithSize(m.width, min(len(envLines)+3, 6)) // collapse if many
      sections = append(sections, envBox.Render())
  }
  ```

- [ ] **3.3.2** Create compact env formatter
  ```go
  func formatEnvVars(env map[string]string, maxWidth int) []string {
      // PORT=8083  DB_URL=postgresql://...  ENV=development
      // Wrap to fit width, truncate long values
  }
  ```

- [ ] **3.3.3** Add expand/collapse toggle (optional)

**Files to modify**:
- `pkg/tui/models/service_model.go`

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Environment                                                         [e] expand │
│ PORT=8083  DB_URL=postgresql://localhost:5432/moments  ENV=development        │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 3.4 Add Stop/Detach Keybindings

**Goal**: Full service control from detail view

### Tasks

- [ ] **3.4.1** Add stop (s) keybinding
  ```go
  case "s":
      return m, m.requestAction(tui.ActionStop)
  ```

- [ ] **3.4.2** Add detach (d) keybinding
  ```go
  case "d":
      // Navigate back without stopping service
      return m, func() tea.Msg { return tui.NavigateBackMsg{} }
  ```

- [ ] **3.4.3** Update footer keybindings
  ```go
  return []widgets.Keybind{
      {Key: "tab", Label: "stream"},
      {Key: "f", Label: "follow"},
      {Key: "/", Label: "filter"},
      {Key: "r", Label: "restart"},
      {Key: "s", Label: "stop"},
      {Key: "k", Label: "kill"},
      {Key: "d", Label: "detach"},
      {Key: "esc", Label: "back"},
  }
  ```

**Files to modify**:
- `pkg/tui/models/service_model.go`
- `pkg/tui/models/root_model.go` (footer keybindings)

---

# Phase 4: Events View Enhancements

## 4.1 Add Service Source Column

**Goal**: Show which service produced each event

### Tasks

- [ ] **4.1.1** Add Source field to EventLogEntry
  ```go
  type EventLogEntry struct {
      At     time.Time
      Text   string
      Source string   // "backend", "frontend", "system"
      Level  LogLevel // DEBUG, INFO, WARN, ERROR
  }
  ```

- [ ] **4.1.2** Update event rendering
  ```go
  line := lipgloss.JoinHorizontal(lipgloss.Center,
      style.Render(icon),
      " ",
      theme.TitleMuted.Render(ts.Format("15:04:05")),
      "  ",
      theme.KeybindKey.Width(10).Render("["+e.Source+"]"),
      "  ",
      style.Render(e.Text),
  )
  ```

**Files to modify**:
- `pkg/tui/domain.go`
- `pkg/tui/models/eventlog_model.go`

**ASCII Target**:
```
│ 14:34:12.234  [backend]    INFO  POST /api/moments                          │
│ 14:34:12.156  [postgres]   INFO  Query: SELECT * FROM moments LIMIT 20      │
│ 14:34:11.987  [frontend]   INFO  HMR update: components/MomentList.tsx      │
```

---

## 4.2 Add Log Level Column with Icons

**Goal**: Visual log level indicators

### Tasks

- [ ] **4.2.1** Add LogLevel type
  ```go
  type LogLevel string
  const (
      LogDebug LogLevel = "DEBUG"
      LogInfo  LogLevel = "INFO"
      LogWarn  LogLevel = "WARN"
      LogError LogLevel = "ERROR"
  )
  ```

- [ ] **4.2.2** Add level icons
  ```go
  // styles/icons.go
  func LogLevelIcon(level LogLevel) string {
      switch level {
      case LogDebug: return "●" // gray
      case LogInfo:  return "ℹ" // blue
      case LogWarn:  return "⚠" // yellow
      case LogError: return "✗" // red
      default: return "?"
      }
  }
  ```

- [ ] **4.2.3** Update event rendering with level

**Files to modify**:
- `pkg/tui/domain.go`
- `pkg/tui/styles/icons.go`
- `pkg/tui/models/eventlog_model.go`

---

## 4.3 Add Service Filter Toggles

**Goal**: Toggle visibility of events by service

### Tasks

- [ ] **4.3.1** Add filter state to EventLogModel
  ```go
  type EventLogModel struct {
      // existing...
      serviceFilters map[string]bool // "backend" -> true (visible)
  }
  ```

- [ ] **4.3.2** Add toggle keybindings
  ```go
  case "1", "2", "3", "4", "5", "6", "7", "8", "9":
      // Toggle service filter by index
      idx := int(v.String()[0] - '1')
      m = m.toggleServiceFilter(idx)
  ```

- [ ] **4.3.3** Render filter status bar
  ```go
  filterBar := "Filters: "
  for name, enabled := range m.serviceFilters {
      icon := "●" // filled if enabled
      if !enabled { icon = "○" }
      filterBar += fmt.Sprintf("%s %s  ", icon, name)
  }
  ```

- [ ] **4.3.4** Apply filters in refreshViewportContent
  ```go
  for _, e := range m.entries {
      if !m.serviceFilters[e.Source] {
          continue // skip filtered services
      }
      // ... render line
  }
  ```

**Files to modify**:
- `pkg/tui/models/eventlog_model.go`

**ASCII Target**:
```
│ Filters: ● backend  ● frontend  ● postgres  ○ system      [space] toggle     │
│ Levels:  ● DEBUG   ● INFO   ● WARN   ● ERROR              [l] level menu     │
```

---

## 4.4 Add Level Filter Toggles

**Goal**: Filter events by log level

### Tasks

- [ ] **4.4.1** Add level filter state
  ```go
  type EventLogModel struct {
      // existing...
      levelFilters map[LogLevel]bool
  }
  ```

- [ ] **4.4.2** Add level toggle menu
  - Press `l` to cycle through levels
  - Or show submenu

- [ ] **4.4.3** Apply level filters

**Files to modify**:
- `pkg/tui/models/eventlog_model.go`

---

## 4.5 Add Stats Line

**Goal**: Show event throughput and buffer status

### Tasks

- [ ] **4.5.1** Track event stats
  ```go
  type EventLogModel struct {
      // existing...
      eventCount   int
      eventsPerSec float64
      droppedCount int
      lastStatTime time.Time
  }
  ```

- [ ] **4.5.2** Calculate events/sec
  ```go
  func (m EventLogModel) updateStats() EventLogModel {
      elapsed := time.Since(m.lastStatTime).Seconds()
      m.eventsPerSec = float64(m.eventCount) / elapsed
      m.eventCount = 0
      m.lastStatTime = time.Now()
      return m
  }
  ```

- [ ] **4.5.3** Render stats line
  ```go
  statsLine := fmt.Sprintf("Stats: %d events (%.0f/sec)   Buffer: %d lines   Dropped: %d",
      len(m.entries), m.eventsPerSec, m.max, m.droppedCount)
  ```

**Files to modify**:
- `pkg/tui/models/eventlog_model.go`

**ASCII Target**:
```
│ Stats: 1,247 events (18/sec)   Buffer: 500 lines   Dropped: 0                │
```

---

## 4.6 Add Pause Toggle

**Goal**: Pause event stream for reading

### Tasks

- [ ] **4.6.1** Add pause state
  ```go
  type EventLogModel struct {
      // existing...
      paused bool
  }
  ```

- [ ] **4.6.2** Add pause (p) keybinding
  ```go
  case "p":
      m.paused = !m.paused
      return m, nil
  ```

- [ ] **4.6.3** Show pause indicator in header
  ```go
  title := "Events"
  if m.paused {
      title = "Events (PAUSED)"
  }
  ```

- [ ] **4.6.4** Queue events while paused
  ```go
  func (m EventLogModel) Append(e tui.EventLogEntry) EventLogModel {
      if m.paused {
          m.queuedEvents = append(m.queuedEvents, e)
          return m
      }
      // ... normal append
  }
  ```

**Files to modify**:
- `pkg/tui/models/eventlog_model.go`

---

# Phase 5: Pipeline View Enhancements

## 5.1 Add Progress Bars

**Goal**: Visual progress for long-running steps

### Tasks

- [ ] **5.1.1** Create progress bar widget
  ```go
  // widgets/progress.go
  type ProgressBar struct {
      Percent int    // 0-100
      Width   int
      Style   lipgloss.Style
  }
  
  func (p ProgressBar) Render() string {
      filled := p.Width * p.Percent / 100
      empty := p.Width - filled
      return fmt.Sprintf("█"*filled + "░"*empty)
  }
  ```

- [ ] **5.1.2** Add progress to step display
  ```go
  if step.ProgressPercent > 0 {
      bar := widgets.NewProgressBar(step.ProgressPercent).
          WithWidth(20)
      line += " " + bar.Render()
  }
  ```

- [ ] **5.1.3** Wire up PipelineStepProgress messages

**Files to create**:
- `pkg/tui/widgets/progress.go`

**Files to modify**:
- `pkg/tui/models/pipeline_model.go`

**ASCII Target**:
```
│ ▶ backend-compile     go build ./cmd/server  ██████████░░░░░░ 65%  5.3s     │
```

---

## 5.2 Add Live Output Viewport

**Goal**: Real-time build output streaming

### Tasks

- [ ] **5.2.1** Add live output state to PipelineModel
  ```go
  type PipelineModel struct {
      // existing...
      liveOutput []string
      liveVp     viewport.Model
  }
  ```

- [ ] **5.2.2** Handle LiveOutputLine messages
  ```go
  type PipelineLiveOutputMsg struct {
      Source string // step name
      Line   string
      Stream string // "stdout" | "stderr"
  }
  
  case tui.PipelineLiveOutputMsg:
      m.liveOutput = append(m.liveOutput, fmt.Sprintf("[%s] %s", v.Source, v.Line))
      m = m.refreshLiveViewport()
  ```

- [ ] **5.2.3** Render live output box
  ```go
  if len(m.liveOutput) > 0 {
      liveBox := widgets.NewBox("Live Output").
          WithContent(m.liveVp.View()).
          WithSize(m.width, 8)
      sections = append(sections, liveBox.Render())
  }
  ```

- [ ] **5.2.4** Wire up streaming from build executor

**Files to modify**:
- `pkg/tui/models/pipeline_model.go`
- `pkg/tui/domain.go` (add message type)
- `pkg/engine/pipeline.go` (emit live output)

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Live Output                                                                    │
│ [backend-compile] Building target: cmd/moments-server                         │
│ [backend-compile] Compiling 247 packages...                                   │
│ [backend-compile] ████████████████████░░░░░░░░░░ 65%                          │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 5.3 Add Config Patches Display

**Goal**: Show what config was mutated by plugins

### Tasks

- [ ] **5.3.1** Add config patches to pipeline state
  ```go
  type ConfigPatch struct {
      Plugin string
      Key    string // dotted path
      Value  string
  }
  
  type PipelineModel struct {
      // existing...
      configPatches []ConfigPatch
  }
  ```

- [ ] **5.3.2** Handle ConfigPatchApplied messages
  ```go
  case tui.PipelineConfigPatchMsg:
      m.configPatches = append(m.configPatches, v.Patch)
  ```

- [ ] **5.3.3** Render patches section
  ```go
  if len(m.configPatches) > 0 {
      var lines []string
      for _, p := range m.configPatches {
          lines = append(lines, fmt.Sprintf(" • %s → %s  (%s)", p.Key, p.Value, p.Plugin))
      }
      patchesBox := widgets.NewBox("Applied Config Patches").
          WithContent(strings.Join(lines, "\n")).
          WithSize(m.width, len(lines)+3)
      sections = append(sections, patchesBox.Render())
  }
  ```

**Files to modify**:
- `pkg/tui/models/pipeline_model.go`
- `pkg/tui/domain.go`
- `pkg/engine/config_mutation.go` (emit patches)

**ASCII Target**:
```
╭──────────────────────────────────────────────────────────────────────────────╮
│Applied Config Patches                                                         │
│ • services.backend.port → 8083                          (moments-config)      │
│ • services.postgres.image → postgres:15-alpine         (moments-config)      │
│ • build.cache_enabled → true                            (moments-build)       │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

# Phase 6: Plugin List View

## 6.1 Create PluginModel

**Goal**: New view for plugin inspection

### Tasks

- [ ] **6.1.1** Add ViewPlugin to view types
  ```go
  const (
      ViewDashboard ViewID = "dashboard"
      ViewService   ViewID = "service"
      ViewEvents    ViewID = "events"
      ViewPipeline  ViewID = "pipeline"
      ViewPlugins   ViewID = "plugins" // NEW
  )
  ```

- [ ] **6.1.2** Create PluginModel struct
  ```go
  type PluginModel struct {
      width   int
      height  int
      plugins []PluginInfo
      selected int
      expanded map[int]bool // which plugins are expanded
  }
  
  type PluginInfo struct {
      Name       string
      Status     string // "active" | "disabled"
      Priority   int
      Path       string
      Protocol   string
      Ops        []string
      Streams    []string
      Commands   []string
      Declares   map[string]string
  }
  ```

- [ ] **6.1.3** Implement Update() for navigation
  ```go
  case tea.KeyMsg:
      switch v.String() {
      case "up", "k":
          m.selected--
      case "down", "j":
          m.selected++
      case "enter", "i":
          m.expanded[m.selected] = !m.expanded[m.selected]
      case "d":
          m = m.disablePlugin(m.selected)
      case "e":
          m = m.enablePlugin(m.selected)
      case "r":
          m = m.reloadPlugin(m.selected)
      case "esc":
          return m, func() tea.Msg { return tui.NavigateBackMsg{} }
      }
  ```

- [ ] **6.1.4** Implement View() with expandable cards
  ```go
  func (m PluginModel) View() string {
      var sections []string
      for i, p := range m.plugins {
          card := m.renderPluginCard(i, p, m.expanded[i])
          sections = append(sections, card)
      }
      return lipgloss.JoinVertical(lipgloss.Left, sections...)
  }
  ```

- [ ] **6.1.5** Wire up to RootModel
  - Add `plugins PluginModel` field
  - Handle navigation to/from
  - Add footer keybindings

**Files to create**:
- `pkg/tui/models/plugin_model.go`

**Files to modify**:
- `pkg/tui/models/root_model.go`

**ASCII Target**:
```
╭─ moments-config ───────────────────────────────────────────────────────────╮
│ Status:   ✓ Active                                    Priority: 10         │
│ Path:     ./moments/plugins/devctl-config.sh                                │
│ Protocol: v1                                                                │
│ Capabilities:                                                               │
│   Ops:      config.mutate, validate.run                                     │
│   Streams:  (none)                                                          │
│   Commands: (none)                                                          │
╰─────────────────────────────────────────────────────────────────────────────╯
```

---

# Phase 7: Navigation and Keybinding Updates

## 7.1 Add Direct View Navigation

**Goal**: Quick jump to any view from anywhere

### Tasks

- [ ] **7.1.1** Add global navigation keybindings
  ```go
  // In RootModel.Update(), before view-specific handling:
  switch v.String() {
  case "s": // services (dashboard)
      m.active = ViewDashboard
      return m, nil
  case "e": // events
      m.active = ViewEvents
      return m, nil
  case "p": // plugins
      m.active = ViewPlugins
      return m, nil
  case "b": // build/pipeline
      m.active = ViewPipeline
      return m, nil
  }
  ```

- [ ] **7.1.2** Update help overlay with new keybindings

- [ ] **7.1.3** Update footer to show view shortcuts

**Files to modify**:
- `pkg/tui/models/root_model.go`

---

# Phase 8: Polish and Testing

## 8.1 Responsive Layout

**Goal**: Graceful handling of small terminals

### Tasks

- [ ] **8.1.1** Define minimum dimensions
  ```go
  const (
      MinWidth  = 60
      MinHeight = 15
  )
  ```

- [ ] **8.1.2** Hide optional sections when too small
- [ ] **8.1.3** Collapse multi-column layouts to single column

## 8.2 Visual Consistency

### Tasks

- [ ] **8.2.1** Audit all views for consistent styling
- [ ] **8.2.2** Ensure icons are consistent across views
- [ ] **8.2.3** Test with light/dark terminal themes

## 8.3 Error Handling

### Tasks

- [ ] **8.3.1** Handle missing data gracefully (show "-" or "N/A")
- [ ] **8.3.2** Show meaningful messages when services unavailable
- [ ] **8.3.3** Handle resize during rendering

## 8.4 Testing

### Tasks

- [ ] **8.4.1** Create fixture repo with all scenarios
- [ ] **8.4.2** Test at multiple terminal sizes (80x24, 120x40, 200x60)
- [ ] **8.4.3** Test all keybindings
- [ ] **8.4.4** Test edge cases (no services, 100 services, etc.)

---

# Summary: Task Count by Phase

| Phase | Tasks | Effort |
|-------|-------|--------|
| 1. Data Layer | 12 | High |
| 2. Dashboard | 10 | Medium |
| 3. Service Detail | 8 | Medium |
| 4. Events View | 12 | Medium |
| 5. Pipeline View | 8 | Medium |
| 6. Plugin View | 5 | Medium |
| 7. Navigation | 3 | Low |
| 8. Polish | 8 | Medium |
| **Total** | **66** | |

---

# Dependencies Graph

```
Phase 1 (Data Layer)
    ├── 1.1 Process Stats
    │       └── Phase 2.1 (Dashboard CPU/MEM)
    │       └── Phase 3.1 (Service Process Info)
    ├── 1.2 Health Checks
    │       └── Phase 2.1 (Dashboard Health)
    │       └── Phase 3.2 (Service Health)
    ├── 1.3 Environment Vars
            └── Phase 3.3 (Service Env Vars)

Phase 4 (Events) - Independent, can start immediately
Phase 5 (Pipeline) - Partially independent (live output needs backend work)
Phase 6 (Plugins) - Independent, can start immediately
Phase 7 (Navigation) - After Phases 2-6
Phase 8 (Polish) - After all features complete
```

---

# Recommended Implementation Order

1. **Phase 4** (Events enhancements) - Low dependencies, high visibility
2. **Phase 2.2** (Dashboard events preview) - Uses events, high value
3. **Phase 1.1** (Process stats) - Enables dashboard/service CPU/MEM
4. **Phase 2.1** (Dashboard health/CPU/MEM) - Depends on 1.1
5. **Phase 3.1-3.3** (Service detail) - Depends on 1.1, 1.2, 1.3
6. **Phase 5** (Pipeline) - Medium effort, good polish
7. **Phase 6** (Plugins) - Lower priority, nice to have
8. **Phase 7-8** (Navigation + Polish) - Final pass
