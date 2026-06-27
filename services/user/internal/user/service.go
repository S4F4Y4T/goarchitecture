package user

import (
	"context"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type UserService struct {
	repo      Repository
	publisher EventPublisher
}

func NewUserService(repo Repository, publisher EventPublisher) *UserService {
	return &UserService{repo: repo, publisher: publisher}
}

// create inserts u and publishes UserCreated on success. This is the single
// place both the HTTP and gRPC creation paths funnel through, so the event
// fires exactly once regardless of which API was used to register the user.
// The publish happens strictly after the insert has committed (Create here
// is a single-statement, non-transactional insert) and is best-effort: a
// publish failure is logged but never fails the request, since the user row
// is the source of truth and a lost notification is recoverable while a
// failed registration is not. See docs/messaging.md Known Gaps for the
// at-most-once tradeoff this implies.
func (s *UserService) create(ctx context.Context, u *User) (*User, error) {
	created, err := s.repo.Create(ctx, u)
	if err != nil {
		return nil, err
	}
	if err := s.publisher.PublishUserCreated(ctx, created); err != nil {
		logger.FromContext(ctx).Error("publishing user.created event", "error", err, "user_id", created.ID)
	}
	return created, nil
}

func (s *UserService) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error) {
	return s.repo.GetAll(ctx, p, opts)
}

func (s *UserService) GetByID(ctx context.Context, id int) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("email already exists")
	}

	return s.create(ctx, &User{Name: req.Name, Email: req.Email})
}

// CreateWithPassword is used by the gRPC Create RPC (the path auth.Register
// calls), which already carries a bcrypt-hashed password and has already
// performed its own ExistsByEmail check before calling Create.
func (s *UserService) CreateWithPassword(ctx context.Context, name, email, password string) (*User, error) {
	return s.create(ctx, &User{Name: name, Email: email, Password: password})
}

func (s *UserService) Update(ctx context.Context, id int, req UpdateUserRequest) (*User, error) {
	var updated *User
	err := s.repo.WithTx(ctx, func(tx Repository) error {
		u, err := tx.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if req.Email != u.Email {
			exists, err := tx.ExistsByEmail(ctx, req.Email)
			if err != nil {
				return err
			}
			if exists {
				return apperror.Conflict("email already exists")
			}
		}
		u.Name = req.Name
		u.Email = req.Email
		updated, err = tx.Update(ctx, id, u)
		return err
	})
	return updated, err
}

func (s *UserService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
