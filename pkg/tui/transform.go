package tui

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"
)

func RegisterDomainToUITransformer(bus *Bus) {
	bus.AddHandler("devctl-domain-to-ui", TopicDevctlEvents, func(msg *message.Message) error {
		defer msg.Ack()

		var env Envelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil {
			return errors.Wrap(err, "unmarshal domain envelope")
		}

		publishUI := func(uiType string, payload any) error {
			uiEnv, err := NewEnvelope(uiType, payload)
			if err != nil {
				return err
			}
			uiBytes, err := uiEnv.MarshalJSONBytes()
			if err != nil {
				return err
			}
			if err := bus.Publisher.Publish(TopicUIMessages, message.NewMessage(watermill.NewUUID(), uiBytes)); err != nil {
				return errors.Wrap(err, "publish ui message")
			}
			return nil
		}

		publishEventText := func(at time.Time, source string, level LogLevel, text string) error {
			entry := EventLogEntry{At: at, Source: source, Level: level, Text: text}
			return publishUI(UITypeEventAppend, entry)
		}

		switch env.Type {
		case DomainTypeStateSnapshot:
			var snap StateSnapshot
			if err := json.Unmarshal(env.Payload, &snap); err != nil {
				return errors.Wrap(err, "unmarshal state snapshot")
			}

			if err := publishUI(UITypeStateSnapshot, snap); err != nil {
				return errors.Wrap(err, "publish ui snapshot")
			}

			text := "state: missing"
			level := LogLevelInfo
			if snap.Exists {
				text = "state: loaded"
				if snap.Error != "" {
					text = "state: error"
					level = LogLevelWarn
				}
			}
			if err := publishEventText(time.Now(), "system", level, text); err != nil {
				return errors.Wrap(err, "publish ui event")
			}
			return nil
		case DomainTypeServiceExit:
			var ev ServiceExitObserved
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal service exit")
			}

			text := fmt.Sprintf("service exit: %s pid=%d", ev.Name, ev.PID)
			if ev.Reason != "" {
				text = fmt.Sprintf("%s (%s)", text, ev.Reason)
			}
			if err := publishEventText(ev.When, ev.Name, LogLevelWarn, text); err != nil {
				return errors.Wrap(err, "publish ui event")
			}
			return nil
		case DomainTypeActionLog:
			var logEv ActionLog
			if err := json.Unmarshal(env.Payload, &logEv); err != nil {
				return errors.Wrap(err, "unmarshal action log")
			}

			if err := publishEventText(logEv.At, "system", LogLevelInfo, logEv.Text); err != nil {
				return errors.Wrap(err, "publish ui event")
			}
			return nil
		case DomainTypePipelineRunStarted:
			var ev PipelineRunStarted
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline run started")
			}
			if err := publishUI(UITypePipelineRunStarted, ev); err != nil {
				return err
			}
			return publishEventText(ev.At, "pipeline", LogLevelInfo, fmt.Sprintf("pipeline: started (%s)", ev.Kind))
		case DomainTypePipelineRunFinished:
			var ev PipelineRunFinished
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline run finished")
			}
			if err := publishUI(UITypePipelineRunFinished, ev); err != nil {
				return err
			}
			if ev.Ok {
				return publishEventText(ev.At, "pipeline", LogLevelInfo, fmt.Sprintf("pipeline: ok (%s)", ev.Kind))
			}
			text := fmt.Sprintf("pipeline: failed (%s)", ev.Kind)
			if ev.Error != "" {
				text = fmt.Sprintf("%s: %s", text, ev.Error)
			}
			return publishEventText(ev.At, "pipeline", LogLevelError, text)
		case DomainTypePipelinePhaseStarted:
			var ev PipelinePhaseStarted
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline phase started")
			}
			return publishUI(UITypePipelinePhaseStarted, ev)
		case DomainTypePipelinePhaseFinished:
			var ev PipelinePhaseFinished
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline phase finished")
			}
			return publishUI(UITypePipelinePhaseFinished, ev)
		case DomainTypePipelineBuildResult:
			var ev PipelineBuildResult
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline build result")
			}
			return publishUI(UITypePipelineBuildResult, ev)
		case DomainTypePipelinePrepareResult:
			var ev PipelinePrepareResult
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline prepare result")
			}
			return publishUI(UITypePipelinePrepareResult, ev)
		case DomainTypePipelineValidateResult:
			var ev PipelineValidateResult
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline validate result")
			}
			return publishUI(UITypePipelineValidateResult, ev)
		case DomainTypePipelineLaunchPlan:
			var ev PipelineLaunchPlan
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal pipeline launch plan")
			}
			return publishUI(UITypePipelineLaunchPlan, ev)

		case DomainTypeStreamStarted:
			var ev StreamStarted
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal stream started")
			}
			if err := publishUI(UITypeStreamStarted, ev); err != nil {
				return err
			}
			return publishEventText(ev.At, "streams", LogLevelInfo, fmt.Sprintf("stream started: %s (plugin=%s)", ev.Op, ev.PluginID))
		case DomainTypeStreamEvent:
			var ev StreamEvent
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal stream event")
			}
			// Intentionally do not echo every stream event into the global event log: telemetry can be high-frequency.
			return publishUI(UITypeStreamEvent, ev)
		case DomainTypeStreamEnded:
			var ev StreamEnded
			if err := json.Unmarshal(env.Payload, &ev); err != nil {
				return errors.Wrap(err, "unmarshal stream ended")
			}
			if err := publishUI(UITypeStreamEnded, ev); err != nil {
				return err
			}
			level := LogLevelInfo
			text := fmt.Sprintf("stream ended: %s (plugin=%s)", ev.Op, ev.PluginID)
			if !ev.Ok {
				level = LogLevelError
				if ev.Error != "" {
					text = fmt.Sprintf("%s: %s", text, ev.Error)
				}
			}
			return publishEventText(ev.At, "streams", level, text)

		default:
			return nil
		}
	})
}
