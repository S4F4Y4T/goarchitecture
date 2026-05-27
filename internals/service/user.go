package service

import (
	"context"
	"microservice/internals/dto"
	"microservice/internals/model"
	"microservice/pkg/appError"
)

type UserService struct {
	repo model.UserRepository
}

func NewUserService(repo model.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetAllUsers(c context.Context, page int, limit int) ([]model.User, error) {
	return s.repo.GetAllUsers(c, page, limit)
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

	if req.Name != "" {
		user.Name = req.Name
	}

	if req.Email != "" && req.Email != user.Email {
		exists, err := s.repo.ExistsByEmail(c, req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, appError.Conflict("email already exists")
		}
		user.Email = req.Email
	}

	return s.repo.UpdateUser(c, id, user)
}

func (s *UserService) DeleteUser(c context.Context, id int) error {
	return s.repo.DeleteUser(c, id)
}
