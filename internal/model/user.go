package model

import (
	"context"
	"time"

	"microservice/pkg/pagination"
	"microservice/pkg/query"
)

// Roles a user can hold. Plain strings (not a custom type) to keep GORM and
// JSON mapping simple; the DB enforces valid values via a CHECK constraint.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// UserListSchema declares the fields clients may sort and filter the user list
// by. Columns are sourced from here only, so they are safe to use in ORDER BY /
// WHERE clauses. String fields use partial (ILIKE) matching.
var UserListSchema = query.Schema{
	"id":         {Column: "id", Sortable: true, Filterable: true},
	"name":       {Column: "name", Sortable: true, Filterable: true, Partial: true},
	"email":      {Column: "email", Sortable: true, Filterable: true, Partial: true},
	"role":       {Column: "role", Sortable: true, Filterable: true},
	"created_at": {Column: "created_at", Sortable: true},
	"updated_at": {Column: "updated_at", Sortable: true},
}

type UserRepository interface {
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetAllUsers(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error)
	CreateUser(ctx context.Context, user *User) (*User, error)
	UpdateUser(ctx context.Context, id int, user *User) (*User, error)
	DeleteUser(ctx context.Context, id int) error

	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
