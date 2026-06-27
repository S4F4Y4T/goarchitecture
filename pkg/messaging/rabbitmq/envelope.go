package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/middleware"
)

// Envelope is the wire format for every message published to domain_events.
// EventID makes consumption idempotent (a consumer can dedupe redeliveries by
// this value); TraceID reuses the same X-Request-ID correlation convention
// the HTTP/gRPC layers already use, so a registration request's logs and the
// async event it triggers can be grepped together.
type Envelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	TraceID   string          `json:"trace_id"`
	Timestamp time.Time       `json:"timestamp"`
	Version   int             `json:"version"`
}

// NewEnvelope marshals payload and stamps it with a fresh event ID, the
// current request's trace ID (if any), and the current time.
func NewEnvelope(ctx context.Context, eventType string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshaling event payload: %w", err)
	}
	return Envelope{
		EventID:   uuid.NewString(),
		EventType: eventType,
		Payload:   raw,
		TraceID:   middleware.GetRequestID(ctx),
		Timestamp: time.Now().UTC(),
		Version:   1,
	}, nil
}
