package auth

import (
	"context"
	"errors"
	"time"

	"cloudsentinel/internal/audit"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type Store interface {
	FindByUsername(context.Context, string) (User, error)
	FindByID(context.Context, uint64) (User, error)
	CreateUser(context.Context, *User) error
	RecordFailure(context.Context, audit.Log) error
	RecordSuccess(context.Context, uint64, time.Time, audit.Log) error
}

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindByUsername(ctx context.Context, username string) (User, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return user, nil
}

func (r *Repository) FindByID(ctx context.Context, id uint64) (User, error) {
	var user User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return user, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) RecordFailure(ctx context.Context, entry audit.Log) error {
	return r.db.WithContext(ctx).Create(&entry).Error
}

func (r *Repository) RecordSuccess(ctx context.Context, userID uint64, at time.Time, entry audit.Log) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).Where("id = ?", userID).Update("last_login_at", at).Error; err != nil {
			return err
		}
		return tx.Create(&entry).Error
	})
}
