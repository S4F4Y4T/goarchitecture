package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes Envelopes to a topic exchange in confirm mode, so
// Publish only returns success once the broker has actually accepted the
// message — not merely once it left the client's TCP buffer.
type Publisher struct {
	ch *amqp.Channel
}

// NewPublisher opens a channel on conn and switches it into confirm mode.
func NewPublisher(conn *amqp.Connection) (*Publisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("opening publisher channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("enabling publisher confirms: %w", err)
	}
	return &Publisher{ch: ch}, nil
}

// DeclareExchange idempotently declares a durable topic exchange. Safe to
// call repeatedly (e.g. once per publisher at startup).
func (p *Publisher) DeclareExchange(name string) error {
	return p.ch.ExchangeDeclare(name, amqp.ExchangeTopic, true, false, false, false, nil)
}

// Publish marshals env and publishes it to exchange/routingKey as a
// persistent message, waiting for the broker's confirmation before
// returning.
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, env Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    env.EventID,
		Timestamp:    env.Timestamp,
		Type:         env.EventType,
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publishing event %s: %w", env.EventType, err)
	}

	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("waiting for broker confirmation of event %s: %w", env.EventType, err)
	}
	if !ok {
		return fmt.Errorf("broker nacked event %s (event_id=%s)", env.EventType, env.EventID)
	}
	return nil
}

func (p *Publisher) Close() error {
	return p.ch.Close()
}
