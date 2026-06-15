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
	repo model.UserRepository
}

func NewUserService(repo model.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetAllUsers(c context.Context, p pagination.Params, opts query.Options) ([]model.User, int64, error) {
	return s.repo.GetAllUsers(c, p, opts)
}

func (s *UserService) GetUserByID(c context.Context, id int) (*model.User, error) {
	return s.repo.GetUserByID(c, id)
}

func (s *UserService) CreateUser(c context.Context, user *model.User) (*model.User, error) {

	exists, err := s.repo.ExistsByEmail(c, user.Email)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, apperror.Conflict("email already exists")
	}

	return s.repo.CreateUser(c, user)
}

func (s *UserService) UpdateUser(c context.Context, id int, req dto.UpdateUserRequest) (*model.User, error) {
	var updated *model.User
	err := s.repo.WithTx(c, func(tx model.UserRepository) error {
		user, err := tx.GetUserByID(c, id)
		if err != nil {
			return err
		}
		if req.Email != user.Email {
			exists, err := tx.ExistsByEmail(c, req.Email)
			if err != nil {
				return err
			}
			if exists {
				return apperror.Conflict("email already exists")
			}
		}
		user.Name = req.Name
		user.Email = req.Email
		updated, err = tx.UpdateUser(c, id, user)
		return err
	})
	return updated, err
}

func (s *UserService) DeleteUser(c context.Context, id int) error {
	return s.repo.DeleteUser(c, id)
}
