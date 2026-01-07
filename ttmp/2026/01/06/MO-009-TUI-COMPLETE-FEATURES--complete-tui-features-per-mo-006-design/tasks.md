# MO-009: TUI Complete Features - Task Tracker

## Phase 1: Data Layer Enhancements

### 1.1 Process Stats
- [ ] 1.1.1 Add CPU/MEM/Command/Cwd fields to state.ServiceRecord
- [ ] 1.1.2 Create pkg/proc/stats.go for reading /proc stats
- [ ] 1.1.3 Integrate stats polling into supervisor (2-5s interval)
- [ ] 1.1.4 Update tui.StateSnapshot to include ProcessStats map

### 1.2 Health Check Data
- [ ] 1.2.1 Define HealthCheckResult struct
- [ ] 1.2.2 Add health polling to supervisor (5s interval)
- [ ] 1.2.3 Update StateSnapshot with Health map
- [ ] 1.2.4 Add HealthIcon function to styles/icons.go

### 1.3 Environment Variables
- [ ] 1.3.1 Capture sanitized env at launch time
- [ ] 1.3.2 Add Environment map to ServiceRecord
- [ ] 1.3.3 Create env sanitization helper (redact secrets)

---

## Phase 2: Dashboard Enhancements

### 2.1 Health/CPU/MEM Columns
- [ ] 2.1.1 Update services table columns (Name, Status, Health, PID, CPU, MEM)
- [ ] 2.1.2 Create formatCPU/formatMem formatters
- [ ] 2.1.3 Add health icon to service row
- [ ] 2.1.4 Handle missing data gracefully (show "-")

### 2.2 Recent Events Preview
- [ ] 2.2.1 Add recentEvents field to DashboardModel
- [ ] 2.2.2 Subscribe dashboard to event log updates
- [ ] 2.2.3 Render events preview box (last 5)
- [ ] 2.2.4 Format event lines compactly

### 2.3 Plugins Summary
- [ ] 2.3.1 Add PluginSummary struct to StateSnapshot
- [ ] 2.3.2 Read plugin info from devctl config
- [ ] 2.3.3 Render plugins summary box

---

## Phase 3: Service Detail Enhancements

### 3.1 Process Info Section
- [ ] 3.1.1 Add process info box (PID, Command, Cwd, CPU, MEM)
- [ ] 3.1.2 Show started time/uptime (humanized)
- [ ] 3.1.3 Adjust layout for all sections

### 3.2 Health Check Info
- [ ] 3.2.1 Add health section with status icon
- [ ] 3.2.2 Show endpoint and last check time

### 3.3 Environment Variables
- [ ] 3.3.1 Add env section with compact formatting
- [ ] 3.3.2 Create env formatter (wrap to width)
- [ ] 3.3.3 Optional: expand/collapse toggle

### 3.4 Keybindings
- [ ] 3.4.1 Add stop (s) keybinding
- [ ] 3.4.2 Add detach (d) keybinding
- [ ] 3.4.3 Update footer keybindings

---

## Phase 4: Events View Enhancements

### 4.1 Service Source Column
- [x] 4.1.1 Add Source field to EventLogEntry
- [x] 4.1.2 Update event rendering with [service] prefix

### 4.2 Log Level Column
- [x] 4.2.1 Add LogLevel type (DEBUG/INFO/WARN/ERROR)
- [x] 4.2.2 Add LogLevelIcon function
- [x] 4.2.3 Update event rendering with level icon

### 4.3 Service Filter Toggles
- [ ] 4.3.1 Add serviceFilters map to EventLogModel
- [ ] 4.3.2 Add toggle keybindings (1-9 or space)
- [ ] 4.3.3 Render filter status bar
- [ ] 4.3.4 Apply filters in refreshViewportContent

### 4.4 Level Filter Toggles
- [ ] 4.4.1 Add levelFilters map
- [ ] 4.4.2 Add level toggle menu (l key)
- [ ] 4.4.3 Apply level filters

### 4.5 Stats Line
- [ ] 4.5.1 Track event count and rate
- [ ] 4.5.2 Calculate events/sec
- [ ] 4.5.3 Render stats line

### 4.6 Pause Toggle
- [ ] 4.6.1 Add paused state
- [ ] 4.6.2 Add pause (p) keybinding
- [ ] 4.6.3 Show pause indicator
- [ ] 4.6.4 Queue events while paused

---

## Phase 5: Pipeline View Enhancements

### 5.1 Progress Bars
- [ ] 5.1.1 Create progress bar widget
- [ ] 5.1.2 Add progress to step display
- [ ] 5.1.3 Wire up PipelineStepProgress messages

### 5.2 Live Output Viewport
- [ ] 5.2.1 Add liveOutput and liveVp to PipelineModel
- [ ] 5.2.2 Handle PipelineLiveOutputMsg
- [ ] 5.2.3 Render live output box
- [ ] 5.2.4 Wire up streaming from build executor

### 5.3 Config Patches Display
- [ ] 5.3.1 Add ConfigPatch struct and patches list
- [ ] 5.3.2 Handle ConfigPatchApplied messages
- [ ] 5.3.3 Render patches section

---

## Phase 6: Plugin List View

### 6.1 PluginModel
- [ ] 6.1.1 Add ViewPlugins to view types
- [ ] 6.1.2 Create PluginModel struct
- [ ] 6.1.3 Implement Update() for navigation
- [ ] 6.1.4 Implement View() with expandable cards
- [ ] 6.1.5 Wire up to RootModel

---

## Phase 7: Navigation Updates

### 7.1 Direct View Navigation
- [ ] 7.1.1 Add global navigation keybindings (s/e/p/b)
- [ ] 7.1.2 Update help overlay
- [ ] 7.1.3 Update footer shortcuts

---

## Phase 8: Polish and Testing

### 8.1 Responsive Layout
- [ ] 8.1.1 Define minimum dimensions
- [ ] 8.1.2 Hide optional sections when small
- [ ] 8.1.3 Collapse layouts for narrow terminals

### 8.2 Visual Consistency
- [ ] 8.2.1 Audit all views for consistent styling
- [ ] 8.2.2 Ensure icons are consistent
- [ ] 8.2.3 Test with light/dark themes

### 8.3 Error Handling
- [ ] 8.3.1 Handle missing data gracefully
- [ ] 8.3.2 Show meaningful unavailable messages
- [ ] 8.3.3 Handle resize during rendering

### 8.4 Testing
- [ ] 8.4.1 Create comprehensive fixture repo
- [ ] 8.4.2 Test at multiple terminal sizes
- [ ] 8.4.3 Test all keybindings
- [ ] 8.4.4 Test edge cases

---

## Summary

| Phase | Tasks | Status |
|-------|-------|--------|
| 1. Data Layer | 12 | ⏳ Not started |
| 2. Dashboard | 11 | ⏳ Not started |
| 3. Service Detail | 9 | ⏳ Not started |
| 4. Events View | 14 | ⏳ Not started |
| 5. Pipeline View | 10 | ⏳ Not started |
| 6. Plugin View | 5 | ⏳ Not started |
| 7. Navigation | 3 | ⏳ Not started |
| 8. Polish | 11 | ⏳ Not started |
| **Total** | **75** | |
