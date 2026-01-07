package tui

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/errors"
)

func RegisterUIForwarder(bus *Bus, p *tea.Program) {
	bus.AddHandler("devctl-ui-forward", TopicUIMessages, func(msg *message.Message) error {
		defer msg.Ack()

		var env Envelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil {
			return errors.Wrap(err, "unmarshal ui envelope")
		}

		switch env.Type {
		case UITypeStateSnapshot:
			var snap StateSnapshot
			if err := json.Unmarshal(env.Payload, &snap); err != nil {
				return errors.Wrap(err, "unmarshal snapshot payload")
			}
			p.Send(StateSnapshotMsg{Snapshot: snap})
		case UITypeEventAppend:
			var entry EventLogEntry
			if err := json.Unmarshal(env.Payload, &entry); err != nil {
				return errors.Wrap(err, "unmarshal event payload")
			}
			p.Send(EventLogAppendMsg{Entry: entry})
		case UITypePipelineRunStarted:
			var ev PipelineRunStarted
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline run started payload")
			}
			p.Send(PipelineRunStartedMsg{Run: ev})
		case UITypePipelineRunFinished:
			var ev PipelineRunFinished
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline run finished payload")
			}
			p.Send(PipelineRunFinishedMsg{Run: ev})
		case UITypePipelinePhaseStarted:
			var ev PipelinePhaseStarted
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline phase started payload")
			}
			p.Send(PipelinePhaseStartedMsg{Event: ev})
		case UITypePipelinePhaseFinished:
			var ev PipelinePhaseFinished
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline phase finished payload")
			}
			p.Send(PipelinePhaseFinishedMsg{Event: ev})
		case UITypePipelineBuildResult:
			var ev PipelineBuildResult
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline build result payload")
			}
			p.Send(PipelineBuildResultMsg{Result: ev})
		case UITypePipelinePrepareResult:
			var ev PipelinePrepareResult
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline prepare result payload")
			}
			p.Send(PipelinePrepareResultMsg{Result: ev})
		case UITypePipelineValidateResult:
			var ev PipelineValidateResult
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline validate result payload")
			}
			p.Send(PipelineValidateResultMsg{Result: ev})
		case UITypePipelineLaunchPlan:
			var ev PipelineLaunchPlan
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline launch plan payload")
			}
			p.Send(PipelineLaunchPlanMsg{Plan: ev})
		}
		return nil
	})
}
