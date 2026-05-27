package handler

import (
	"encoding/json"
	"log"
	"microservice/internals/dto"
	"microservice/internals/model"
	"microservice/internals/service"
	"microservice/pkg/appError"
	"microservice/pkg/response"
	"microservice/pkg/validation"
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
	var req dto.CreateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, appError.InvalidInput("invalid request body"))
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	log.Printf("Creating user: %+v", req)

	createdUser, err := h.service.CreateUser(r.Context(), &model.User{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, "User created successfully", createdUser)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// Implement logic to handle the request and call the service method
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Implement logic to handle the request and call the service method
}
