package dbkv

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
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

type Format string

const (
	FormatInt64   Format = "int64"
	FormatBool    Format = "bool"
	FormatFloat64 Format = "float64"
	FormatJSONStr Format = "json_str"
	FormatString  Format = "string"
)

type Entry struct {
	ID          uint64 `gorm:"primaryKey" json:"id"`
	Key         string `gorm:"size:191;not null;uniqueIndex" json:"key"`
	Format      Format `gorm:"size:8;not null;check:dbkv_format,format IN ('int64','bool','float64','json_str','string')" json:"format"`
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
	ErrFormatMismatch     = errors.New("dbkv format cannot be changed")
	ErrInvalidValue       = errors.New("dbkv value does not match its format")

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
	worker, err := easyroutinelib.SafeGoWithCtx(ctx, func(taskCtx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				if err := Refresh(taskCtx); err != nil && taskCtx.Err() == nil {
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

func Refresh(ctx context.Context) error {
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
		if _, err := entry.decodeValue(); err != nil {
			return nil, err
		}
		snapshot[entry.Key] = entry
	}
	marker, exists := snapshot[LastUpdatedKey]
	if !exists {
		return nil, fmt.Errorf("dbkv update marker is missing")
	}
	if marker.Format != FormatInt64 {
		return nil, fmt.Errorf("dbkv update marker must have int64 format")
	}
	return snapshot, nil
}

func createTable(ctx context.Context, connection *gorm.DB) error {
	migrator := connection.WithContext(ctx).Migrator()
	if migrator.HasTable(&Entry{}) {
		if !migrator.HasColumn(&Entry{}, "Format") {
			return fmt.Errorf("dbkv schema is outdated; drop the dbkv table before restarting")
		}
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

func (format Format) CheckFormat(value any) error {
	switch format {
	case FormatInt64:
		if _, valid := value.(int64); valid {
			return nil
		}
	case FormatBool:
		if _, valid := value.(bool); valid {
			return nil
		}
	case FormatFloat64:
		if number, valid := value.(float64); valid {
			if math.IsNaN(number) || math.IsInf(number, 0) {
				return fmt.Errorf("%w: float64 must be finite", ErrInvalidValue)
			}
			return nil
		}
	case FormatJSONStr:
		if text, valid := value.(string); valid {
			if !json.Valid([]byte(text)) {
				return fmt.Errorf("%w: json_str must be valid JSON", ErrInvalidValue)
			}
			return nil
		}
	case FormatString:
		if _, valid := value.(string); valid {
			return nil
		}
	default:
		return fmt.Errorf("%w: unsupported format %q", ErrInvalidValue, format)
	}
	return fmt.Errorf("%w: %q does not accept %T", ErrInvalidValue, format, value)
}

func (entry Entry) decodeValue() (any, error) {
	var value any
	var err error
	switch entry.Format {
	case FormatInt64:
		value, err = strconv.ParseInt(entry.Value, 10, 64)
	case FormatBool:
		if entry.Value != "true" && entry.Value != "false" {
			err = fmt.Errorf("must be true or false")
		} else {
			value = entry.Value == "true"
		}
	case FormatFloat64:
		value, err = strconv.ParseFloat(entry.Value, 64)
	case FormatJSONStr, FormatString:
		value = entry.Value
	}
	if err != nil {
		return nil, fmt.Errorf("%w: invalid %q value for %q: %w", ErrInvalidValue, entry.Format, entry.Key, err)
	}
	if err := entry.Format.CheckFormat(value); err != nil {
		return nil, fmt.Errorf("value for %q: %w", entry.Key, err)
	}
	return value, nil
}

func newEntry(name string, format Format, value any, description string) (*Entry, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	if name == LastUpdatedKey {
		return nil, fmt.Errorf("key %q is reserved", name)
	}
	if err := format.CheckFormat(value); err != nil {
		return nil, fmt.Errorf("value for %q: %w", name, err)
	}
	var encoded string
	switch typedValue := value.(type) {
	case int64:
		encoded = strconv.FormatInt(typedValue, 10)
	case bool:
		encoded = strconv.FormatBool(typedValue)
	case float64:
		encoded = strconv.FormatFloat(typedValue, 'g', -1, 64)
	case string:
		encoded = typedValue
	}
	return &Entry{Key: name, Format: format, Value: encoded, Description: description, Visible: true}, nil
}

func set(ctx context.Context, name string, format Format, value any, description string) error {
	entry, err := newEntry(name, format, value, description)
	if err != nil {
		return err
	}

	return withUpdateMarker(ctx, func(transaction *gorm.DB) error {
		return upsertEntry(transaction, entry)
	})
}

func upsertEntry(database *gorm.DB, entry *Entry) error {
	if err := database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoNothing: true,
	}).Create(entry).Error; err != nil {
		return err
	}
	var existing Entry
	if err := database.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(clause.Eq{Column: "key", Value: entry.Key}).First(&existing).Error; err != nil {
		return err
	}
	if existing.Format != entry.Format {
		return fmt.Errorf("%w for %q: stored %q, requested %q", ErrFormatMismatch, entry.Key, existing.Format, entry.Format)
	}
	return database.Model(&existing).Updates(map[string]any{
		"value":       entry.Value,
		"description": entry.Description,
		"visible":     entry.Visible,
	}).Error
}

func ensureUpdateMarker(database *gorm.DB) error {
	return database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoNothing: true,
	}).Create(&Entry{
		Key:         LastUpdatedKey,
		Format:      FormatInt64,
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
			Format:      FormatInt64,
			Value:       strconv.FormatInt(time.Now().UnixNano(), 10),
			Description: "Last update time in Unix nanoseconds",
			Visible:     false,
		})
	})
}

func GetRecordFromDB(ctx context.Context, name string) (*Entry, bool, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, false, err
	}
	name = strings.ToLower(name)
	entry, err := getFromDatabase(ctx, name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return entry, true, nil
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
	if _, err := entry.decodeValue(); err != nil {
		return nil, err
	}
	return &entry, nil
}

func GetRecord(ctx context.Context, name string) (*Entry, bool, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return nil, false, err
	}
	name = strings.ToLower(name)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	snapshot := cache.Load()
	if snapshot == nil {
		return nil, false, ErrNotInitialized
	}
	entry, exists := (*snapshot)[name]
	if !exists {
		return nil, false, nil
	}
	return &entry, true, nil
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

func SetRecord(ctx context.Context, entry Entry) error {
	value, err := entry.decodeValue()
	if err != nil {
		return err
	}
	return set(ctx, entry.Key, entry.Format, value, entry.Description)
}

func SetString(ctx context.Context, name, value string, description string) error {
	return set(ctx, name, FormatString, value, description)
}

func SetInt64(ctx context.Context, name string, value int64, description string) error {
	return set(ctx, name, FormatInt64, value, description)
}

func SetBool(ctx context.Context, name string, value bool, description string) error {
	return set(ctx, name, FormatBool, value, description)
}

func SetFloat64(ctx context.Context, name string, value float64, description string) error {
	return set(ctx, name, FormatFloat64, value, description)
}

func SetJSONStr(ctx context.Context, name, value string, description string) error {
	return set(ctx, name, FormatJSONStr, value, description)
}

func Get(ctx context.Context, name string) (any, bool, error) {
	entry, exists, err := GetRecord(ctx, name)
	if err != nil || !exists {
		return nil, false, err
	}
	value, err := entry.decodeValue()
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}
