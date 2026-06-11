package model

import (
	"context"
	"microservice/pkg/pagination"
	"microservice/pkg/query"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserListSchema declares the fields clients may sort and filter the user list
// by. Columns are sourced from here only, so they are safe to use in ORDER BY /
// WHERE clauses. String fields use partial (ILIKE) matching.
var UserListSchema = query.Schema{
	"id":    {Column: "id", Sortable: true, Filterable: true},
	"name":  {Column: "name", Sortable: true, Filterable: true, Partial: true},
	"email": {Column: "email", Sortable: true, Filterable: true, Partial: true},
}

type UserRepository interface {
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetAllUsers(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error)
	CreateUser(ctx context.Context, user *User) (*User, error)
	UpdateUser(ctx context.Context, id int, user *User) (*User, error)
	DeleteUser(ctx context.Context, id int) error

	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
