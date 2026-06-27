package notification

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	gormquery "github.com/s4f4y4t/go-microservice/pkg/query/gorm"
	"gorm.io/gorm"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) Repository {
	return &NotificationRepository{db: db}
}

// Create returns apperror.Conflict if event_id already exists — the caller
// (NotificationService) treats that as "already processed", not an error.
func (r *NotificationRepository) Create(ctx context.Context, n *Notification) (*Notification, error) {
	if err := r.db.WithContext(ctx).Create(n).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, apperror.Conflict("event already processed")
		}
		return nil, apperror.Internal(err)
	}
	return n, nil
}

func (r *NotificationRepository) ExistsByEventID(ctx context.Context, eventID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Notification{}).Where("event_id = ?", eventID).Count(&count).Error; err != nil {
		return false, apperror.Internal(err)
	}
	return count > 0, nil
}

func (r *NotificationRepository) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]Notification, int64, error) {
	var (
		notifications []Notification
		total         int64
	)
	if err := r.db.WithContext(ctx).Model(&Notification{}).Scopes(gormquery.Filters(opts)).Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	if err := r.db.WithContext(ctx).Model(&Notification{}).
		Scopes(gormquery.Filters(opts), gormquery.Sorts(opts)).
		Offset(p.Offset()).Limit(p.Limit).Find(&notifications).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return notifications, total, nil
}

func (r *NotificationRepository) GetByID(ctx context.Context, id int) (*Notification, error) {
	var n Notification
	if err := r.db.WithContext(ctx).First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("notification not found with id " + strconv.Itoa(id))
		}
		return nil, apperror.Internal(err)
	}
	return &n, nil
}
