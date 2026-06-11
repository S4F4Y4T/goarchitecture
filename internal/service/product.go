package service

import (
	"context"
	"microservice/internals/dto"
	"microservice/internals/model"
	"microservice/pkg/pagination"
)

type ProductService struct {
	repo model.ProductRepository
}

func NewProductService(repo model.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) GetAllProducts(c context.Context, p pagination.Params) ([]model.Product, int64, error) {
	return s.repo.GetAllProducts(c, p)
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

	product, err := s.repo.GetProductByID(c, id)
	if err != nil {
		return nil, err
	}

	product.Name = req.Name
	product.Description = req.Description
	product.Price = req.Price

	return s.repo.UpdateProduct(c, id, product)
}

func (s *ProductService) DeleteProduct(c context.Context, id int) error {
	return s.repo.DeleteProduct(c, id)
}
