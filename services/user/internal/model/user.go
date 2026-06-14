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
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserListSchema declares the fields clients may sort and filter the user list
// by. Columns are sourced from here only, so they are safe to use in ORDER BY /
// WHERE clauses. String fields use partial (ILIKE) matching.
var UserListSchema = query.Schema{
	"id":         {Column: "id", Sortable: true, Filterable: true},
	"name":       {Column: "name", Sortable: true, Filterable: true, Partial: true},
	"email":      {Column: "email", Sortable: true, Filterable: true, Partial: true},
	"created_at": {Column: "created_at", Sortable: true},
	"updated_at": {Column: "updated_at", Sortable: true},
}

type UserRepository interface {
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetAllUsers(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error)
	CreateUser(ctx context.Context, user *User) (*User, error)
	UpdateUser(ctx context.Context, id int, user *User) (*User, error)
	DeleteUser(ctx context.Context, id int) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	GetByEmail(ctx context.Context, email string) (*User, error)

	// WithTx runs fn inside a single database transaction. The repo passed to fn
	// shares the same transaction so all operations are atomic.
	WithTx(ctx context.Context, fn func(repo UserRepository) error) error
}
