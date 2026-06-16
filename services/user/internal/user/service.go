package user

import (
	"context"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type UserService struct {
	repo Repository
}

func NewUserService(repo Repository) *UserService {
	return &UserService{repo: repo}
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

	return s.repo.Create(ctx, &User{Name: req.Name, Email: req.Email})
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
