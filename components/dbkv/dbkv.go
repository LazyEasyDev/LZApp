package dbkv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	easyroutinelib "github.com/LazyEasyDev/EasyRoutine"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const LastUpdatedKey = "__last_updated__"
const updateInterval = time.Minute

type Entry struct {
	ID          uint64 `gorm:"primaryKey" json:"id"`
	Key         string `gorm:"size:191;not null;uniqueIndex" json:"key"`
	Value       string `gorm:"type:text;not null" json:"value"`
	Description string `gorm:"type:text;not null" json:"description"`
}

func (Entry) TableName() string {
	return "dbkv"
}

var ErrNotInitialized = errors.New("dbkv cache is not initialized")

var local struct {
	sync.RWMutex
	database        *gorm.DB
	entries         map[string]Entry
	snapshotVersion string
}

var refreshMu sync.Mutex
var lifecycleMu sync.Mutex
var refreshWorker *easyroutinelib.Handle

var database *gorm.DB

func Init(ctx context.Context, db *gorm.DB) error {
	database = db
	return initWithInterval(ctx, updateInterval)
}

func initWithInterval(ctx context.Context, interval time.Duration) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if interval <= 0 {
		return fmt.Errorf("refresh interval must be positive")
	}
	if database == nil {
		return fmt.Errorf("database is not initialized")
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	if refreshWorker != nil {
		refreshWorker.Stop()
		refreshWorker.Wait()
		refreshWorker = nil
	}
	refreshMu.Lock()
	defer refreshMu.Unlock()
	clearLocal()
	if err := createTable(ctx); err != nil {
		return fmt.Errorf("create dbkv table: %w", err)
	}
	if err := ensureUpdateMarker(database.WithContext(ctx)); err != nil {
		return fmt.Errorf("initialize dbkv update marker: %w", err)
	}
	if err := loadSnapshot(ctx, database); err != nil {
		return fmt.Errorf("load dbkv cache: %w", err)
	}
	worker, err := easyroutinelib.SafeGo(ctx, func(taskCtx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				if err := refresh(taskCtx, database); err != nil && taskCtx.Err() == nil {
					slog.Error("refresh dbkv cache", "error", err)
				}
			}
		}
	}, func(recovered easyroutinelib.Panic, failures int) easyroutinelib.PanicDecision {
		slog.Error("dbkv refresh panicked", "panic", recovered.Value, "stack", string(recovered.Stack), "failures", failures)
		return easyroutinelib.PanicDecision{Retry: true, After: time.Second}
	})
	if err != nil {
		clearLocal()
		return fmt.Errorf("start dbkv refresh: %w", err)
	}
	refreshWorker = worker
	return nil
}

func Close() {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	if refreshWorker != nil {
		refreshWorker.Stop()
		refreshWorker.Wait()
		refreshWorker = nil
	}
	refreshMu.Lock()
	defer refreshMu.Unlock()
	clearLocal()
}

func clearLocal() {
	local.Lock()
	defer local.Unlock()
	local.database = nil
	local.entries = nil
	local.snapshotVersion = ""
}

func refresh(ctx context.Context, database *gorm.DB) error {
	refreshMu.Lock()
	defer refreshMu.Unlock()
	marker, err := getFromDatabase(ctx, database, LastUpdatedKey)
	if err != nil {
		return err
	}
	local.RLock()
	unchanged := local.database == database && local.entries != nil && local.snapshotVersion == marker.Value
	local.RUnlock()
	if unchanged {
		return nil
	}
	return loadSnapshot(ctx, database)
}

func loadSnapshot(ctx context.Context, database *gorm.DB) error {
	var entries []Entry
	if err := database.WithContext(ctx).Find(&entries).Error; err != nil {
		return err
	}
	snapshot := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		if !json.Valid([]byte(entry.Value)) {
			return fmt.Errorf("value for %q must be valid JSON", entry.Key)
		}
		snapshot[entry.Key] = entry
	}
	marker, exists := snapshot[LastUpdatedKey]
	if !exists {
		return fmt.Errorf("dbkv update marker is missing")
	}
	if timestamp, err := strconv.ParseInt(marker.Value, 10, 64); err != nil || timestamp < 0 {
		return fmt.Errorf("invalid last-update timestamp")
	}
	local.Lock()
	defer local.Unlock()
	local.database = database
	local.entries = snapshot
	local.snapshotVersion = marker.Value
	return nil
}

func publishChange(database *gorm.DB, key string, entry *Entry, marker *Entry) {
	local.Lock()
	defer local.Unlock()
	if local.database != database || local.entries == nil {
		return
	}
	if entry == nil {
		delete(local.entries, key)
	} else {
		local.entries[key] = *entry
	}
	local.entries[LastUpdatedKey] = *marker
}

func createTable(ctx context.Context) error {
	migrator := database.WithContext(ctx).Migrator()
	if migrator.HasTable(&Entry{}) {
		return nil
	}
	return migrator.CreateTable(&Entry{})
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("key name is required")
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 191 {
		return fmt.Errorf("key name must be valid UTF-8 and at most 191 characters")
	}
	return nil
}

