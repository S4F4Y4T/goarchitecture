package repository

import (
	"context"
	"errors"
	"microservice/internal/model"
	"microservice/pkg/apperror"
	"microservice/pkg/pagination"
	"strconv"

	"gorm.io/gorm"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) model.ProductRepository {
	return &ProductRepository{
		db: db,
	}
}

func (r *ProductRepository) GetAllProducts(ctx context.Context, p pagination.Params) ([]model.Product, int64, error) {
	var (
		products []model.Product
		total    int64
	)
	if err := r.db.WithContext(ctx).Model(&model.Product{}).Count(&total).Error; err != nil {
		return nil, 0, appError.Internal(err)
	}
	if err := r.db.WithContext(ctx).Offset(p.Offset()).Limit(p.Limit).Find(&products).Error; err != nil {
		return nil, 0, appError.Internal(err)
	}
	return products, total, nil
}

func (r *ProductRepository) GetProductByID(ctx context.Context, id int) (*model.Product, error) {
	var product model.Product
	if err := r.db.WithContext(ctx).First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appError.NotFound("product not found with id " + strconv.Itoa(id))
		}
		return nil, appError.Internal(err)
	}
	return &product, nil
}

func (r *ProductRepository) CreateProduct(ctx context.Context, product *model.Product) (*model.Product, error) {
	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, appError.Conflict("product already exists")
		}
		return nil, appError.Internal(err)
	}
	return product, nil
}

func (r *ProductRepository) UpdateProduct(ctx context.Context, id int, product *model.Product) (*model.Product, error) {
	// Select the replaceable columns explicitly so PUT writes zero values too
	// (e.g. clearing description or setting price to 0); a plain Updates with a
	// struct would skip zero-value fields.
	res := r.db.WithContext(ctx).Model(&model.Product{}).Where("id = ?", id).
		Select("name", "description", "price").Updates(product)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return nil, appError.Conflict("product already exists")
		}
		return nil, appError.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, appError.NotFound("product not found with id " + strconv.Itoa(id))
	}
	return product, nil
}

func (r *ProductRepository) DeleteProduct(ctx context.Context, id int) error {
	res := r.db.WithContext(ctx).Delete(&model.Product{}, id)
	if res.Error != nil {
		return appError.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return appError.NotFound("product not found with id " + strconv.Itoa(id))
	}
	return nil
}
