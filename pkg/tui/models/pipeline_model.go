package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
	"github.com/go-go-golems/devctl/pkg/tui/widgets"
)

type PipelineModel struct {
	width  int
	height int

	runStarted  *tui.PipelineRunStarted
	runFinished *tui.PipelineRunFinished

	phaseOrder []tui.PipelinePhase
	phases     map[tui.PipelinePhase]*pipelinePhaseState

	buildSteps       []tui.PipelineStepResult
	prepareSteps     []tui.PipelineStepResult
	buildArtifacts   map[string]string
	prepareArtifacts map[string]string
	validate         *tui.PipelineValidateResult
	launchPlan       *tui.PipelineLaunchPlan

	// Live output viewport
	liveOutput   []string
	liveVp       viewport.Model
	liveVpReady  bool
	showLiveVp   bool
	liveVpHeight int

	// Config patches applied by plugins
	configPatches []tui.ConfigPatch

	// Step progress tracking (step name -> percent)
	stepProgress map[string]int

	focus            pipelineFocus
	buildCursor      int
	buildShow        bool
	prepareCursor    int
	prepareShow      bool
	validationCursor int
	validationShow   bool
}

type pipelineFocus string

const (
	pipelineFocusBuild      pipelineFocus = "build"
	pipelineFocusPrepare    pipelineFocus = "prepare"
	pipelineFocusValidation pipelineFocus = "validation"
)

type pipelinePhaseState struct {
	startedAt  time.Time
	finishedAt time.Time
	ok         *bool
	durationMs int64
	errText    string
}

func NewPipelineModel() PipelineModel {
	return PipelineModel{
		phases:       map[tui.PipelinePhase]*pipelinePhaseState{},
		stepProgress: map[string]int{},
		liveVpHeight: 8,
	}
}

func (m PipelineModel) WithSize(width, height int) PipelineModel {
	m.width, m.height = width, height
	if m.liveVpReady {
		m.liveVp.Width = width - 4 // account for box borders
		m.liveVp.Height = m.liveVpHeight - 3
	}
	return m
}

