package router

import (
	"microservice/internal/handler"
	"microservice/pkg/middleware"
	"net/http"
)

// RegisterUsersRoute mounts the user routes. All routes require
// authentication. Self-service signup lives at /auth/register, so direct user
// creation is admin-only; update/delete enforce self-or-admin in the handler.
func RegisterUsersRoute(mux *http.ServeMux, handler *handler.UserHandler, auth, admin middleware.Middleware) *http.ServeMux {
	userMux := http.NewServeMux()

	userMux.Handle("GET /", middleware.With(handler.GetAllUsers, auth))
	userMux.Handle("GET /{id}", middleware.With(handler.GetUserByID, auth))
	userMux.Handle("POST /", middleware.With(handler.CreateUser, auth, admin))
	userMux.Handle("PUT /{id}", middleware.With(handler.UpdateUser, auth))
	userMux.Handle("DELETE /{id}", middleware.With(handler.DeleteUser, auth))

	mux.Handle("/users/", http.StripPrefix("/users", userMux))
	return mux
}