func Set(ctx context.Context, name string, value any, description string) error {
	// Normalize the key name: trim spaces and convert to lowercase
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return err
	}
	name = strings.ToLower(name)
	if name == LastUpdatedKey {
		return fmt.Errorf("key %q is reserved", name)
	}
	// Encode the value as JSON
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode value for %q as JSON: %w", name, err)
	}
	// Create the entry with the encoded value and description
	entry := Entry{Key: name, Value: string(encoded), Description: description}

	refreshMu.Lock()
	defer refreshMu.Unlock()
	var stored Entry
	marker, err := withUpdateMarker(ctx, database, func(transaction *gorm.DB) error {
		if err := upsertEntry(transaction, &entry); err != nil {
			return err
		}
		return transaction.Where(clause.Eq{Column: "key", Value: name}).First(&stored).Error
	})
	if err != nil {
		return err
	}
	publishChange(database, stored.Key, &stored, marker)
	return nil
}

func upsertEntry(database *gorm.DB, entry *Entry) error {
	return database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "description"}),
	}).Create(entry).Error
}

func ensureUpdateMarker(database *gorm.DB) error {
	return database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoNothing: true,
	}).Create(&Entry{
		Key:         LastUpdatedKey,
		Value:       "0",
		Description: "Last update time in Unix nanoseconds",
	}).Error
}

func withUpdateMarker(ctx context.Context, database *gorm.DB, change func(*gorm.DB) error) (*Entry, error) {
	if database == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var marker Entry
	err := database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := ensureUpdateMarker(transaction); err != nil {
			return err
		}
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(clause.Eq{Column: "key", Value: LastUpdatedKey}).First(&marker).Error; err != nil {
			return err
		}
		previous, err := strconv.ParseInt(marker.Value, 10, 64)
		if err != nil || previous < 0 || previous == math.MaxInt64 {
			return fmt.Errorf("invalid last-update timestamp")
		}
		if err := change(transaction); err != nil {
			return err
		}
		timestamp := max(time.Now().UnixNano(), previous+1)
		marker.Value = strconv.FormatInt(timestamp, 10)
		return transaction.Model(&Entry{}).Where("id = ?", marker.ID).
			UpdateColumn("value", marker.Value).Error
	})
	if err != nil {
		return nil, err
	}
	return &marker, nil
}

func GetFromDB(ctx context.Context, name string) (*Entry, error) {
	// Normalize the key name: trim spaces and convert to lowercase
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	return getFromDatabase(ctx, database, name)
}

func getFromDatabase(ctx context.Context, database *gorm.DB, name string) (*Entry, error) {
	if database == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var entry Entry
	if err := database.WithContext(ctx).
		Where(clause.Eq{Column: "key", Value: name}).First(&entry).Error; err != nil {
		return nil, err
	}
	if !json.Valid([]byte(entry.Value)) {
		return nil, fmt.Errorf("value for %q must be valid JSON", name)
	}
	return &entry, nil
}

func Get(ctx context.Context, name string) (*Entry, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	local.RLock()
	defer local.RUnlock()
	if local.entries == nil {
		return nil, ErrNotInitialized
	}
	entry, exists := local.entries[name]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	return &entry, nil
}

func Delete(ctx context.Context, name string) error {
	// Normalize the key name: trim spaces and convert to lowercase
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return err
	}
	name = strings.ToLower(name)
	if name == LastUpdatedKey {
		return fmt.Errorf("key %q is reserved", name)
	}
	// Get the database instance

	refreshMu.Lock()
	defer refreshMu.Unlock()
	marker, err := withUpdateMarker(ctx, database, func(transaction *gorm.DB) error {
		result := transaction.Where(clause.Eq{Column: "key", Value: name}).Delete(&Entry{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	publishChange(database, name, nil, marker)
	return nil
}

type Value[ValueType any] struct {
	Value ValueType `json:"value"`
}

type StringValue = Value[string]
type Int64Value = Value[int64]
type BoolValue = Value[bool]
type Float64Value = Value[float64]

func SetString(ctx context.Context, name, value string, description string) error {
	return Set(ctx, name, StringValue{Value: value}, description)
}

func GetString(ctx context.Context, name string) (string, error) {
	return getValue[string](ctx, name)
}

func SetInt64(ctx context.Context, name string, value int64, description string) error {
	return Set(ctx, name, Int64Value{Value: value}, description)
}

func GetInt64(ctx context.Context, name string) (int64, error) {
	return getValue[int64](ctx, name)
}

func SetBool(ctx context.Context, name string, value bool, description string) error {
	return Set(ctx, name, BoolValue{Value: value}, description)
}

func GetBool(ctx context.Context, name string) (bool, error) {
	return getValue[bool](ctx, name)
}

func SetFloat64(ctx context.Context, name string, value float64, description string) error {
	return Set(ctx, name, Float64Value{Value: value}, description)
}

func GetFloat64(ctx context.Context, name string) (float64, error) {
	return getValue[float64](ctx, name)
}

func getValue[ValueType any](ctx context.Context, name string) (ValueType, error) {
	var zero ValueType
	entry, err := Get(ctx, name)
	if err != nil {
		return zero, err
	}
	var decoded Value[*ValueType]
	if err := json.Unmarshal([]byte(entry.Value), &decoded); err != nil {
		return zero, fmt.Errorf("decode value for %q: %w", name, err)
	}
	if decoded.Value == nil {
		return zero, fmt.Errorf("key %q must contain a non-null value field", name)
	}
	return *decoded.Value, nil
}
