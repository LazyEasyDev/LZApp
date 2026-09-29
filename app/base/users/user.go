package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/components"
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

const USER_LCACHE_SECONDS = 15
const USER_REDIS_DURATION = 30 * time.Minute
const USER_NOT_FOUND_LCACHE_SECONDS = 15
const USER_NOT_FOUND_REDIS_DURATION = 1 * time.Minute

func GetByID(ctx context.Context, id uint64) (*User, error) {

	cache_key := "user:id:" + fmt.Sprint(id)

	// Try to get the user from the cache first
	if cached, found := LCache.Get(cache_key); found {
		if user, ok := cached.(*User); ok {
			if user == nil {
				return nil, gorm.ErrRecordNotFound
			}
			return user, nil
		}
	}

	// If not found in the cache, fetch from the Redis
	redis_user_str, err := components.GetRedis().Get(ctx, cache_key).Result()
	if err == nil {
		var user *User
		if err := json.Unmarshal([]byte(redis_user_str), &user); err == nil {
			if user == nil {
				LCache.Set(cache_key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
				return nil, gorm.ErrRecordNotFound
			}
			// Store the user in the cache
			LCache.Set(cache_key, user, USER_LCACHE_SECONDS)
			return user, nil
		}
	}

	// Store the user in the cache after fetching from the database
	database := components.GetDB()
	var user User
	if err := database.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			LCache.Set(cache_key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
			components.GetRedis().Set(ctx, cache_key, "null", USER_NOT_FOUND_REDIS_DURATION)
		}
		return nil, err
	}

	// Store the user in the cache
	LCache.Set(cache_key, &user, USER_LCACHE_SECONDS)
	// Also store the user in Redis for future requests
	redis_user_bytes, _ := json.Marshal(&user)
	components.GetRedis().Set(ctx, cache_key, redis_user_bytes, USER_REDIS_DURATION)

	return &user, nil
}

func GetByEmail(ctx context.Context, email string) (*User, error) {

	cache_key := "user:email:" + email

	// Try to get the user from the cache first
	if cached, found := LCache.Get(cache_key); found {
		if user, ok := cached.(*User); ok {
			if user == nil {
				return nil, gorm.ErrRecordNotFound
			}
			if user.Password != "" {
				return user, nil
			}
		}
	}

	// If not found in the cache, fetch from the Redis
	redis_user_str, err := components.GetRedis().Get(ctx, cache_key).Result()
	if err == nil {
		var user *User
		if err := json.Unmarshal([]byte(redis_user_str), &user); err == nil {
			if user == nil {
				LCache.Set(cache_key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
				return nil, gorm.ErrRecordNotFound
			}
			// Store the user in the cache
			if user.Password != "" {
				LCache.Set(cache_key, user, USER_LCACHE_SECONDS)
				return user, nil
			}
		}
	}

	// Store the user in the cache after fetching from the database
	database := components.GetDB()
	var user User
	if err := database.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			LCache.Set(cache_key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
			components.GetRedis().Set(ctx, cache_key, "null", USER_NOT_FOUND_REDIS_DURATION)
		}
		return nil, err
	}

	// Store the user in the cache
	LCache.Set(cache_key, &user, USER_LCACHE_SECONDS)
	// Also store the user in Redis for future requests
	redis_user_bytes, _ := json.Marshal(&user)
	components.GetRedis().Set(ctx, cache_key, redis_user_bytes, USER_REDIS_DURATION)

	return &user, nil
}

func GetByApiToken(ctx context.Context, api_token string) (*User, error) {

	cache_key := "user:api_token:" + api_token

	// Try to get the user from the cache first
	if cached, found := LCache.Get(cache_key); found {
		if user, ok := cached.(*User); ok {
			if user == nil {
				return nil, gorm.ErrRecordNotFound
			}
			return user, nil
		}
	}

	// If not found in the cache, fetch from the Redis
	redis_user_str, err := components.GetRedis().Get(ctx, cache_key).Result()
	if err == nil {
		var user *User
		if err := json.Unmarshal([]byte(redis_user_str), &user); err == nil {
			if user == nil {
				LCache.Set(cache_key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
				return nil, gorm.ErrRecordNotFound
			}
			// Store the user in the cache
			LCache.Set(cache_key, user, USER_LCACHE_SECONDS)
			return user, nil
		}
	}

	// Store the user in the cache after fetching from the database
	database := components.GetDB()
	var user User
	if err := database.WithContext(ctx).Where("api_token = ?", api_token).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			LCache.Set(cache_key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
			components.GetRedis().Set(ctx, cache_key, "null", USER_NOT_FOUND_REDIS_DURATION)
		}
		return nil, err
	}

	// Store the user in the cache
	LCache.Set(cache_key, &user, USER_LCACHE_SECONDS)
	// Also store the user in Redis for future requests
	redis_user_bytes, _ := json.Marshal(&user)
	components.GetRedis().Set(ctx, cache_key, redis_user_bytes, USER_REDIS_DURATION)

	return &user, nil
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

func invalidateUserCache(ctx context.Context, user *User) error {
	keys := []string{
		"user:id:" + fmt.Sprint(user.ID),
		"user:email:" + user.Email,
		"user:api_token:" + user.ApiToken}
	var cacheErr error
	for _, key := range keys {
		LCache.Delete(key)
		cacheErr = errors.Join(cacheErr, components.GetRedis().Del(ctx, key).Err())
	}
	if cacheErr != nil {
		return fmt.Errorf("user cache invalidation failed: %w", cacheErr)
	}
	return nil
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
	if err := database.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}
	return invalidateUserCache(ctx, user)
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
	return errors.Join(invalidateUserCache(ctx, &previous), invalidateUserCache(ctx, user))
}

func Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("user ID is required")
	}
	database := components.GetDB()

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
	if err := invalidateUserCache(ctx, &previous); err != nil {
		slog.Error("user deleted from db but cache deletion failed", "error", err)
	}

	return nil
}
