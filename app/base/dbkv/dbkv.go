package dbkv

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	Visible     bool   `gorm:"not null" json:"visible"`
}

func (Entry) TableName() string {
	return "dbkv"
}

var (
	ErrNotInitialized     = errors.New("dbkv is not initialized")
	ErrAlreadyInitialized = errors.New("dbkv is already initialized")

	lifecycleMu   sync.Mutex
	database      atomic.Pointer[gorm.DB]
	cache         atomic.Pointer[map[string]Entry]
	refreshWorker *easyroutinelib.Handle
)

func Init(ctx context.Context, connection *gorm.DB) error {
	return initWithInterval(ctx, connection, updateInterval)
}

func initWithInterval(ctx context.Context, connection *gorm.DB, interval time.Duration) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if interval <= 0 {
		return fmt.Errorf("refresh interval must be positive")
	}
	if connection == nil {
		return fmt.Errorf("database is not initialized")
	}
	if database.Load() != nil {
		return ErrAlreadyInitialized
	}

	if err := createTable(ctx, connection); err != nil {
		return fmt.Errorf("create dbkv table: %w", err)
	}
	if err := ensureUpdateMarker(connection.WithContext(ctx)); err != nil {
		return fmt.Errorf("initialize dbkv update marker: %w", err)
	}
	snapshot, err := loadSnapshot(ctx, connection)
	if err != nil {
		return fmt.Errorf("load dbkv cache: %w", err)
	}
	database.Store(connection)
	cache.Store(&snapshot)
	worker, err := easyroutinelib.SafeGo(ctx, func(taskCtx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				if err := refresh(taskCtx); err != nil && taskCtx.Err() == nil {
					slog.Error("refresh dbkv cache", "error", err)
				}
			}
		}
	}, func(recovered easyroutinelib.Panic, failures int) easyroutinelib.PanicDecision {
		slog.Error("dbkv refresh panicked", "panic", recovered.Value, "stack", string(recovered.Stack), "failures", failures)
		return easyroutinelib.PanicDecision{Retry: true, After: time.Second}
	})
	if err != nil {
		database.Store(nil)
		cache.Store(nil)
		return fmt.Errorf("start dbkv refresh: %w", err)
	}
	refreshWorker = worker
	return nil
}

func Close() {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if database.Load() == nil && refreshWorker == nil {
		return
	}
	slog.Info("closing dbkv store")
	if refreshWorker != nil {
		refreshWorker.Stop()
		refreshWorker.Wait()
		refreshWorker = nil
	}
	database.Store(nil)
	cache.Store(nil)
	slog.Info("dbkv store closed")
}

func refresh(ctx context.Context) error {
	marker, err := getFromDatabase(ctx, LastUpdatedKey)
	if err != nil {
		return err
	}
	current := cache.Load()
	if current != nil && (*current)[LastUpdatedKey].Value == marker.Value {
		return nil
	}
	snapshot, err := loadSnapshot(ctx, database.Load())
	if err != nil {
		return err
	}
	cache.Store(&snapshot)
	return nil
}

func loadSnapshot(ctx context.Context, connection *gorm.DB) (map[string]Entry, error) {
	if connection == nil {
		return nil, ErrNotInitialized
	}
	var entries []Entry
	if err := connection.WithContext(ctx).Find(&entries).Error; err != nil {
		return nil, err
	}
	snapshot := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		if !json.Valid([]byte(entry.Value)) {
			return nil, fmt.Errorf("value for %q must be valid JSON", entry.Key)
		}
		snapshot[entry.Key] = entry
	}
	if _, exists := snapshot[LastUpdatedKey]; !exists {
		return nil, fmt.Errorf("dbkv update marker is missing")
	}
	return snapshot, nil
}

func createTable(ctx context.Context, connection *gorm.DB) error {
	migrator := connection.WithContext(ctx).Migrator()
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
	entry := Entry{Key: name, Value: string(encoded), Description: description, Visible: true}

	return withUpdateMarker(ctx, func(transaction *gorm.DB) error {
		return upsertEntry(transaction, &entry)
	})
}

func upsertEntry(database *gorm.DB, entry *Entry) error {
	return database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "description", "visible"}),
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
		Visible:     false,
	}).Error
}

func withUpdateMarker(ctx context.Context, change func(*gorm.DB) error) error {
	connection := database.Load()
	if connection == nil {
		return ErrNotInitialized
	}
	return connection.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := change(transaction); err != nil {
			return err
		}
		return upsertEntry(transaction, &Entry{
			Key:         LastUpdatedKey,
			Value:       strconv.FormatInt(time.Now().UnixNano(), 10),
			Description: "Last update time in Unix nanoseconds",
			Visible:     false,
		})
	})
}

func GetFromDB(ctx context.Context, name string) (*Entry, error) {
	// Normalize the key name: trim spaces and convert to lowercase
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	return getFromDatabase(ctx, name)
}

func getFromDatabase(ctx context.Context, name string) (*Entry, error) {
	connection := database.Load()
	if connection == nil {
		return nil, ErrNotInitialized
	}
	var entry Entry
	if err := connection.WithContext(ctx).
		Where(clause.Eq{Column: "key", Value: name}).First(&entry).Error; err != nil {
		return nil, err
	}
	if !json.Valid([]byte(entry.Value)) {
		return nil, fmt.Errorf("value for %q must be valid JSON", name)
	}
	return &entry, nil
}

func GetRecord(ctx context.Context, name string) (*Entry, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot := cache.Load()
	if snapshot == nil {
		return nil, ErrNotInitialized
	}
	entry, exists := (*snapshot)[name]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	return &entry, nil
}

func List(ctx context.Context) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot := cache.Load()
	if snapshot == nil {
		return nil, ErrNotInitialized
	}
	entries := make([]Entry, 0, len(*snapshot))
	for _, entry := range *snapshot {
		if entry.Visible {
			entries = append(entries, entry)
		}
	}
	slices.SortFunc(entries, func(first, second Entry) int {
		return cmp.Compare(first.ID, second.ID)
	})
	return entries, nil
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

	return withUpdateMarker(ctx, func(transaction *gorm.DB) error {
		result := transaction.Where(clause.Eq{Column: "key", Value: name}).Delete(&Entry{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func SetString(ctx context.Context, name, value string, description string) error {
	return Set(ctx, name, value, description)
}

func GetString(ctx context.Context, name string) (string, error) {
	return Get[string](ctx, name)
}

func SetInt64(ctx context.Context, name string, value int64, description string) error {
	return Set(ctx, name, value, description)
}

func GetInt64(ctx context.Context, name string) (int64, error) {
	return Get[int64](ctx, name)
}

func SetBool(ctx context.Context, name string, value bool, description string) error {
	return Set(ctx, name, value, description)
}

func GetBool(ctx context.Context, name string) (bool, error) {
	return Get[bool](ctx, name)
}

func SetFloat64(ctx context.Context, name string, value float64, description string) error {
	return Set(ctx, name, value, description)
}

func GetFloat64(ctx context.Context, name string) (float64, error) {
	return Get[float64](ctx, name)
}

func Get[ValueType any](ctx context.Context, name string) (ValueType, error) {
	var zero ValueType
	entry, err := GetRecord(ctx, name)
	if err != nil {
		return zero, err
	}
	var decoded *ValueType
	if err := json.Unmarshal([]byte(entry.Value), &decoded); err != nil {
		return zero, fmt.Errorf("decode value for %q: %w", name, err)
	}
	if decoded == nil {
		return zero, fmt.Errorf("key %q must contain a non-null value", name)
	}
	return *decoded, nil
}
