package service

import (
	"context"
	"microservice/internal/dto"
	"microservice/internal/model"
	"microservice/pkg/apperror"
	"microservice/pkg/pagination"
	"microservice/pkg/query"
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
		return nil, appError.Conflict("email already exists")
	}

	return s.repo.CreateUser(c, user)
}

func (s *UserService) UpdateUser(c context.Context, id int, req dto.UpdateUserRequest) (*model.User, error) {

	user, err := s.repo.GetUserByID(c, id)
	if err != nil {
		return nil, err
	}

	if req.Email != user.Email {
		exists, err := s.repo.ExistsByEmail(c, req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, appError.Conflict("email already exists")
		}
	}

	user.Name = req.Name
	user.Email = req.Email

	return s.repo.UpdateUser(c, id, user)
}

func (s *UserService) DeleteUser(c context.Context, id int) error {
	return s.repo.DeleteUser(c, id)
}
