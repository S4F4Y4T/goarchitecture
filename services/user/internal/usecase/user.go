package usecase

import (
	"context"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	userDomain "github.com/s4f4y4t/go-microservice/services/user/internal/domain/user"
	"github.com/s4f4y4t/go-microservice/services/user/internal/usecase/port"
)

type UserService struct {
	repo userDomain.Repository
}

var _ port.UserUseCase = (*UserService)(nil)

func NewUserService(repo userDomain.Repository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]userDomain.User, int64, error) {
	return s.repo.GetAll(ctx, p, opts)
}

func (s *UserService) GetByID(ctx context.Context, id int) (*userDomain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) Create(ctx context.Context, input port.CreateUserInput) (*userDomain.User, error) {
	email, err := userDomain.NewEmail(input.Email)
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

	return s.repo.Create(ctx, userDomain.NewWithoutPassword(input.Name, email))
}

func (s *UserService) Update(ctx context.Context, id int, input port.UpdateUserInput) (*userDomain.User, error) {
	email, err := userDomain.NewEmail(input.Email)
	if err != nil {
		return nil, apperror.InvalidInput(err.Error())
	}

	var updated *userDomain.User
	err = s.repo.WithTx(ctx, func(tx userDomain.Repository) error {
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
		user.Name = input.Name
		user.Email = email
		updated, err = tx.Update(ctx, id, user)
		return err
	})
	return updated, err
}

func (s *UserService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
