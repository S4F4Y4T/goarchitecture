package service

import (
	"context"
	"microservice/internals/model"
)

type UserService struct {
	repo model.UserRepository
}

func NewUserService(repo model.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetAllUsers(c context.Context) ([]model.User, error) {
	return s.repo.GetAllUsers(c)
}

func (s *UserService) GetUserByID(c context.Context, id int) (*model.User, error) {
	return s.repo.GetUserByID(c, id)
}
