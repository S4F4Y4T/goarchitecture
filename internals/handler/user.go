package handler

import (
	"log"
	"microservice/internals/dto"
	"microservice/internals/middleweare"
	"microservice/internals/model"
	"microservice/internals/service"
	"microservice/pkg/appError"
	"microservice/pkg/pagination"
	"microservice/pkg/request"
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

	queryParams := r.URL.Query()

	page, _ := strconv.Atoi(queryParams.Get("page"))
	limit, _ := strconv.Atoi(queryParams.Get("limit"))
	params := pagination.NewParams(page, limit)

	log.Printf("Fetching users with page: %d, limit: %d", params.Page, params.Limit)

	users, total, err := h.service.GetAllUsers(r.Context(), params)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.SuccessWithMeta(w, http.StatusOK, "Users retrieved successfully", users, pagination.NewMeta(params, total))
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

	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	log.Printf("[%s] Creating user: %+v", middleweare.GetRequestID(r.Context()), req)

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
	id := r.PathValue("id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		response.Error(w, r, appError.InvalidInput("invalid user id"))
		return
	}

	var req dto.UpdateUserRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	updateUser, err := h.service.UpdateUser(r.Context(), uid, req)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "User updated successfully", updateUser)
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		response.Error(w, r, appError.InvalidInput("invalid user id"))
		return
	}

	if err := h.service.DeleteUser(r.Context(), uid); err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "User deleted successfully", nil)
}
