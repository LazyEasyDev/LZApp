package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/LazyEasyDev/LZApp/app/data"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/components/redis"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Name      *string   `gorm:"size:100" json:"name"`
	Email     string    `gorm:"size:254;not null;uniqueIndex" json:"email"`
	Password  string    `gorm:"size:128;not null" json:"password"`
	ApiToken  string    `gorm:"size:128;not null;uniqueIndex" json:"api_token"`
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
	password_plain := "admin"
	email := "admin@lzapp.local"
	pass, err := components.GetPasswordHasher().HashPassword(password_plain)
	if err != nil {
		return err
	}

	apiToken, err := components.GetTokenSigner().GenerateRandToken() // Replace with a proper API token generator
	if err != nil {
		return err
	}
	// Initialize the admin user with the given credentials
	initial := User{
		Name:     &name,
		Email:    email,
		Access:   GetAccessListJsonStr(),
		Password: pass,
		ApiToken: apiToken, // Replace with a proper initial API token if needed
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
	var allowed []string = GetAccessList()
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

func GetByID(ctx context.Context, id uint64) (*User, bool, error) {
	return getByID(ctx, id, components.GetDB(), components.GetRedis(), false)
}

func getByID(ctx context.Context, id uint64, database *gorm.DB, cache *redis.Client, forceUpdate bool) (*User, bool, error) {
	return data.GetRLCached(ctx, "user:id:"+fmt.Sprint(id), cache, forceUpdate, func(ctx context.Context) (*User, bool, error) {
		var user User
		err := database.WithContext(ctx).First(&user, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &user, false, nil
	})
}

func GetByEmail(ctx context.Context, email string) (*User, bool, error) {
	return getByEmail(ctx, email, components.GetDB(), components.GetRedis(), false)
}

func getByEmail(ctx context.Context, email string, database *gorm.DB, cache *redis.Client, forceUpdate bool) (*User, bool, error) {
	return data.GetRLCached(ctx, "user:email:"+email, cache, forceUpdate, func(ctx context.Context) (*User, bool, error) {
		var user User
		err := database.WithContext(ctx).Where("email = ?", email).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &user, false, nil
	})
}

func GetByApiToken(ctx context.Context, api_token string) (*User, bool, error) {
	return getByApiToken(ctx, api_token, components.GetDB(), components.GetRedis(), false)
}

func getByApiToken(ctx context.Context, apiToken string, database *gorm.DB, cache *redis.Client, forceUpdate bool) (*User, bool, error) {
	return data.GetRLCached(ctx, "user:api_token:"+apiToken, cache, forceUpdate, func(ctx context.Context) (*User, bool, error) {
		var user User
		err := database.WithContext(ctx).Where("api_token = ?", apiToken).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &user, false, nil
	})
}

type ListFilter struct {
	ID       uint64
	Name     string
	Email    string
	ApiToken string
	Access   string
}

func applyListFilter(query *gorm.DB, filter ListFilter) *gorm.DB {
	if filter.ID != 0 {
		query = query.Where("id = ?", filter.ID)
	}
	escape := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	if filter.Name != "" {
		query = query.Where("name LIKE ? ESCAPE '!'", "%"+escape.Replace(filter.Name)+"%")
	}
	if filter.Email != "" {
		query = query.Where("email LIKE ? ESCAPE '!'", "%"+escape.Replace(filter.Email)+"%")
	}
	if filter.ApiToken != "" {
		query = query.Where("api_token = ?", filter.ApiToken)
	}
	if filter.Access != "" {
		query = query.Where("access LIKE ?", "%\""+escape.Replace(filter.Access)+"\"%")
	}
	return query
}

func List(ctx context.Context, filter ListFilter, limit, offset int) ([]User, error) {
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
	err := applyListFilter(components.GetDB().WithContext(ctx), filter).
		Select("id", "name", "email", "access", "created_at", "updated_at").
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, err
}

func Count(ctx context.Context, filter ListFilter) (int64, error) {
	var total int64
	err := applyListFilter(components.GetDB().WithContext(ctx).Model(&User{}), filter).Count(&total).Error
	return total, err
}

func refreshUserCache(ctx context.Context, database *gorm.DB, cache *redis.Client, users ...*User) {
	ids := make(map[uint64]bool)
	emails := make(map[string]bool)
	tokens := make(map[string]bool)
	for _, user := range users {
		if !ids[user.ID] {
			ids[user.ID] = true
			_, _, _ = getByID(ctx, user.ID, database, cache, true)
		}
		if !emails[user.Email] {
			emails[user.Email] = true
			_, _, _ = getByEmail(ctx, user.Email, database, cache, true)
		}
		if !tokens[user.ApiToken] {
			tokens[user.ApiToken] = true
			_, _, _ = getByApiToken(ctx, user.ApiToken, database, cache, true)
		}
	}
}

func Create(ctx context.Context, user *User) error {
	return create(ctx, user, components.GetDB(), components.GetRedis())
}

func create(ctx context.Context, user *User, database *gorm.DB, cache *redis.Client) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}
	access, err := normalizeAccess(user.Access)
	if err != nil {
		return err
	}
	user.Access = access
	if err := database.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}
	refreshUserCache(ctx, database, cache, user)
	return nil
}

func Update(ctx context.Context, user *User) error {
	return update(ctx, user, components.GetDB(), components.GetRedis())
}

func update(ctx context.Context, user *User, database *gorm.DB, cache *redis.Client) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("user with an ID is required")
	}
	access, err := normalizeAccess(user.Access)
	if err != nil {
		return err
	}
	user.Access = access
	var previous User
	err = database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, user.ID).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"name":      user.Name,
			"email":     user.Email,
			"access":    user.Access,
			"password":  user.Password,
			"api_token": user.ApiToken,
		}
		return transaction.Model(&User{}).
			Where("id = ?", user.ID).
			Updates(updates).Error
	})
	if err != nil {
		return err
	}
	refreshUserCache(ctx, database, cache, &previous, user)
	return nil
}

func Delete(ctx context.Context, id uint64) error {
	return deleteUser(ctx, id, components.GetDB(), components.GetRedis())
}

func deleteUser(ctx context.Context, id uint64, database *gorm.DB, cache *redis.Client) error {
	if id == 0 {
		return fmt.Errorf("user ID is required")
	}

	var previous User
	err := database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, id).Error; err != nil {
			return err
		}
		return transaction.Delete(&User{}, id).Error
	})
	if err != nil {
		return err
	}
	refreshUserCache(ctx, database, cache, &previous)

	return nil
}
