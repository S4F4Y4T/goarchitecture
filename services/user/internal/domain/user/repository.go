package user

import (
	"context"

	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

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
