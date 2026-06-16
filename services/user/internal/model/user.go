package model

import (
	"context"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     Email     `json:"email"`
	Password  Password  `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func New(name string, email Email, password Password) *User {
	return &User{Name: name, Email: email, Password: password}
}

func NewWithoutPassword(name string, email Email) *User {
	return &User{Name: name, Email: email}
}

var UserListSchema = query.Schema{
	"id":         {Column: "id", Sortable: true, Filterable: true},
	"name":       {Column: "name", Sortable: true, Filterable: true, Partial: true},
	"email":      {Column: "email", Sortable: true, Filterable: true, Partial: true},
	"created_at": {Column: "created_at", Sortable: true},
	"updated_at": {Column: "updated_at", Sortable: true},
}

type Repository interface {
	GetByID(ctx context.Context, id int) (*User, error)
	GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error)
	Create(ctx context.Context, user *User) (*User, error)
	Update(ctx context.Context, id int, user *User) (*User, error)
	Delete(ctx context.Context, id int) error
	ExistsByEmail(ctx context.Context, email Email) (bool, error)
	GetByEmail(ctx context.Context, email Email) (*User, error)
	WithTx(ctx context.Context, fn func(Repository) error) error
}
