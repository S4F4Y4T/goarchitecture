package service

import (
	"context"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/model"
)

type UserService struct {
	repo model.Repository
}

func NewUserService(repo model.Repository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]model.User, int64, error) {
	return s.repo.GetAll(ctx, p, opts)
}

func (s *UserService) GetByID(ctx context.Context, id int) (*model.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) Create(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
	email, err := model.NewEmail(req.Email)
	if err != nil {
		return nil, apperror.InvalidInput(err.Error())
	}

	exists, err := s.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("email already exists")
	}

	return s.repo.Create(ctx, model.NewWithoutPassword(req.Name, email))
}

func (s *UserService) Update(ctx context.Context, id int, req dto.UpdateUserRequest) (*model.User, error) {
	email, err := model.NewEmail(req.Email)
	if err != nil {
		return nil, apperror.InvalidInput(err.Error())
	}

	var updated *model.User
	err = s.repo.WithTx(ctx, func(tx model.Repository) error {
		user, err := tx.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if email != user.Email {
			exists, err := tx.ExistsByEmail(ctx, email)
			if err != nil {
				return err
			}
			if exists {
				return apperror.Conflict("email already exists")
			}
		}
		user.Name = req.Name
		user.Email = email
		updated, err = tx.Update(ctx, id, user)
		return err
	})
	return updated, err
}

func (s *UserService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
