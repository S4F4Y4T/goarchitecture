package model

import (
	"context"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type Product struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var ProductListSchema = query.Schema{
	"id":          {Column: "id", Sortable: true, Filterable: true},
	"name":        {Column: "name", Sortable: true, Filterable: true, Partial: true},
	"description": {Column: "description", Sortable: false, Filterable: true, Partial: true},
	"price":       {Column: "price", Sortable: true, Filterable: true},
	"created_at":  {Column: "created_at", Sortable: true},
	"updated_at":  {Column: "updated_at", Sortable: true},
}

type ProductRepository interface {
	GetProductByID(ctx context.Context, id int) (*Product, error)
	GetAllProducts(ctx context.Context, p pagination.Params, opts query.Options) ([]Product, int64, error)
	CreateProduct(ctx context.Context, product *Product) (*Product, error)
	UpdateProduct(ctx context.Context, id int, product *Product) (*Product, error)
	DeleteProduct(ctx context.Context, id int) error

	// WithTx runs fn inside a single database transaction. The repo passed to fn
	// shares the same transaction so all operations are atomic.
	WithTx(ctx context.Context, fn func(repo ProductRepository) error) error
}
