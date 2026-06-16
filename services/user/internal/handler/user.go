package handler

import (
	"net/http"
	"strconv"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	"github.com/s4f4y4t/go-microservice/pkg/request"
	"github.com/s4f4y4t/go-microservice/pkg/response"
	"github.com/s4f4y4t/go-microservice/pkg/validation"
	"github.com/s4f4y4t/go-microservice/services/user/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/user/internal/model"
	"github.com/s4f4y4t/go-microservice/services/user/internal/service"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()

	page, _ := strconv.Atoi(queryParams.Get("page"))
	limit, _ := strconv.Atoi(queryParams.Get("limit"))
	params := pagination.NewParams(page, limit)
	opts := query.Parse(queryParams, model.UserListSchema)

	logger.FromContext(r.Context()).Debug("fetching users", "page", params.Page, "limit", params.Limit)

	users, total, err := h.service.GetAll(r.Context(), params, opts)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.SuccessWithMeta(w, http.StatusOK, "Users retrieved successfully", users, pagination.NewMeta(params, total))
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid user id"))
		return
	}
	user, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "User retrieved successfully", user)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	logger.FromContext(r.Context()).Info("creating user", "name", req.Name)

	user, err := h.service.Create(r.Context(), req)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, "User created successfully", user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid user id"))
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

	user, err := h.service.Update(r.Context(), uid, req)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "User updated successfully", user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid user id"))
		return
	}

	if err := h.service.Delete(r.Context(), uid); err != nil {
		response.Error(w, r, err)
		return
	}

	response.NoContent(w)
}
