package tui

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"
)

func PublishStreamStart(pub message.Publisher, req StreamStartRequest) error {
	if pub == nil {
		return errors.New("missing publisher")
	}
	if req.Op == "" {
		return errors.New("missing stream op")
	}

	env, err := NewEnvelope(UITypeStreamStartRequest, req)
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

func PublishStreamStop(pub message.Publisher, req StreamStopRequest) error {
	if pub == nil {
		return errors.New("missing publisher")
	}
	if req.StreamKey == "" {
		return errors.New("missing stream key")
	}

	env, err := NewEnvelope(UITypeStreamStopRequest, req)
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
