package router

import (
	"microservice/internal/handler"
	"microservice/pkg/middleware"
	"net/http"
)

// RegisterProductRoutes mounts the product routes. Reads require
// authentication; writes additionally require the admin role.
func RegisterProductRoutes(mux *http.ServeMux, handler *handler.ProductHandler, auth, admin middleware.Middleware) *http.ServeMux {
	productMux := http.NewServeMux()

	productMux.Handle("GET /", middleware.With(handler.GetAllProducts, auth))
	productMux.Handle("GET /{id}", middleware.With(handler.GetProductByID, auth))
	productMux.Handle("POST /", middleware.With(handler.CreateProduct, auth, admin, middleware.Test))
	productMux.Handle("PUT /{id}", middleware.With(handler.UpdateProduct, auth, admin))
	productMux.Handle("DELETE /{id}", middleware.With(handler.DeleteProduct, auth, admin))

	mux.Handle("/products/", http.StripPrefix("/products", productMux))
	return mux
}
