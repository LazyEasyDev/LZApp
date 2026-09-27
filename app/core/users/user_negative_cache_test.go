package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/components"
	"github.com/LazyEasyDev/LZApp/config"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var userCacheLookups = []struct {
	name string
	key  string
	get  func(context.Context) (*User, error)
}{
	{"ID", "user:id:42", func(ctx context.Context) (*User, error) { return GetByID(ctx, 42) }},
	{"Email", "user:email:cache-test@example.com", func(ctx context.Context) (*User, error) {
		return GetByEmail(ctx, "cache-test@example.com")
	}},
	{"ApiToken", "user:api_token:cache-test-token", func(ctx context.Context) (*User, error) {
		return GetByApiToken(ctx, "cache-test-token")
	}},
}

func TestUserLookupLocalCache(test *testing.T) {
	LCache.Init(LCache.DefaultConfig())
	test.Cleanup(LCache.Close)
	for _, lookup := range userCacheLookups {
		test.Run(lookup.name, func(test *testing.T) {
			LCache.Set(lookup.key, (*User)(nil), USER_NOT_FOUND_LCACHE_SECONDS)
			user, err := lookup.get(context.Background())
			if user != nil || !errors.Is(err, gorm.ErrRecordNotFound) {
				test.Fatalf("negative hit = (%v, %v), want (nil, ErrRecordNotFound)", user, err)
			}

			expected := &User{ID: 42, Email: "cache-test@example.com", ApiToken: "cache-test-token"}
			LCache.Set(lookup.key, expected, USER_LCACHE_SECONDS)
			user, err = lookup.get(context.Background())
			if user != expected || err != nil {
				test.Fatalf("positive hit = (%v, %v), want original cached pointer", user, err)
			}
		})
	}
}

func initUserCacheIntegration(test *testing.T) {
	test.Helper()
	if os.Getenv("LZAPP_USER_CACHE_INTEGRATION") != "1" {
		test.Skip("set LZAPP_USER_CACHE_INTEGRATION=1 to use the debug MySQL and Redis configuration")
	}
	ctx := context.Background()
	config.InitConfig(config.ProfileDebug)
	appConfig := config.GetConfig()
	redisConfig := *appConfig.Redis
	redisConfig.KeyPrefix += fmt.Sprintf("user-cache-test:%d:", time.Now().UnixNano())
	databaseConfig := *appConfig.DB
	if username := os.Getenv("LZAPP_TEST_DB_USER"); username != "" {
		databaseConfig.User = username
	}
	if password, configured := os.LookupEnv("LZAPP_TEST_DB_PASSWORD"); configured {
		databaseConfig.Password = password
	}
	if databaseName := os.Getenv("LZAPP_TEST_DB_NAME"); databaseName != "" {
		databaseConfig.DBName = databaseName
	}
	if databaseConfig.Charset == "" {
		databaseConfig.Charset = "utf8mb4"
	}
	testConfig := &config.AppConfig{DB: &databaseConfig, Redis: &redisConfig}
	if err := components.InitRedis(ctx, testConfig); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = components.CloseRedis() })
	if err := components.InitDB(ctx, testConfig); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = components.CloseDB() })
	LCache.Init(LCache.DefaultConfig())
	test.Cleanup(LCache.Close)
}

