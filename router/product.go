package router

import (
	"microservice/internals/handler"
	"net/http"
)

func RegisterProductRoutes(mux *http.ServeMux, handler *handler.ProductHandler) *http.ServeMux {
	productMux := http.NewServeMux()

	productMux.HandleFunc("GET /", handler.GetAllProducts)
	productMux.HandleFunc("GET /{id}", handler.GetProductByID)
	productMux.HandleFunc("POST /", handler.CreateProduct)
	productMux.HandleFunc("PUT /{id}", handler.UpdateProduct)
	productMux.HandleFunc("DELETE /{id}", handler.DeleteProduct)

	mux.Handle("/products/", http.StripPrefix("/products", productMux))
	return mux
}
