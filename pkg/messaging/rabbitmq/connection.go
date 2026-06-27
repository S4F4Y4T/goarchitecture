package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Dial opens a single AMQP connection and fails fast if the broker is
// unreachable, the same fail-fast-at-startup contract every other
// platform dependency in this codebase follows (see
// services/*/internal/platform/{database,redis}). There is no automatic
// reconnect loop yet — see docs/messaging.md Known Gaps.
func Dial(url string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}
	return conn, nil
}
