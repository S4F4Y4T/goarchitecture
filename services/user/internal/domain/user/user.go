package user

import "time"

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
