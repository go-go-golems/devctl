package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/tui"
)

type PipelineModel struct {
	width  int
	height int

	runStarted  *tui.PipelineRunStarted
	runFinished *tui.PipelineRunFinished

	phaseOrder []tui.PipelinePhase
	phases     map[tui.PipelinePhase]*pipelinePhaseState

	buildSteps   []tui.PipelineStepResult
	prepareSteps []tui.PipelineStepResult
	validate     *tui.PipelineValidateResult
	launchPlan   *tui.PipelineLaunchPlan

	validationCursor int
	validationShow   bool
}

type pipelinePhaseState struct {
	startedAt  time.Time
	finishedAt time.Time
	ok         *bool
	durationMs int64
	errText    string
}

func NewPipelineModel() PipelineModel {
	return PipelineModel{
		phases: map[tui.PipelinePhase]*pipelinePhaseState{},
	}
}

func (m PipelineModel) WithSize(width, height int) PipelineModel {
	m.width, m.height = width, height
	return m
}

func (m PipelineModel) Update(msg tea.Msg) (PipelineModel, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "up", "k":
			m.validationCursor--
			if m.validationCursor < 0 {
				m.validationCursor = 0
			}
			return m, nil
		case "down", "j":
			m.validationCursor++
			return m, nil
		case "enter":
			m.validationShow = !m.validationShow
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
		m.validate = nil
		m.launchPlan = nil
		m.validationCursor = 0
		m.validationShow = false

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
		return m, nil
	case tui.PipelinePrepareResultMsg:
		if m.runStarted == nil || m.runStarted.RunID != v.Result.RunID {
			return m, nil
		}
		m.prepareSteps = append([]tui.PipelineStepResult{}, v.Result.Steps...)
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
	default:
		return m, nil
	}
}

func (m PipelineModel) View() string {
	if m.runStarted == nil {
		return "No pipeline run recorded yet.\n\nRun `u` (up) or `r` (restart) from the dashboard to see progress here.\n"
	}

	run := m.runStarted
	status := "running"
	if m.runFinished != nil {
		if m.runFinished.Ok {
			status = "ok"
		} else {
			status = "failed"
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Pipeline: %s  run=%s  (%s)\n", run.Kind, run.RunID, status))
	b.WriteString(fmt.Sprintf("Started: %s\n\n", run.At.Format("2006-01-02 15:04:05")))

	b.WriteString("Phases:\n")
	for _, p := range m.phaseOrder {
		st := m.phases[p]
		b.WriteString(fmt.Sprintf("- %s: %s\n", p, formatPhaseState(st)))
	}
	b.WriteString("\n")

	if len(m.buildSteps) > 0 {
		b.WriteString("Build steps:\n")
		for _, s := range m.buildSteps {
			b.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, formatStep(s)))
		}
		b.WriteString("\n")
	}

	if len(m.prepareSteps) > 0 {
		b.WriteString("Prepare steps:\n")
		for _, s := range m.prepareSteps {
			b.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, formatStep(s)))
		}
		b.WriteString("\n")
	}

	if m.validate != nil {
		v := m.validate
		if v.Valid {
			b.WriteString(fmt.Sprintf("Validate: ok (%d warnings)\n\n", len(v.Warnings)))
		} else {
			b.WriteString(fmt.Sprintf("Validate: failed (%d errors, %d warnings)\n", len(v.Errors), len(v.Warnings)))
			if len(v.Errors) > 0 {
				first := v.Errors[0]
				b.WriteString(fmt.Sprintf("First error: %s: %s\n\n", first.Code, first.Message))
			} else {
				b.WriteString("\n")
			}
		}

		issues := validationIssues(v)
		if len(issues) > 0 {
			if m.validationCursor >= len(issues) {
				m.validationCursor = len(issues) - 1
			}
			if m.validationCursor < 0 {
				m.validationCursor = 0
			}

			b.WriteString("Validation issues (↑/↓ select, enter toggle details):\n")
			for i, is := range issues {
				cursor := " "
				if i == m.validationCursor {
					cursor = ">"
				}
				b.WriteString(fmt.Sprintf("%s %s %s: %s\n", cursor, is.kind, is.code, is.message))
			}
			b.WriteString("\n")

			if m.validationShow {
				sel := issues[m.validationCursor]
				b.WriteString(fmt.Sprintf("Details: %s %s\n", sel.kind, sel.code))
				if sel.details == nil || len(sel.details) == 0 {
					b.WriteString("(no details)\n\n")
				} else {
					j, err := json.MarshalIndent(sel.details, "", "  ")
					if err != nil {
						b.WriteString(fmt.Sprintf("(failed to render details: %v)\n\n", err))
					} else {
						lines := strings.Split(string(j), "\n")
						const maxLines = 12
						if len(lines) > maxLines {
							lines = append(lines[:maxLines], "  ...")
						}
						for _, line := range lines {
							b.WriteString(line)
							b.WriteString("\n")
						}
						b.WriteString("\n")
					}
				}
			}
		}
	}

	if m.launchPlan != nil {
		b.WriteString(fmt.Sprintf("Launch plan: %d services\n", len(m.launchPlan.Services)))
		if len(m.launchPlan.Services) > 0 {
			b.WriteString(fmt.Sprintf("Services: %s\n", strings.Join(m.launchPlan.Services, ", ")))
		}
		b.WriteString("\n")
	}

	if m.runFinished != nil {
		f := m.runFinished
		if f.DurationMs > 0 {
			b.WriteString(fmt.Sprintf("Total: %s\n", formatDurationMs(f.DurationMs)))
		}
		if !f.Ok && f.Error != "" {
			b.WriteString(fmt.Sprintf("Error: %s\n", f.Error))
		}
	}

	return b.String()
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

func formatPhaseState(st *pipelinePhaseState) string {
	if st == nil {
		return "pending"
	}
	if st.ok == nil && !st.startedAt.IsZero() {
		return "running"
	}
	if st.ok == nil {
		return "pending"
	}
	state := "ok"
	if !*st.ok {
		state = "failed"
	}
	if st.durationMs > 0 {
		state = fmt.Sprintf("%s (%s)", state, formatDurationMs(st.durationMs))
	}
	if st.errText != "" && !*st.ok {
		state = fmt.Sprintf("%s: %s", state, st.errText)
	}
	return state
}

func formatStep(s tui.PipelineStepResult) string {
	state := "ok"
	if !s.Ok {
		state = "failed"
	}
	if s.DurationMs > 0 {
		state = fmt.Sprintf("%s (%s)", state, formatDurationMs(s.DurationMs))
	}
	return state
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
