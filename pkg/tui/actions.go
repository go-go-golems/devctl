package tui

import (
	"encoding/json"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"
)

type ActionKind string

const (
	ActionUp      ActionKind = "up"
	ActionDown    ActionKind = "down"
	ActionRestart ActionKind = "restart"
)

type ActionRequest struct {
	Kind ActionKind `json:"kind"`
	At   time.Time  `json:"at"`
}

func PublishAction(pub message.Publisher, req ActionRequest) error {
	if pub == nil {
		return errors.New("missing publisher")
	}
	if req.Kind == "" {
		return errors.New("missing action kind")
	}
	if req.At.IsZero() {
		req.At = time.Now()
	}

	env, err := NewEnvelope(UITypeActionRequest, req)
	if err != nil {
		return err
	}
	b, err := env.MarshalJSONBytes()
	if err != nil {
		return err
	}
	_ = json.Valid(b)
	return pub.Publish(TopicUIActions, message.NewMessage(watermill.NewUUID(), b))
}
