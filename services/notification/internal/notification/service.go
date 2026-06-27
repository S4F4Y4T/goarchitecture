package notification

import (
	"context"
	"fmt"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	eventsuser "github.com/s4f4y4t/go-microservice/pkg/events/user"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"github.com/s4f4y4t/go-microservice/pkg/mailer"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type NotificationService struct {
	repo   Repository
	sender mailer.Sender
}

func NewNotificationService(repo Repository, sender mailer.Sender) *NotificationService {
	return &NotificationService{repo: repo, sender: sender}
}

func (s *NotificationService) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]Notification, int64, error) {
	return s.repo.GetAll(ctx, p, opts)
}

func (s *NotificationService) GetByID(ctx context.Context, id int) (*Notification, error) {
	return s.repo.GetByID(ctx, id)
}

// HandleUserCreated is the consumer-facing entry point for the user.created
// event: check-then-send-then-record, idempotent on eventID. A redelivery of
// an event that already produced a row is a no-op, not a resend. On send
// failure it returns the error untouched so the caller's retry/DLQ topology
// decides what happens next — this method never writes a "failed" row
// itself (see RecordFailure, called only once retries are exhausted).
func (s *NotificationService) HandleUserCreated(ctx context.Context, eventID string, payload eventsuser.UserCreatedPayload) error {
	exists, err := s.repo.ExistsByEventID(ctx, eventID)
	if err != nil {
		return err
	}
	if exists {
		logger.FromContext(ctx).Info("notification: event already processed, skipping", "event_id", eventID)
		return nil
	}

	body, err := renderWelcomeEmail(payload.Name)
	if err != nil {
		return fmt.Errorf("rendering welcome email: %w", err)
	}

	if err := s.sender.Send(ctx, mailer.Message{
		To:      payload.Email,
		Subject: "Welcome!",
		Body:    body,
	}); err != nil {
		return fmt.Errorf("sending welcome email: %w", err)
	}

	if _, err := s.repo.Create(ctx, &Notification{
		EventID:   eventID,
		Type:      eventsuser.UserCreated,
		Recipient: payload.Email,
		Status:    StatusSent,
	}); err != nil {
		// A unique-violation here means a concurrent redelivery already
		// recorded this event after our ExistsByEventID check above — the
		// email may have gone out twice, but the audit dedup itself held.
		if apperror.From(err).Code == apperror.CodeConflict {
			return nil
		}
		return err
	}

	return nil
}

// RecordFailure writes an audit row for an event that exhausted its
// retries, called only from the consumer's DLQ-parking path so the failure
// stays visible via the read API even though no email was ever sent.
func (s *NotificationService) RecordFailure(ctx context.Context, eventID, eventType, recipient, reason string) error {
	_, err := s.repo.Create(ctx, &Notification{
		EventID:   eventID,
		Type:      eventType,
		Recipient: recipient,
		Status:    StatusFailed,
		Error:     reason,
	})
	if err != nil && apperror.From(err).Code != apperror.CodeConflict {
		return err
	}
	return nil
}
