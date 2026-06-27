package mailer

import "context"

// Message is a single outbound email. Body is pre-rendered HTML — mailer is
// transport only, templating is the caller's business logic.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender is the outbound port any service that needs to send email depends
// on. SMTPSender is the only implementation today; tests can provide a fake.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
