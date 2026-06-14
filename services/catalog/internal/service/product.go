package service

import (
	"context"
	"github.com/s4f4y4t/go-microservice/services/catalog/internal/dto"
	"github.com/s4f4y4t/go-microservice/services/catalog/internal/model"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
)

type ProductService struct {
	repo model.ProductRepository
}

func NewProductService(repo model.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) GetAllProducts(c context.Context, p pagination.Params, opts query.Options) ([]model.Product, int64, error) {
	return s.repo.GetAllProducts(c, p, opts)
}

func (s *ProductService) GetProductByID(c context.Context, id int) (*model.Product, error) {
	return s.repo.GetProductByID(c, id)
}

func (s *ProductService) CreateProduct(c context.Context, product dto.CreateProductRequest) (*model.Product, error) {
	p := &model.Product{
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
	}
	return s.repo.CreateProduct(c, p)
}

func (s *ProductService) UpdateProduct(c context.Context, id int, req dto.UpdateProductRequest) (*model.Product, error) {
	var updated *model.Product
	err := s.repo.WithTx(c, func(tx model.ProductRepository) error {
		product, err := tx.GetProductByID(c, id)
		if err != nil {
			return err
		}
		product.Name = req.Name
		product.Description = req.Description
		product.Price = req.Price
		updated, err = tx.UpdateProduct(c, id, product)
		return err
	})
	return updated, err
}

func (s *ProductService) DeleteProduct(c context.Context, id int) error {
	return s.repo.DeleteProduct(c, id)
}
