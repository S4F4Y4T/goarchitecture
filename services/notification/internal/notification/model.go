package notification

import (
	"context"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type Notification struct {
	ID        int       `json:"id"`
	EventID   string    `json:"event_id"`
	Type      string    `json:"type"`
	Recipient string    `json:"recipient"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	StatusSent   = "sent"
	StatusFailed = "failed"
)

var ListSchema = query.Schema{
	"id":         {Column: "id", Sortable: true, Filterable: true},
	"type":       {Column: "type", Sortable: true, Filterable: true},
	"recipient":  {Column: "recipient", Sortable: true, Filterable: true, Partial: true},
	"status":     {Column: "status", Sortable: true, Filterable: true},
	"created_at": {Column: "created_at", Sortable: true},
	"updated_at": {Column: "updated_at", Sortable: true},
}

// Repository persists notification audit rows. Create doubles as the
// idempotency gate: a unique violation on event_id means this event was
// already processed, which the implementation must surface as a sentinel
// the service layer can recognize rather than a hard failure (see
// repository.go's isUniqueViolation, the same 23505 convention
// services/user/internal/user/repository.go uses).
type Repository interface {
	Create(ctx context.Context, n *Notification) (*Notification, error)
	ExistsByEventID(ctx context.Context, eventID string) (bool, error)
	GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]Notification, int64, error)
	GetByID(ctx context.Context, id int) (*Notification, error)
}