func (m PipelineModel) Update(msg tea.Msg) (PipelineModel, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "b":
			m.focus = pipelineFocusBuild
			m.buildShow = true
			return m, nil
		case "p":
			m.focus = pipelineFocusPrepare
			m.prepareShow = true
			return m, nil
		case "v":
			m.focus = pipelineFocusValidation
			m.validationShow = true
			return m, nil
		case "o": // toggle live output
			m.showLiveVp = !m.showLiveVp
			return m, nil
		case "up", "k":
			m = m.moveCursor(-1)
			return m, nil
		case "down", "j":
			m = m.moveCursor(1)
			return m, nil
		case "enter":
			m = m.toggleDetails()
			return m, nil
		default:
			return m, nil
		}
	case tui.PipelineRunStartedMsg:
		run := v.Run
		m.runStarted = &run
		m.runFinished = nil
		m.buildSteps = nil
		m.prepareSteps = nil
		m.buildArtifacts = nil
		m.prepareArtifacts = nil
		m.validate = nil
		m.launchPlan = nil
		m.focus = pipelineFocusBuild
		m.buildCursor = 0
		m.buildShow = false
		m.prepareCursor = 0
		m.prepareShow = false
		m.validationCursor = 0
		m.validationShow = false

		// Reset new state
		m.liveOutput = nil
		m.configPatches = nil
		m.stepProgress = map[string]int{}
		m.showLiveVp = false

		// Initialize live viewport
		m.liveVp = viewport.New(m.width-4, m.liveVpHeight-3)
		m.liveVpReady = true

		m.phases = map[tui.PipelinePhase]*pipelinePhaseState{}
		if len(run.Phases) > 0 {
			m.phaseOrder = append([]tui.PipelinePhase{}, run.Phases...)
		} else {
			m.phaseOrder = []tui.PipelinePhase{
				tui.PipelinePhaseMutateConfig,
				tui.PipelinePhaseBuild,
				tui.PipelinePhasePrepare,
				tui.PipelinePhaseValidate,
				tui.PipelinePhaseLaunchPlan,
				tui.PipelinePhaseSupervise,
				tui.PipelinePhaseStateSave,
			}
		}
		return m, nil
	case tui.PipelineRunFinishedMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Run.RunID {
			return m, nil
		}
		run := v.Run
		m.runFinished = &run
		return m, nil
	case tui.PipelinePhaseStartedMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Event.RunID {
			return m, nil
		}
		ph := m.phase(v.Event.Phase)
		ph.startedAt = v.Event.At
		ph.finishedAt = time.Time{}
		ph.ok = nil
		ph.durationMs = 0
		ph.errText = ""
		return m, nil
	case tui.PipelinePhaseFinishedMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Event.RunID {
			return m, nil
		}
		ph := m.phase(v.Event.Phase)
		ph.finishedAt = v.Event.At
		ph.durationMs = v.Event.DurationMs
		ok := v.Event.Ok
		ph.ok = &ok
		ph.errText = v.Event.Error
		return m, nil
	case tui.PipelineBuildResultMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Result.RunID {
			return m, nil
		}
		m.buildSteps = append([]tui.PipelineStepResult{}, v.Result.Steps...)
		m.buildArtifacts = copyStringMap(v.Result.Artifacts)
		if m.focus == "" {
			m.focus = pipelineFocusBuild
		}
		return m, nil
	case tui.PipelinePrepareResultMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Result.RunID {
			return m, nil
		}
		m.prepareSteps = append([]tui.PipelineStepResult{}, v.Result.Steps...)
		m.prepareArtifacts = copyStringMap(v.Result.Artifacts)
		return m, nil
	case tui.PipelineValidateResultMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Result.RunID {
			return m, nil
		}
		res := v.Result
		m.validate = &res
		m.validationCursor = 0
		m.validationShow = len(res.Errors) > 0 || len(res.Warnings) > 0
		return m, nil
	case tui.PipelineLaunchPlanMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Plan.RunID {
			return m, nil
		}
		plan := v.Plan
		m.launchPlan = &plan
		return m, nil
	case tui.PipelineLiveOutputMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Output.RunID {
			return m, nil
		}
		m.liveOutput = append(m.liveOutput, formatLiveOutputLine(v.Output))
		m.showLiveVp = true
		m = m.refreshLiveViewport()
		return m, nil
	case tui.PipelineConfigPatchesMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Patches.RunID {
			return m, nil
		}
		m.configPatches = append(m.configPatches, v.Patches.Patches...)
		return m, nil
	case tui.PipelineStepProgressMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.RunID {
			return m, nil
		}
		if m.stepProgress == nil {
			m.stepProgress = map[string]int{}
		}
		m.stepProgress[v.Step] = v.Percent
		return m, nil
	default:
		return m, nil
	}
}

