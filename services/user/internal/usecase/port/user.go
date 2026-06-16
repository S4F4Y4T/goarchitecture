package port

import (
	"context"

	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	userDomain "github.com/s4f4y4t/go-microservice/services/user/internal/domain/user"
)

type CreateUserInput struct {
	Name  string
	Email string
}

type UpdateUserInput struct {
	Name  string
	Email string
}

type UserUseCase interface {
	GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]userDomain.User, int64, error)
	GetByID(ctx context.Context, id int) (*userDomain.User, error)
	Create(ctx context.Context, input CreateUserInput) (*userDomain.User, error)
	Update(ctx context.Context, id int, input UpdateUserInput) (*userDomain.User, error)
	Delete(ctx context.Context, id int) error
}
