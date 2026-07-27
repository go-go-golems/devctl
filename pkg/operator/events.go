package operator

import (
	"context"
	"time"
)

const EventVersion = 1

type EventKind string

const (
	EventOperationStarted  EventKind = "operation.started"
	EventOperationFinished EventKind = "operation.finished"
	EventPhaseStarted      EventKind = "phase.started"
	EventPhaseFinished     EventKind = "phase.finished"
	EventServicePlanned    EventKind = "service.planned"
	EventServiceStarting   EventKind = "service.starting"
	EventServiceReady      EventKind = "service.ready"
	EventServiceUnhealthy  EventKind = "service.unhealthy"
	EventServiceStopping   EventKind = "service.stopping"
	EventServiceExited     EventKind = "service.exited"
	EventServiceFailed     EventKind = "service.failed"
	EventServiceUnknown    EventKind = "service.unknown"
	EventDiagnostic        EventKind = "diagnostic"
)

type OperatorEvent struct {
	Version     int            `json:"version"`
	OperationID string         `json:"operation_id"`
	At          time.Time      `json:"at"`
	Kind        EventKind      `json:"kind"`
	Phase       string         `json:"phase,omitempty"`
	Service     string         `json:"service,omitempty"`
	Status      string         `json:"status,omitempty"`
	Message     string         `json:"message,omitempty"`
	Error       *OperatorError `json:"error,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
}

type EventSink interface {
	Send(context.Context, OperatorEvent) error
}

type NopEventSink struct{}

var _ EventSink = NopEventSink{}

func (NopEventSink) Send(context.Context, OperatorEvent) error {
	return nil
}
