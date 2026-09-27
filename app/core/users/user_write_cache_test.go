package users

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/LazyEasyDev/LCache"
	"github.com/LazyEasyDev/LZApp/components"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userCacheDeleteFailureHook struct {
	key string
	err error
}

func (hook *userCacheDeleteFailureHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return next
}

func (hook *userCacheDeleteFailureHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, command goredis.Cmder) error {
		if command.Name() == "del" && len(command.Args()) == 2 && command.Args()[1] == hook.key {
			return hook.err
		}
		return next(ctx, command)
	}
}

func (hook *userCacheDeleteFailureHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

func TestUserWriteCacheInvalidationIntegration(test *testing.T) {
	initUserCacheIntegration(test)
	var logOutput bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))
	test.Cleanup(func() { slog.SetDefault(previousLogger) })
	ctx := context.Background()
	database := components.GetDB()
	redisClient := components.GetRedis()
	redisFailure := errors.New("Redis delete failed")
	deleteHook := &userCacheDeleteFailureHook{err: redisFailure}
	redisClient.Raw().AddHook(deleteHook)
	previous := User{ID: 42, Email: "old@example.com", ApiToken: "stored-token", Access: "[]"}
	var readError, writeError error
	var writeRows int64
	readCount, writeCount := 0, 0

	originalQuery := database.Callback().Query().Get("gorm:query")
	if err := database.Callback().Query().Replace("gorm:query", func(transaction *gorm.DB) {
		readCount++
		locking, locked := transaction.Statement.Clauses["FOR"].Expression.(clause.Locking)
		if !locked || locking.Strength != "UPDATE" {
			test.Error("persisted user identifiers must be read with a row lock")
		}
		if readError != nil {
			transaction.AddError(readError)
			return
		}
		*transaction.Statement.Dest.(*User) = previous
		transaction.RowsAffected = 1
	}); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = database.Callback().Query().Replace("gorm:query", originalQuery) })

	writeCallback := func(transaction *gorm.DB) {
		writeCount++
		transaction.RowsAffected = writeRows
		transaction.AddError(writeError)
	}
	originalUpdate := database.Callback().Update().Get("gorm:update")
	if err := database.Callback().Update().Replace("gorm:update", writeCallback); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = database.Callback().Update().Replace("gorm:update", originalUpdate) })
	originalDelete := database.Callback().Delete().Get("gorm:delete")
	if err := database.Callback().Delete().Replace("gorm:delete", writeCallback); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = database.Callback().Delete().Replace("gorm:delete", originalDelete) })

	for _, operation := range []string{"Update", "Delete"} {
		test.Run(operation, func(test *testing.T) {
			for _, scenario := range []struct {
				name         string
				readError    error
				writeError   error
				negative     bool
				redisFailure bool
				writeRows    int64
			}{
				{name: "Success", writeRows: 1},
				{name: "NegativeEntries", negative: true, writeRows: 1},
				{name: "MissingUser", readError: gorm.ErrRecordNotFound},
				{name: "ReadFailure", readError: errors.New("database read failed")},
				{name: "WriteFailure", writeError: errors.New("database write failed")},
				{name: "RedisFailure", redisFailure: true, writeRows: 1},
				{name: "UnchangedUpdate", writeRows: 0},
			} {
				if operation == "Delete" && scenario.name == "UnchangedUpdate" {
					continue
				}
				test.Run(scenario.name, func(test *testing.T) {
					logOutput.Reset()
					readError, writeError, writeRows = scenario.readError, scenario.writeError, scenario.writeRows
					readCount, writeCount = 0, 0
					input := &User{ID: previous.ID, Email: "new@example.com", ApiToken: "unrelated-token", Access: "[]"}
					if scenario.name == "UnchangedUpdate" {
						input.Email = previous.Email
					}
					keys := []string{"user:id:42", "user:email:" + previous.Email, "user:api_token:" + previous.ApiToken}
					if operation == "Update" && input.Email != previous.Email {
						keys = append(keys, "user:email:"+input.Email)
					}
					unrelatedKey := "user:api_token:" + input.ApiToken
					allKeys := append(keys, unrelatedKey)
					test.Cleanup(func() {
						deleteHook.key = ""
						for _, key := range allKeys {
							LCache.Delete(key)
							if err := redisClient.Del(ctx, key).Err(); err != nil {
								test.Error(err)
							}
						}
					})
					for _, key := range allKeys {
						cachedUser := input
						encoded := `{"id":42,"email":"old@example.com","api_token":"stored-token"}`
						if scenario.negative || key == "user:email:new@example.com" {
							cachedUser = nil
							encoded = "null"
						}
						LCache.Set(key, cachedUser, USER_LCACHE_SECONDS)
						if err := redisClient.Set(ctx, key, encoded, USER_REDIS_DURATION).Err(); err != nil {
							test.Fatal(err)
						}
					}
					if scenario.redisFailure {
						deleteHook.key = redisClient.Key(keys[0])
					}

					var err error
					if operation == "Update" {
						err = Update(ctx, input)
					} else {
						err = Delete(ctx, previous.ID)
					}
					expectedError := scenario.readError
					if expectedError == nil {
						expectedError = scenario.writeError
					}
					if scenario.redisFailure && operation == "Update" {
						expectedError = redisFailure
					}
					if !errors.Is(err, expectedError) {
						test.Fatalf("%s error = %v, want %v", operation, err, expectedError)
					}
					if scenario.redisFailure && operation == "Delete" && !strings.Contains(logOutput.String(), redisFailure.Error()) {
						test.Fatal("Delete did not log its Redis cache invalidation failure")
					}
					expectedWrites := 1
					if scenario.readError != nil {
						expectedWrites = 0
					}
					if readCount != 1 || writeCount != expectedWrites {
						test.Fatalf("reads = %d, writes = %d; want 1, %d", readCount, writeCount, expectedWrites)
					}
					writeFailed := scenario.readError != nil || scenario.writeError != nil
					for _, key := range allKeys {
						shouldRemain := writeFailed || key == unrelatedKey
						if _, found := LCache.Get(key); found != shouldRemain {
							test.Errorf("local key %s: found = %v, want %v", key, found, shouldRemain)
						}
						redisShouldRemain := shouldRemain || (scenario.redisFailure && key == keys[0])
						redisErr := redisClient.Get(ctx, key).Err()
						if redisShouldRemain {
							if redisErr != nil {
								test.Errorf("Redis key %s should remain: %v", key, redisErr)
							}
						} else if !errors.Is(redisErr, goredis.Nil) {
							test.Errorf("Redis key %s was not invalidated: %v", key, redisErr)
						}
					}
				})
			}
		})
	}
}
