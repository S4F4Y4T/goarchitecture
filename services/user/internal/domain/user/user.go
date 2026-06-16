package user

import (
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type User struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Email     Email    `json:"email"`
	Password  Password `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

func New(name string, email Email, password Password) *User {
	return &User{
		Name:     name,
		Email:    email,
		Password: password,
	}
}

var ListSchema = query.Schema{
	"id":         {Column: "id", Sortable: true, Filterable: true},
	"name":       {Column: "name", Sortable: true, Filterable: true, Partial: true},
	"email":      {Column: "email", Sortable: true, Filterable: true, Partial: true},
	"created_at": {Column: "created_at", Sortable: true},
	"updated_at": {Column: "updated_at", Sortable: true},
}
