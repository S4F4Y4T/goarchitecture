package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler processes one event. Returning an error leaves the delivery
// unacknowledged so Consume can retry it, rather than acknowledging a
// message whose side effect (e.g. sending an email) never happened.
type Handler func(ctx context.Context, env Envelope) error

// OnExhausted, if non-nil, is invoked once a delivery has exhausted
// maxRetries, right before it is parked in the DLQ — giving the caller a
// chance to record the terminal failure (e.g. an audit row) using the
// decoded envelope and the last handler error. This package has no opinion
// on what that record looks like; it only reports that retries ran out.
type OnExhausted func(ctx context.Context, env Envelope, lastErr error)

// Consumer consumes a single queue with manual acknowledgement.
type Consumer struct {
	ch *amqp.Channel
}

// NewConsumer opens a channel on conn and caps in-flight deliveries to a
// sane prefetch so one slow handler can't starve the others on the channel.
func NewConsumer(conn *amqp.Connection) (*Consumer, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("opening consumer channel: %w", err)
	}
	if err := ch.Qos(10, 0, false); err != nil {
		return nil, fmt.Errorf("setting consumer prefetch: %w", err)
	}
	return &Consumer{ch: ch}, nil
}

// Consume runs h for every delivery on queue until ctx is cancelled or the
// channel closes. queue must already have the retry/DLQ topology declared
// on it via DeclareConsumerTopology — Consume relies on the conventions that
// sets up (RetryQueueName, DLQQueueName).
//
// A successful h Acks the delivery. A failing h either Nacks it — which,
// given that topology, bounces it through the retry queue for RetryTTL
// before redelivery — or, once it has already cycled through the retry
// queue maxRetries times (read from the delivery's x-death history), parks
// a copy on DLQQueueName(queue) and Acks the original. This guarantees a
// permanently failing message is never retried forever and never silently
// dropped — it ends up visible in the DLQ for manual inspection.
func (c *Consumer) Consume(ctx context.Context, queue string, maxRetries int, h Handler, onExhausted OnExhausted) error {
	deliveries, err := c.ch.ConsumeWithContext(ctx, queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("registering consumer on %s: %w", queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return nil
			}
			c.handle(ctx, queue, maxRetries, d, h, onExhausted)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, queue string, maxRetries int, d amqp.Delivery, h Handler, onExhausted OnExhausted) {
	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		slog.Error("messaging: dropping undecodable delivery", "queue", queue, "error", err)
		_ = d.Ack(false)
		return
	}

	err := h(ctx, env)
	if err == nil {
		if ackErr := d.Ack(false); ackErr != nil {
			slog.Error("messaging: ack failed", "queue", queue, "event_id", env.EventID, "error", ackErr)
		}
		return
	}

	slog.Error("messaging: handler failed", "queue", queue, "event_id", env.EventID, "event_type", env.EventType, "error", err)

	if retriesSoFar(d, RetryQueueName(queue)) < maxRetries {
		if nackErr := d.Nack(false, false); nackErr != nil {
			slog.Error("messaging: nack failed", "queue", queue, "event_id", env.EventID, "error", nackErr)
		}
		return
	}

	if onExhausted != nil {
		onExhausted(ctx, env, err)
	}
	c.park(ctx, queue, d, env)
}

// park gives up retrying and republishes the delivery to its DLQ queue
// before acking the original. Publishing to the default ("") exchange with
// the queue name as routing key delivers directly to that named queue.
// This is two non-atomic steps (publish, then ack) — a crash between them
// can produce a duplicate DLQ entry, which is harmless here because
// downstream processing is idempotent on event_id (see docs/messaging.md
// Known Gaps).
func (c *Consumer) park(ctx context.Context, queue string, d amqp.Delivery, env Envelope) {
	dlq := DLQQueueName(queue)
	err := c.ch.PublishWithContext(ctx, "", dlq, false, false, amqp.Publishing{
		ContentType:  d.ContentType,
		DeliveryMode: amqp.Persistent,
		MessageId:    env.EventID,
		Timestamp:    env.Timestamp,
		Type:         env.EventType,
		Body:         d.Body,
	})
	if err != nil {
		slog.Error("messaging: failed to park exhausted delivery in dlq, requeuing instead", "queue", queue, "dlq", dlq, "event_id", env.EventID, "error", err)
		if nackErr := d.Nack(false, true); nackErr != nil {
			slog.Error("messaging: requeue-after-failed-park also failed", "queue", queue, "event_id", env.EventID, "error", nackErr)
		}
		return
	}

	slog.Error("messaging: exhausted retries, parked in dlq", "queue", queue, "dlq", dlq, "event_id", env.EventID, "event_type", env.EventType)
	if ackErr := d.Ack(false); ackErr != nil {
		slog.Error("messaging: ack after dlq park failed", "queue", queue, "event_id", env.EventID, "error", ackErr)
	}
}

// retriesSoFar reads how many times the delivery has already cycled through
// retryQueue by inspecting the x-death header RabbitMQ attaches on every
// dead-lettering. Each full retry cycle (Nack -> retry queue -> TTL expiry
// -> back to the main queue) increments the "expired" entry for retryQueue
// by exactly one, so this count is the number of retries already attempted
// — not the number of total delivery attempts.
func retriesSoFar(d amqp.Delivery, retryQueue string) int {
	raw, ok := d.Headers["x-death"]
	if !ok {
		return 0
	}
	deaths, ok := raw.([]any)
	if !ok {
		return 0
	}
	for _, entry := range deaths {
		death, ok := entry.(amqp.Table)
		if !ok {
			continue
		}
		if queue, _ := death["queue"].(string); queue != retryQueue {
			continue
		}
		switch count := death["count"].(type) {
		case int64:
			return int(count)
		case int32:
			return int(count)
		case int:
			return int(count)
		}
	}
	return 0
}
