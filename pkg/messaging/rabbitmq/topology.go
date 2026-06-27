package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TopologySpec describes the exchange/queue/retry/DLQ wiring for one
// consumer. Exchange is the topic exchange events are published to;
// RoutingKey is the event type this queue should receive; QueueName is this
// consumer's own queue — even if two different consumers bind the same
// RoutingKey on the same Exchange, each needs its own QueueName (and
// therefore its own retry/DLQ queues), since one consumer's failures must
// not be retried against the other's.
type TopologySpec struct {
	Exchange   string
	RoutingKey string
	QueueName  string
	RetryTTL   time.Duration
	MaxRetries int
}

// DeclareConsumerTopology sets up the standard "retry via per-queue TTL +
// dead-letter-exchange bounce-back" pattern, RabbitMQ's usual workaround for
// delayed redelivery without the rabbitmq-delayed-message-exchange plugin
// (deliberately not used here — one fewer moving part):
//
//	Exchange (topic) --[RoutingKey]--> QueueName
//	    Nack(requeue=false) dead-letters to QueueName.retry.exchange
//	        --[RoutingKey]--> QueueName.retry (x-message-ttl=RetryTTL)
//	            TTL expiry dead-letters back to Exchange --[RoutingKey]--> QueueName
//	QueueName.dlq -- parking queue Consumer publishes to once a message has
//	    bounced through the retry cycle MaxRetries times (see consumer.go)
//
// Retries are a flat delay, not an exponential backoff ladder — see
// docs/messaging.md Known Gaps. Safe to call repeatedly; every declare is
// idempotent.
func DeclareConsumerTopology(ch *amqp.Channel, spec TopologySpec) error {
	retryExchange := spec.QueueName + ".retry.exchange"
	retryQueue := RetryQueueName(spec.QueueName)
	dlqQueue := DLQQueueName(spec.QueueName)

	if err := ch.ExchangeDeclare(spec.Exchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring exchange %s: %w", spec.Exchange, err)
	}
	if err := ch.ExchangeDeclare(retryExchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring retry exchange %s: %w", retryExchange, err)
	}

	if _, err := ch.QueueDeclare(spec.QueueName, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange": retryExchange,
	}); err != nil {
		return fmt.Errorf("declaring queue %s: %w", spec.QueueName, err)
	}
	if err := ch.QueueBind(spec.QueueName, spec.RoutingKey, spec.Exchange, false, nil); err != nil {
		return fmt.Errorf("binding queue %s to %s: %w", spec.QueueName, spec.Exchange, err)
	}

	if _, err := ch.QueueDeclare(retryQueue, true, false, false, false, amqp.Table{
		"x-message-ttl":             int(spec.RetryTTL / time.Millisecond),
		"x-dead-letter-exchange":    spec.Exchange,
		"x-dead-letter-routing-key": spec.RoutingKey,
	}); err != nil {
		return fmt.Errorf("declaring retry queue %s: %w", retryQueue, err)
	}
	if err := ch.QueueBind(retryQueue, spec.RoutingKey, retryExchange, false, nil); err != nil {
		return fmt.Errorf("binding retry queue %s to %s: %w", retryQueue, retryExchange, err)
	}

	if _, err := ch.QueueDeclare(dlqQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring dlq %s: %w", dlqQueue, err)
	}

	return nil
}

// RetryQueueName returns the retry queue name DeclareConsumerTopology
// derives from a consumer's main queue name — exported so Consumer can
// recognize it when inspecting a delivery's x-death history.
func RetryQueueName(queueName string) string { return queueName + ".retry" }

// DLQQueueName returns the parking-queue name DeclareConsumerTopology
// derives from a consumer's main queue name.
func DLQQueueName(queueName string) string { return queueName + ".dlq" }
