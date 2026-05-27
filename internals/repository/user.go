package repository

import (
	"context"
	"errors"
	"microservice/internals/model"
	"microservice/pkg/appError"
	"microservice/pkg/pagination"
	"strconv"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) model.UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetAllUsers(ctx context.Context, p pagination.Params) ([]model.User, int64, error) {
	var (
		users []model.User
		total int64
	)
	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, appError.Internal(err)
	}
	if err := r.db.WithContext(ctx).Offset(p.Offset()).Limit(p.Limit).Find(&users).Error; err != nil {
		return nil, 0, appError.Internal(err)
	}
	return users, total, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appError.NotFound("user not found with id " + strconv.Itoa(id))
		}
		return nil, appError.Internal(err)
	}
	return &user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, appError.Internal(err)
	}
	return user, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, id int, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appError.NotFound("user not found with id " + strconv.Itoa(id))
		}
		return nil, appError.Internal(err)
	}
	return user, nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, id int) error {
	if err := r.db.WithContext(ctx).Delete(&model.User{}, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appError.NotFound("user not found with id " + strconv.Itoa(id))
		}
		return appError.Internal(err)
	}
	return nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, appError.Internal(err)
	}
	return count > 0, nil
}
