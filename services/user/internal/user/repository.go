package user

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/pagination"
	"github.com/s4f4y4t/go-microservice/pkg/query"
	gormquery "github.com/s4f4y4t/go-microservice/pkg/query/gorm"
	"gorm.io/gorm"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) Repository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetAll(ctx context.Context, p pagination.Params, opts query.Options) ([]User, int64, error) {
	var (
		users []User
		total int64
	)
	if err := r.db.WithContext(ctx).Model(&User{}).Scopes(gormquery.Filters(opts)).Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	if err := r.db.WithContext(ctx).Model(&User{}).
		Scopes(gormquery.Filters(opts), gormquery.Sorts(opts)).
		Offset(p.Offset()).Limit(p.Limit).Find(&users).Error; err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return users, total, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("user not found with id " + strconv.Itoa(id))
		}
		return nil, apperror.Internal(err)
	}
	return &u, nil
}

func (r *UserRepository) Create(ctx context.Context, u *User) (*User, error) {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, apperror.Conflict("email already exists")
		}
		return nil, apperror.Internal(err)
	}
	return u, nil
}

func (r *UserRepository) Update(ctx context.Context, id int, u *User) (*User, error) {
	res := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(u)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return nil, apperror.Conflict("email already exists")
		}
		return nil, apperror.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, apperror.NotFound("user not found with id " + strconv.Itoa(id))
	}
	return u, nil
}

func (r *UserRepository) Delete(ctx context.Context, id int) error {
	res := r.db.WithContext(ctx).Delete(&User{}, id)
	if res.Error != nil {
		return apperror.Internal(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperror.NotFound("user not found with id " + strconv.Itoa(id))
	}
	return nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, apperror.Internal(err)
	}
	return count > 0, nil
}

func (r *UserRepository) WithTx(ctx context.Context, fn func(Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&UserRepository{db: tx})
	})
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Internal(err)
	}
	return &u, nil
}
