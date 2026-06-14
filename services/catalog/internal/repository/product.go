package repository

import (
	"context"
	"errors"
	"github.com/s4f4y4t/go-microservice/services/catalog/internal/model"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	gormquery "github.com/s4f4y4t/go-microservice/pkg/query/gorm"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) model.ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) GetAllProducts(ctx context.Context, p pagination.Params, opts query.Options) ([]model.Product, int64, error) {
	var (
		products []model.Product
		total    int64
	)
	if err := r.db.WithContext(ctx).Model(&model.Product{}).Scopes(gormquery.Filters(opts)).Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	if err := r.db.WithContext(ctx).Model(&model.Product{}).
		Scopes(gormquery.Filters(opts), gormquery.Sorts(opts)).
		Offset(p.Offset()).Limit(p.Limit).Find(&products).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return products, total, nil
}

func (r *ProductRepository) GetProductByID(ctx context.Context, id int) (*model.Product, error) {
	var product model.Product
	if err := r.db.WithContext(ctx).First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("product not found with id " + strconv.Itoa(id))
		}
		return nil, apperror.Internal(err)
	}
	return &product, nil
}

func (r *ProductRepository) CreateProduct(ctx context.Context, product *model.Product) (*model.Product, error) {
	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, apperror.Conflict("product already exists")
		}
		return nil, apperror.Internal(err)
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
			return nil, apperror.Conflict("product already exists")
		}
		return nil, apperror.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, apperror.NotFound("product not found with id " + strconv.Itoa(id))
	}
	return product, nil
}

func (r *ProductRepository) DeleteProduct(ctx context.Context, id int) error {
	res := r.db.WithContext(ctx).Delete(&model.Product{}, id)
	if res.Error != nil {
		return apperror.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperror.NotFound("product not found with id " + strconv.Itoa(id))
	}
	return nil
}

func (r *ProductRepository) WithTx(ctx context.Context, fn func(model.ProductRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&ProductRepository{db: tx})
	})
}