func TestUserLookupNegativeCacheIntegration(test *testing.T) {
	initUserCacheIntegration(test)
	ctx := context.Background()
	database := components.GetDB()
	queryCount := 0
	var queryError error
	originalQuery := database.Callback().Query().Get("gorm:query")
	if err := database.Callback().Query().Replace("gorm:query", func(transaction *gorm.DB) {
		queryCount++
		transaction.AddError(queryError)
	}); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = database.Callback().Query().Replace("gorm:query", originalQuery) })

	for _, lookup := range userCacheLookups {
		test.Run(lookup.name, func(test *testing.T) {
			for _, scenario := range []struct {
				name string
				err  error
			}{
				{"NotFound", gorm.ErrRecordNotFound},
				{"WrappedNotFound", fmt.Errorf("query failed: %w", gorm.ErrRecordNotFound)},
				{"DatabaseFailure", errors.New("database unavailable")},
				{"Canceled", context.Canceled},
				{"Timeout", context.DeadlineExceeded},
			} {
				test.Run(scenario.name, func(test *testing.T) {
					queryError = scenario.err
					queryCount = 0
					LCache.Delete(lookup.key)
					if err := components.GetRedis().Del(ctx, lookup.key).Err(); err != nil {
						test.Fatal(err)
					}
					test.Cleanup(func() {
						LCache.Delete(lookup.key)
						_ = components.GetRedis().Del(ctx, lookup.key).Err()
					})

					user, err := lookup.get(ctx)
					if user != nil || !errors.Is(err, scenario.err) || queryCount != 1 {
						test.Fatalf("database lookup = (%v, %v), queries = %d", user, err, queryCount)
					}
					if !errors.Is(scenario.err, gorm.ErrRecordNotFound) {
						if _, found := LCache.Get(lookup.key); found {
							test.Fatal("database failure was cached locally")
						}
						if err := components.GetRedis().Get(ctx, lookup.key).Err(); !errors.Is(err, goredis.Nil) {
							test.Fatalf("Redis should be empty after database failure: %v", err)
						}
						return
					}

					cached, _, ttlSeconds, found := LCache.GetWithTTL(lookup.key)
					cachedUser, valid := cached.(*User)
					if !found || !valid || cachedUser != nil || ttlSeconds <= 0 || ttlSeconds > USER_NOT_FOUND_LCACHE_SECONDS {
						test.Fatalf("invalid local negative entry: value=%v, ttl=%d, found=%v", cached, ttlSeconds, found)
					}
					encoded, err := components.GetRedis().Get(ctx, lookup.key).Result()
					if err != nil || encoded != "null" {
						test.Fatalf("Redis negative entry = (%q, %v), want null", encoded, err)
					}
					ttl, err := components.GetRedis().TTL(ctx, lookup.key).Result()
					if err != nil || ttl <= 0 || ttl > USER_NOT_FOUND_REDIS_DURATION {
						test.Fatalf("Redis negative TTL = (%v, %v)", ttl, err)
					}
					user, err = lookup.get(ctx)
					if user != nil || !errors.Is(err, gorm.ErrRecordNotFound) || queryCount != 1 {
						test.Fatalf("local negative hit = (%v, %v), queries = %d", user, err, queryCount)
					}

					LCache.Delete(lookup.key)
					user, err = lookup.get(ctx)
					if user != nil || !errors.Is(err, gorm.ErrRecordNotFound) || queryCount != 1 {
						test.Fatalf("Redis negative hit = (%v, %v), queries = %d", user, err, queryCount)
					}
					cached, found = LCache.Get(lookup.key)
					cachedUser, valid = cached.(*User)
					if !found || !valid || cachedUser != nil {
						test.Fatal("Redis negative hit did not populate local cache")
					}
				})
			}

			test.Run("RedisPositive", func(test *testing.T) {
				LCache.Delete(lookup.key)
				test.Cleanup(func() {
					LCache.Delete(lookup.key)
					_ = components.GetRedis().Del(ctx, lookup.key).Err()
				})
				expected := &User{ID: 42, Email: "cache-test@example.com", ApiToken: "cache-test-token"}
				encoded, err := json.Marshal(expected)
				if err != nil {
					test.Fatal(err)
				}
				if err := components.GetRedis().Set(ctx, lookup.key, encoded, time.Minute).Err(); err != nil {
					test.Fatal(err)
				}
				queryCount = 0
				user, err := lookup.get(ctx)
				if err != nil || !reflect.DeepEqual(user, expected) || queryCount != 0 {
					test.Fatalf("Redis positive hit = (%v, %v), queries = %d", user, err, queryCount)
				}
				cached, found := LCache.Get(lookup.key)
				if !found || cached != user {
					test.Fatal("Redis positive hit did not populate local cache with the returned pointer")
				}
			})
		})
	}
}
