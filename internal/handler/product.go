package handler

import (
	"microservice/internal/dto"
	"microservice/internal/service"
	"microservice/pkg/apperror"
	"microservice/pkg/logger"
	"microservice/pkg/pagination"
	"microservice/pkg/request"
	"microservice/pkg/response"
	"microservice/pkg/validation"
	"net/http"
	"strconv"
)

type ProductHandler struct {
	service *service.ProductService
}

func NewProductHandler(service *service.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) GetAllProducts(w http.ResponseWriter, r *http.Request) {

	queryParams := r.URL.Query()

	page, _ := strconv.Atoi(queryParams.Get("page"))
	limit, _ := strconv.Atoi(queryParams.Get("limit"))
	params := pagination.NewParams(page, limit)

	logger.FromContext(r.Context()).Debug("fetching products", "page", params.Page, "limit", params.Limit)

	products, total, err := h.service.GetAllProducts(r.Context(), params)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.SuccessWithMeta(w, http.StatusOK, "Products retrieved successfully", products, pagination.NewMeta(params, total))
}

func (h *ProductHandler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid product id"))
		return
	}
	product, err := h.service.GetProductByID(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, "Product retrieved successfully", product)
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProductRequest

	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	logger.FromContext(r.Context()).Info("creating product", "name", req.Name)

	createdProduct, err := h.service.CreateProduct(r.Context(), req)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, "Product created successfully", createdProduct)
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid product id"))
		return
	}

	var req dto.UpdateProductRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := validation.Validate(&req); err != nil {
		response.Error(w, r, err)
		return
	}

	updatedProduct, err := h.service.UpdateProduct(r.Context(), uid, req)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.Success(w, http.StatusOK, "Product updated successfully", updatedProduct)
}

func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		response.Error(w, r, apperror.InvalidInput("invalid product id"))
		return
	}

	if err := h.service.DeleteProduct(r.Context(), uid); err != nil {
		response.Error(w, r, err)
		return
	}

	response.NoContent(w)
}
