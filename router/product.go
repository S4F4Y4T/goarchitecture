package router

import (
	"microservice/internal/handler"
	"microservice/pkg/middleware"
	"net/http"
)

func RegisterProductRoutes(mux *http.ServeMux, handler *handler.ProductHandler) *http.ServeMux {
	productMux := http.NewServeMux()

	productMux.HandleFunc("GET /", handler.GetAllProducts)
	productMux.HandleFunc("GET /{id}", handler.GetProductByID)
	productMux.Handle("POST /", middleware.With(handler.CreateProduct, middleware.Test))
	productMux.HandleFunc("PUT /{id}", handler.UpdateProduct)
	productMux.HandleFunc("DELETE /{id}", handler.DeleteProduct)

	mux.Handle("/products/", http.StripPrefix("/products", productMux))
	return mux
}
