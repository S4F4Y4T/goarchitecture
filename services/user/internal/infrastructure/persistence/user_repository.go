package persistence

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	gormquery "github.com/s4f4y4t/go-microservice/pkg/query/gorm"
	userDomain "github.com/s4f4y4t/go-microservice/services/user/internal/domain/user"
	"gorm.io/gorm"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) userDomain.Repository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]userDomain.User, int64, error) {
	var (
		users []userDomain.User
		total int64
	)
	if err := r.db.WithContext(ctx).Model(&userDomain.User{}).Scopes(gormquery.Filters(opts)).Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	if err := r.db.WithContext(ctx).Model(&userDomain.User{}).
		Scopes(gormquery.Filters(opts), gormquery.Sorts(opts)).
		Offset(p.Offset()).Limit(p.Limit).Find(&users).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return users, total, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int) (*userDomain.User, error) {
	var user userDomain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("user not found with id " + strconv.Itoa(id))
		}
		return nil, apperror.Internal(err)
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *userDomain.User) (*userDomain.User, error) {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, apperror.Conflict("email already exists")
		}
		return nil, apperror.Internal(err)
	}
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, id int, user *userDomain.User) (*userDomain.User, error) {
	res := r.db.WithContext(ctx).Model(&userDomain.User{}).Where("id = ?", id).Updates(user)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return nil, apperror.Conflict("email already exists")
		}
		return nil, apperror.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, apperror.NotFound("user not found with id " + strconv.Itoa(id))
	}
	return user, nil
}

func (r *UserRepository) Delete(ctx context.Context, id int) error {
	res := r.db.WithContext(ctx).Delete(&userDomain.User{}, id)
	if res.Error != nil {
		return apperror.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperror.NotFound("user not found with id " + strconv.Itoa(id))
	}
	return nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email userDomain.Email) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&userDomain.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, apperror.Internal(err)
	}
	return count > 0, nil
}

func (r *UserRepository) WithTx(ctx context.Context, fn func(userDomain.Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&UserRepository{db: tx})
	})
}

func (r *UserRepository) GetByEmail(ctx context.Context, email userDomain.Email) (*userDomain.User, error) {
	var user userDomain.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Internal(err)
	}
	return &user, nil
}
