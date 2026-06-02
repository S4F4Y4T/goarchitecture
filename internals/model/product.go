package model

import (
	"context"
	"microservice/pkg/pagination"
)

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

type ProductRepository interface {
	GetProductByID(ctx context.Context, id int) (*Product, error)
	GetAllProducts(ctx context.Context, p pagination.Params) ([]Product, int64, error)
	CreateProduct(ctx context.Context, product *Product) (*Product, error)
	UpdateProduct(ctx context.Context, id int, product *Product) (*Product, error)
	DeleteProduct(ctx context.Context, id int) error
}
