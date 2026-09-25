package users

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/LazyEasyDev/LZApp/components"
	"gorm.io/gorm"
)

type User struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Name      *string   `gorm:"size:100" json:"name"`
	Email     string    `gorm:"size:254;not null;uniqueIndex" json:"email"`
	ApiToken  string    `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Access    string    `gorm:"type:text" json:"access"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func CreateTable(ctx context.Context) error {
	migrator := components.GetDB().WithContext(ctx).Migrator()
	if migrator.HasTable(&User{}) {
		return nil
	}
	return migrator.CreateTable(&User{})
}

func InitData(ctx context.Context) error {
	database := components.GetDB()
	name := "admin"
	initial := User{
		Name:   &name,
		Email:  "admin@lzapp.local",
		Access: GetAccessListJsonStr(),
	}
	return database.WithContext(ctx).
		Where(User{Email: initial.Email}).
		Attrs(initial).
		FirstOrCreate(&initial).Error
}

func normalizeAccess(value string) (string, error) {
	if value == "" {
		return "[]", nil
	}
	var access []string
	if err := json.Unmarshal([]byte(value), &access); err != nil {
		return "", fmt.Errorf("access must be a JSON array of strings: %w", err)
	}
	if access == nil {
		return "", fmt.Errorf("access must be a JSON array of strings")
	}
	var allowed []string
	if err := json.Unmarshal([]byte(GetAccessListJsonStr()), &allowed); err != nil {
		return "", fmt.Errorf("decode access catalog: %w", err)
	}
	normalized := make([]string, 0, len(allowed))
	for _, permission := range access {
		if !slices.Contains(allowed, permission) {
			return "", fmt.Errorf("unknown access permission %q", permission)
		}
		if !slices.Contains(normalized, permission) {
			normalized = append(normalized, permission)
		}
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("encode access: %w", err)
	}
	return string(encoded), nil
}

func Create(ctx context.Context, user *User) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}
	access, err := normalizeAccess(user.Access)
	if err != nil {
		return err
	}
	user.Access = access
	database := components.GetDB()
	return database.WithContext(ctx).Create(user).Error
}

func Get(ctx context.Context, id uint64) (*User, error) {
	database := components.GetDB()
	var user User
	if err := database.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func List(ctx context.Context, limit, offset int) ([]User, error) {
	database := components.GetDB()
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
	err := database.WithContext(ctx).
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, err
}

func Update(ctx context.Context, user *User) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("user with an ID is required")
	}
	access, err := normalizeAccess(user.Access)
	if err != nil {
		return err
	}
	user.Access = access
	database := components.GetDB()
	result := database.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"name":   user.Name,
			"email":  user.Email,
			"access": user.Access,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("user ID is required")
	}
	database := components.GetDB()

	result := database.WithContext(ctx).Delete(&User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
