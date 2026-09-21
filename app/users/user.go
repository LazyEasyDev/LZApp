package users

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:64;not null;uniqueIndex" json:"username"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Email     string    `gorm:"size:254;not null;uniqueIndex" json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repository struct {
	database *gorm.DB
}

func New(database *gorm.DB) (*Repository, error) {
	if database == nil {
		return nil, fmt.Errorf("GORM database is required")
	}
	return &Repository{database: database}, nil
}

func CreateTable(ctx context.Context, database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("GORM database is required")
	}
	return database.WithContext(ctx).AutoMigrate(&User{})
}

func InitData(ctx context.Context, database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("GORM database is required")
	}
	initial := User{
		Username: "admin",
		Name:     "Administrator",
		Email:    "admin@lzapp.local",
	}
	return database.WithContext(ctx).
		Where(User{Username: initial.Username}).
		Attrs(initial).
		FirstOrCreate(&initial).Error
}

func (repository *Repository) Create(ctx context.Context, user *User) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}
	return repository.database.WithContext(ctx).Create(user).Error
}

func (repository *Repository) Get(ctx context.Context, id uint64) (*User, error) {
	var user User
	if err := repository.database.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (repository *Repository) List(ctx context.Context, limit, offset int) ([]User, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	var users []User
	err := repository.database.WithContext(ctx).
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, err
}

func (repository *Repository) Update(ctx context.Context, user *User) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("user with an ID is required")
	}
	result := repository.database.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"username": user.Username,
			"name":     user.Name,
			"email":    user.Email,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (repository *Repository) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("user ID is required")
	}
	result := repository.database.WithContext(ctx).Delete(&User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