func (m PipelineModel) View() string {
	theme := styles.DefaultTheme()

	if m.runStarted == nil {
		box := widgets.NewBox("Pipeline").
			WithContent(lipgloss.JoinVertical(lipgloss.Left,
				theme.TitleMuted.Render("No pipeline run recorded yet."),
				"",
				theme.TitleMuted.Render("Run [u] (up) or [r] (restart) from the dashboard to see progress here."),
			)).
			WithSize(m.width, 6)
		return box.Render()
	}

	run := m.runStarted
	statusIcon := styles.IconRunning
	statusText := "Running"
	statusStyle := theme.StatusRunning
	if m.runFinished != nil {
		if m.runFinished.Ok {
			statusIcon = styles.IconSuccess
			statusText = "OK"
		} else {
			statusIcon = styles.IconError
			statusText = "Failed"
			statusStyle = theme.StatusDead
		}
	}

	var sections []string

	// Pipeline header info
	headerLines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center,
			statusStyle.Render(statusIcon),
			" ",
			theme.Title.Render(fmt.Sprintf("Pipeline: %s", run.Kind)),
			"  ",
			statusStyle.Render(statusText),
		),
		theme.TitleMuted.Render(fmt.Sprintf("Run ID: %s", run.RunID)),
		theme.TitleMuted.Render(fmt.Sprintf("Started: %s", run.At.Format("2006-01-02 15:04:05"))),
	}

	// Focus indicator
	if m.focus != "" {
		focusLine := lipgloss.JoinHorizontal(lipgloss.Center,
			theme.TitleMuted.Render("Focus: "),
			theme.KeybindKey.Render(string(m.focus)),
			theme.TitleMuted.Render("  [b] build  [p] prepare  [v] validation  [o] output  [↑/↓] select"),
		)
		headerLines = append(headerLines, "", focusLine)
	}

	headerBox := widgets.NewBox("Pipeline Progress").
		WithContent(lipgloss.JoinVertical(lipgloss.Left, headerLines...)).
		WithSize(m.width, len(headerLines)+3)
	sections = append(sections, headerBox.Render())

	// Phases box
	var phaseLines []string
	for _, p := range m.phaseOrder {
		st := m.phases[p]
		icon, style := m.phaseIconAndStyle(st, theme)
		stateText := m.formatStyledPhaseState(st, theme)
		phaseLine := lipgloss.JoinHorizontal(lipgloss.Center,
			style.Render(icon),
			" ",
			lipgloss.NewStyle().Width(18).Render(string(p)),
			stateText,
		)
		phaseLines = append(phaseLines, phaseLine)
	}
	phasesBox := widgets.NewBox("Phases").
		WithContent(lipgloss.JoinVertical(lipgloss.Left, phaseLines...)).
		WithSize(m.width, len(phaseLines)+3)
	sections = append(sections, phasesBox.Render())

	// Build steps
	if len(m.buildSteps) > 0 {
		sections = append(sections, m.renderStyledSteps("Build Steps", m.buildSteps, m.focus == pipelineFocusBuild, m.buildCursor, m.buildShow, m.buildArtifacts, theme))
	}

	// Prepare steps
	if len(m.prepareSteps) > 0 {
		sections = append(sections, m.renderStyledSteps("Prepare Steps", m.prepareSteps, m.focus == pipelineFocusPrepare, m.prepareCursor, m.prepareShow, m.prepareArtifacts, theme))
	}

	// Validation
	if m.validate != nil {
		sections = append(sections, m.renderStyledValidation(theme))
	}

	// Config patches (show after validation, before launch plan)
	if len(m.configPatches) > 0 {
		sections = append(sections, m.renderConfigPatches(theme))
	}

	// Live output viewport
	if m.showLiveVp && len(m.liveOutput) > 0 {
		sections = append(sections, m.renderLiveOutput(theme))
	}

	// Launch plan
	if m.launchPlan != nil && len(m.launchPlan.Services) > 0 {
		launchContent := lipgloss.JoinHorizontal(lipgloss.Center,
			theme.StatusRunning.Render(styles.IconSuccess),
			" ",
			theme.TitleMuted.Render(fmt.Sprintf("%d services: ", len(m.launchPlan.Services))),
			theme.Title.Render(strings.Join(m.launchPlan.Services, ", ")),
		)
		launchBox := widgets.NewBox("Launch Plan").
			WithContent(launchContent).
			WithSize(m.width, 4)
		sections = append(sections, launchBox.Render())
	}

	// Final status
	if m.runFinished != nil {
		f := m.runFinished
		var finalLines []string
		if f.DurationMs > 0 {
			finalLines = append(finalLines, theme.TitleMuted.Render(fmt.Sprintf("Total: %s", formatDurationMs(f.DurationMs))))
		}
		if !f.Ok && f.Error != "" {
			finalLines = append(finalLines, theme.StatusDead.Render(fmt.Sprintf("Error: %s", f.Error)))
		}
		if len(finalLines) > 0 {
			sections = append(sections, lipgloss.JoinVertical(lipgloss.Left, finalLines...))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m PipelineModel) phaseIconAndStyle(st *pipelinePhaseState, theme styles.Theme) (string, lipgloss.Style) {
	if st == nil {
		return styles.IconPending, theme.StatusPending
	}
	if st.ok == nil && !st.startedAt.IsZero() {
		return styles.IconRunning, theme.StatusRunning
	}
	if st.ok == nil {
		return styles.IconPending, theme.StatusPending
	}
	if *st.ok {
		return styles.IconSuccess, theme.StatusRunning
	}
	return styles.IconError, theme.StatusDead
}

func (m PipelineModel) formatStyledPhaseState(st *pipelinePhaseState, theme styles.Theme) string {
	if st == nil {
		return theme.StatusPending.Render("pending")
	}
	if st.ok == nil && !st.startedAt.IsZero() {
		return theme.StatusRunning.Render("running...")
	}
	if st.ok == nil {
		return theme.StatusPending.Render("pending")
	}
	stateText := "ok"
	style := theme.StatusRunning
	if !*st.ok {
		stateText = "failed"
		style = theme.StatusDead
	}
	if st.durationMs > 0 {
		stateText = fmt.Sprintf("%s (%s)", stateText, formatDurationMs(st.durationMs))
	}
	return style.Render(stateText)
}

func (m PipelineModel) renderStyledSteps(title string, steps []tui.PipelineStepResult, focused bool, cursor int, showDetails bool, artifacts map[string]string, theme styles.Theme) string {
	var lines []string
	for i, s := range steps {
		icon := styles.IconSuccess
		style := theme.StatusRunning
		if !s.Ok {
			icon = styles.IconError
			style = theme.StatusDead
		}

		// Check for in-progress step with progress
		progressPct := s.ProgressPercent
		if pct, ok := m.stepProgress[s.Name]; ok && pct > 0 {
			progressPct = pct
		}

		cursorStr := "  "
		nameStyle := theme.TitleMuted
		if focused && i == clampInt(cursor, 0, maxInt(0, len(steps)-1)) {
			cursorStr = theme.KeybindKey.Render("> ")
			nameStyle = theme.Title
		}

		durationText := ""
		if s.DurationMs > 0 {
			durationText = theme.TitleMuted.Render(fmt.Sprintf(" (%s)", formatDurationMs(s.DurationMs)))
		}

		// Add progress bar for in-progress steps
		progressText := ""
		if progressPct > 0 && progressPct < 100 {
			bar := widgets.NewProgressBar(progressPct).
				WithWidth(15).
				WithStyle(theme.StatusRunning)
			progressText = " " + bar.Render()
			icon = styles.IconRunning
			style = theme.StatusRunning
		}

		line := lipgloss.JoinHorizontal(lipgloss.Center,
			cursorStr,
			style.Render(icon),
			" ",
			nameStyle.Width(20).Render(s.Name),
			progressText,
			durationText,
		)
		lines = append(lines, line)
	}

	// Details section
	if focused && showDetails && len(steps) > 0 {
		idx := clampInt(cursor, 0, len(steps)-1)
		sel := steps[idx]
		lines = append(lines, "")
		lines = append(lines, theme.Title.Render(fmt.Sprintf("Details: %s", sel.Name)))
		lines = append(lines, theme.TitleMuted.Render(fmt.Sprintf("  Status: %v", sel.Ok)))
		if sel.DurationMs > 0 {
			lines = append(lines, theme.TitleMuted.Render(fmt.Sprintf("  Duration: %s", formatDurationMs(sel.DurationMs))))
		}
		if len(artifacts) > 0 {
			lines = append(lines, theme.TitleMuted.Render(fmt.Sprintf("  Artifacts: %d", len(artifacts))))
		}
	}

	box := widgets.NewBox(title).
		WithTitleRight(fmt.Sprintf("%d steps", len(steps))).
		WithContent(lipgloss.JoinVertical(lipgloss.Left, lines...)).
		WithSize(m.width, len(lines)+3)
	return box.Render()
}

func (m PipelineModel) renderStyledValidation(theme styles.Theme) string {
	v := m.validate
	var headerLines []string

	// Summary
	if v.Valid {
		headerLines = append(headerLines, lipgloss.JoinHorizontal(lipgloss.Center,
			theme.StatusRunning.Render(styles.IconSuccess),
			" ",
			theme.Title.Render("Validation passed"),
			theme.TitleMuted.Render(fmt.Sprintf(" (%d warnings)", len(v.Warnings))),
		))
	} else {
		headerLines = append(headerLines, lipgloss.JoinHorizontal(lipgloss.Center,
			theme.StatusDead.Render(styles.IconError),
			" ",
			theme.StatusDead.Render("Validation failed"),
			theme.TitleMuted.Render(fmt.Sprintf(" (%d errors, %d warnings)", len(v.Errors), len(v.Warnings))),
		))
	}

	issues := validationIssues(v)
	if len(issues) == 0 {
		box := widgets.NewBox("Validation").
			WithContent(lipgloss.JoinVertical(lipgloss.Left, headerLines...)).
			WithSize(m.width, 4)
		return box.Render()
	}

	headerLines = append(headerLines, "")

	// Issue list
	for i, is := range issues {
		icon := styles.IconError
		style := theme.StatusDead
		if is.kind == "warn" {
			icon = styles.IconWarning
			style = lipgloss.NewStyle().Foreground(theme.Warning)
		}

		cursorStr := "  "
		if m.focus == pipelineFocusValidation && i == clampInt(m.validationCursor, 0, maxInt(0, len(issues)-1)) {
			cursorStr = theme.KeybindKey.Render("> ")
		}

		line := lipgloss.JoinHorizontal(lipgloss.Center,
			cursorStr,
			style.Render(icon),
			" ",
			style.Render(is.code),
			theme.TitleMuted.Render(": "),
			theme.TitleMuted.Render(is.message),
		)
		headerLines = append(headerLines, line)
	}

	// Details for selected issue
	if m.focus == pipelineFocusValidation && m.validationShow && len(issues) > 0 {
		sel := issues[clampInt(m.validationCursor, 0, maxInt(0, len(issues)-1))]
		headerLines = append(headerLines, "")
		headerLines = append(headerLines, theme.Title.Render(fmt.Sprintf("Details: %s %s", sel.kind, sel.code)))
		if len(sel.details) == 0 {
			headerLines = append(headerLines, theme.TitleMuted.Render("(no details)"))
		} else {
			j, err := json.MarshalIndent(sel.details, "  ", "  ")
			if err != nil {
				headerLines = append(headerLines, theme.StatusDead.Render(fmt.Sprintf("(failed to render: %v)", err)))
			} else {
				detailLines := strings.Split(string(j), "\n")
				const maxDetailLines = 8
				if len(detailLines) > maxDetailLines {
					detailLines = append(detailLines[:maxDetailLines], "  ...")
				}
				for _, line := range detailLines {
					headerLines = append(headerLines, theme.TitleMuted.Render(line))
				}
			}
		}
	}

	box := widgets.NewBox("Validation").
		WithTitleRight("[v] focus  [enter] details").
		WithContent(lipgloss.JoinVertical(lipgloss.Left, headerLines...)).
		WithSize(m.width, len(headerLines)+3)
	return box.Render()
}

func (m PipelineModel) renderConfigPatches(theme styles.Theme) string {
	var lines []string
	for _, p := range m.configPatches {
		line := lipgloss.JoinHorizontal(lipgloss.Left,
			theme.TitleMuted.Render(" • "),
			theme.Title.Render(p.Key),
			theme.TitleMuted.Render(" → "),
			theme.StatusRunning.Render(p.Value),
			theme.TitleMuted.Render("  ("),
			theme.KeybindKey.Render(p.Plugin),
			theme.TitleMuted.Render(")"),
		)
		lines = append(lines, line)
	}
	box := widgets.NewBox("Applied Config Patches").
		WithTitleRight(fmt.Sprintf("%d patches", len(m.configPatches))).
		WithContent(lipgloss.JoinVertical(lipgloss.Left, lines...)).
		WithSize(m.width, len(lines)+3)
	return box.Render()
}

func (m PipelineModel) renderLiveOutput(theme styles.Theme) string {
	title := "Live Output"
	if len(m.liveOutput) > 0 {
		title = fmt.Sprintf("Live Output (%d lines)", len(m.liveOutput))
	}
	box := widgets.NewBox(title).
		WithTitleRight("[o] toggle").
		WithContent(m.liveVp.View()).
		WithSize(m.width, m.liveVpHeight)
	return box.Render()
}

func (m PipelineModel) moveCursor(delta int) PipelineModel {
	switch m.focus {
	case pipelineFocusBuild:
		m.buildCursor = clampInt(m.buildCursor+delta, 0, maxInt(0, len(m.buildSteps)-1))
	case pipelineFocusPrepare:
		m.prepareCursor = clampInt(m.prepareCursor+delta, 0, maxInt(0, len(m.prepareSteps)-1))
	case pipelineFocusValidation:
		v := m.validate
		issues := validationIssues(v)
		m.validationCursor = clampInt(m.validationCursor+delta, 0, maxInt(0, len(issues)-1))
	}
	return m
}

func (m PipelineModel) toggleDetails() PipelineModel {
	switch m.focus {
	case pipelineFocusBuild:
		m.buildShow = !m.buildShow
	case pipelineFocusPrepare:
		m.prepareShow = !m.prepareShow
	case pipelineFocusValidation:
		m.validationShow = !m.validationShow
	}
	return m
}

type validationIssue struct {
	kind    string
	code    string
	message string
	details map[string]any
}

func validationIssues(v *tui.PipelineValidateResult) []validationIssue {
	if v == nil {
		return nil
	}
	out := make([]validationIssue, 0, len(v.Errors)+len(v.Warnings))
	for _, e := range v.Errors {
		out = append(out, validationIssue{
			kind:    "error",
			code:    e.Code,
			message: e.Message,
			details: e.Details,
		})
	}
	for _, e := range v.Warnings {
		out = append(out, validationIssue{
			kind:    "warn",
			code:    e.Code,
			message: e.Message,
			details: e.Details,
		})
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (m PipelineModel) phase(p tui.PipelinePhase) *pipelinePhaseState {
	if m.phases == nil {
		m.phases = map[tui.PipelinePhase]*pipelinePhaseState{}
	}
	st := m.phases[p]
	if st == nil {
		st = &pipelinePhaseState{}
		m.phases[p] = st
	}
	return st
}

func formatDurationMs(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	sec := float64(d) / float64(time.Second)
	if sec < 10 {
		return fmt.Sprintf("%.1fs", sec)
	}
	return fmt.Sprintf("%.0fs", sec)
}

func formatLiveOutputLine(out tui.PipelineLiveOutput) string {
	prefix := out.Source
	if len(prefix) > 15 {
		prefix = prefix[:15]
	}
	stream := ""
	if out.Stream == "stderr" {
		stream = " (err)"
	}
	return fmt.Sprintf("[%-15s]%s %s", prefix, stream, out.Line)
}

func (m PipelineModel) refreshLiveViewport() PipelineModel {
	if !m.liveVpReady {
		return m
	}
	// Keep only last N lines to prevent unbounded growth
	const maxLines = 500
	if len(m.liveOutput) > maxLines {
		m.liveOutput = m.liveOutput[len(m.liveOutput)-maxLines:]
	}
	content := strings.Join(m.liveOutput, "\n")
	m.liveVp.SetContent(content)
	m.liveVp.GotoBottom()
	return m
}
