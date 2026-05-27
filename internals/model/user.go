package model

import (
	"context"
	"microservice/pkg/pagination"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserRepository interface {
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetAllUsers(ctx context.Context, p pagination.Params) ([]User, int64, error)
	CreateUser(ctx context.Context, user *User) (*User, error)
	UpdateUser(ctx context.Context, id int, user *User) (*User, error)
	DeleteUser(ctx context.Context, id int) error

	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
