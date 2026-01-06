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

		switch env.Type {
		case DomainTypeStateSnapshot:
			var snap StateSnapshot
			if err := json.Unmarshal(env.Payload, &snap); err != nil {
				return errors.Wrap(err, "unmarshal state snapshot")
			}

			uiEnv, err := NewEnvelope(UITypeStateSnapshot, snap)
			if err != nil {
				return err
			}
			uiBytes, err := uiEnv.MarshalJSONBytes()
			if err != nil {
				return err
			}
			if err := bus.Publisher.Publish(TopicUIMessages, message.NewMessage(watermill.NewUUID(), uiBytes)); err != nil {
				return errors.Wrap(err, "publish ui snapshot")
			}

			text := "state: missing"
			if snap.Exists {
				text = "state: loaded"
				if snap.Error != "" {
					text = "state: error"
				}
			}
			entry := EventLogEntry{At: time.Now(), Text: text}
			evEnv, err := NewEnvelope(UITypeEventAppend, entry)
			if err != nil {
				return err
			}
			evBytes, err := evEnv.MarshalJSONBytes()
			if err != nil {
				return err
			}
			if err := bus.Publisher.Publish(TopicUIMessages, message.NewMessage(watermill.NewUUID(), evBytes)); err != nil {
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
			entry := EventLogEntry{At: ev.When, Text: text}
			evEnv, err := NewEnvelope(UITypeEventAppend, entry)
			if err != nil {
				return err
			}
			evBytes, err := evEnv.MarshalJSONBytes()
			if err != nil {
				return err
			}
			if err := bus.Publisher.Publish(TopicUIMessages, message.NewMessage(watermill.NewUUID(), evBytes)); err != nil {
				return errors.Wrap(err, "publish ui event")
			}
			return nil
		case DomainTypeActionLog:
			var logEv ActionLog
			if err := json.Unmarshal(env.Payload, &logEv); err != nil {
				return errors.Wrap(err, "unmarshal action log")
			}

			entry := EventLogEntry{At: logEv.At, Text: logEv.Text}
			evEnv, err := NewEnvelope(UITypeEventAppend, entry)
			if err != nil {
				return err
			}
			evBytes, err := evEnv.MarshalJSONBytes()
			if err != nil {
				return err
			}
			if err := bus.Publisher.Publish(TopicUIMessages, message.NewMessage(watermill.NewUUID(), evBytes)); err != nil {
				return errors.Wrap(err, "publish ui event")
			}
			return nil

		default:
			return nil
		}
	})
}
