package messaging

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/s4f4y4t/go-microservice/pkg/messaging/rabbitmq"

	eventsuser "github.com/s4f4y4t/go-microservice/pkg/events/user"
	"github.com/s4f4y4t/go-microservice/services/user/internal/user"
)

// EventPublisher adapts pkg/messaging/rabbitmq.Publisher to user.EventPublisher
// so UserService stays unaware that messaging is involved.
type EventPublisher struct {
	pub      *rabbitmq.Publisher
	exchange string
}

// NewEventPublisher declares the durable topic exchange events are
// published to and returns an adapter ready to publish on it.
func NewEventPublisher(conn *amqp.Connection, exchange string) (*EventPublisher, error) {
	pub, err := rabbitmq.NewPublisher(conn)
	if err != nil {
		return nil, err
	}
	if err := pub.DeclareExchange(exchange); err != nil {
		return nil, err
	}
	return &EventPublisher{pub: pub, exchange: exchange}, nil
}

func (e *EventPublisher) PublishUserCreated(ctx context.Context, u *user.User) error {
	env, err := rabbitmq.NewEnvelope(ctx, eventsuser.UserCreated, eventsuser.UserCreatedPayload{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
	})
	if err != nil {
		return err
	}
	return e.pub.Publish(ctx, e.exchange, eventsuser.UserCreated, env)
}
