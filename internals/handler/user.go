package handler

import (
	"microservice/internals/service"
	"microservice/pkg/appError"
	"microservice/pkg/response"
	"net/http"
	"strconv"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.GetAllUsers(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "Users retrieved successfully", users)
}

func (h *UserHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, r, appError.InvalidInput("invalid user id"))
		return
	}
	user, err := h.service.GetUserByID(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "User retrieved successfully", user)
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Implement logic to handle the request and call the service method
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// Implement logic to handle the request and call the service method
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Implement logic to handle the request and call the service method
}
