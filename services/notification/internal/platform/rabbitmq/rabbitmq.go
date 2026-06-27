package rabbitmq

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/s4f4y4t/go-microservice/pkg/messaging/rabbitmq"
	"github.com/s4f4y4t/go-microservice/services/notification/internal/config"
)

// Open dials RabbitMQ and fails fast if the broker is unreachable, the same
// startup contract internal/platform/database.Open follows.
func Open(cfg config.RabbitMQConfig) (*amqp.Connection, error) {
	return rabbitmq.Dial(cfg.URL)
}
