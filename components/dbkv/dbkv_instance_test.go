package dbkv

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openInstanceTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	return database
}

func newInstanceTestStore(t *testing.T, database *gorm.DB) *DBKV {
	t.Helper()
	store, err := New(context.Background(), database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func TestDBKVInstancesAreIndependent(t *testing.T) {
	ctx := context.Background()
	firstDatabase := openInstanceTestDatabase(t)
	first := newInstanceTestStore(t, firstDatabase)
	second := newInstanceTestStore(t, openInstanceTestDatabase(t))

	if err := first.SetString(ctx, " Shared.Key ", "first", "First store"); err != nil {
		t.Fatal(err)
	}
	if err := second.SetString(ctx, "shared.key", "second", "Second store"); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		store *DBKV
		want  string
	}{
		{store: first, want: "first"},
		{store: second, want: "second"},
	} {
		if err := testCase.store.refresh(ctx); err != nil {
			t.Fatal(err)
		}
		if value, err := testCase.store.GetString(ctx, " SHARED.KEY "); err != nil || value != testCase.want {
			t.Fatalf("GetString = %q, %v; want %q", value, err, testCase.want)
		}
		entry, err := testCase.store.GetFromDB(ctx, " SHARED.KEY ")
		if err != nil {
			t.Fatal(err)
		}
		var decoded StringValue
		if err := json.Unmarshal([]byte(entry.Value), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Value != testCase.want {
			t.Fatalf("GetFromDB value = %q; want %q", decoded.Value, testCase.want)
		}
	}

	if err := first.Delete(ctx, " SHARED.KEY "); err != nil {
		t.Fatal(err)
	}
	if err := first.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(ctx, "shared.key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Get deleted key error = %v; want ErrRecordNotFound", err)
	}
	if _, err := first.GetFromDB(ctx, "shared.key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetFromDB deleted key error = %v; want ErrRecordNotFound", err)
	}
	first.Close()
	first.Close()
	if _, err := first.Get(ctx, "shared.key"); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Get after Close error = %v; want ErrNotInitialized", err)
	}
	if value, err := second.GetString(ctx, "shared.key"); err != nil || value != "second" {
		t.Fatalf("second store after first Close = %q, %v; want second", value, err)
	}
	if err := second.SetString(ctx, "shared.key", "still open", ""); err != nil {
		t.Fatal(err)
	}
	if err := second.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if value, err := second.GetString(ctx, "shared.key"); err != nil || value != "still open" {
		t.Fatalf("second store update after first Close = %q, %v; want still open", value, err)
	}
	connection, err := firstDatabase.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.PingContext(ctx); err != nil {
		t.Fatalf("Close closed the supplied database: %v", err)
	}
}

func TestDBKVWritesWaitForRefresh(t *testing.T) {
	ctx := context.Background()
	store := newInstanceTestStore(t, openInstanceTestDatabase(t))
	initial := store.cache.Load()
	if marker := (*initial)[LastUpdatedKey]; marker.Visible || marker.Value != "0" {
		t.Fatalf("initial marker = %+v; want hidden marker with value 0", marker)
	}
	if err := store.SetString(ctx, "key", "first", ""); err != nil {
		t.Fatal(err)
	}
	if store.cache.Load() != initial {
		t.Fatal("Set changed the local cache")
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Get before refresh error = %v; want ErrRecordNotFound", err)
	}
	entry, err := store.GetFromDB(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Visible || entry.Value != `{"value":"first"}` {
		t.Fatalf("stored entry = %+v; want visible entry with first value", entry)
	}
	marker, err := store.GetFromDB(ctx, LastUpdatedKey)
	if err != nil {
		t.Fatal(err)
	}
	if marker.Visible || marker.Value == "0" {
		t.Fatalf("stored marker = %+v; want hidden marker with updated timestamp", marker)
	}
	if err := store.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	first := store.cache.Load()
	if first == initial || (*first)[LastUpdatedKey] != *marker {
		t.Fatal("refresh did not replace the snapshot with the database marker")
	}
	if _, exists := (*initial)["key"]; exists || (*initial)[LastUpdatedKey].Value != "0" {
		t.Fatal("refresh modified the initial snapshot")
	}
	if err := store.SetString(ctx, "key", "second", ""); err != nil {
		t.Fatal(err)
	}
	if value, err := store.GetString(ctx, "key"); err != nil || value != "first" {
		t.Fatalf("cached value after update = %q, %v; want first", value, err)
	}
	if err := store.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	second := store.cache.Load()
	if second == first || (*first)["key"].Value != `{"value":"first"}` {
		t.Fatal("refresh modified the previous snapshot")
	}
	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if store.cache.Load() != second {
		t.Fatal("Delete changed the local cache")
	}
	if value, err := store.GetString(ctx, "key"); err != nil || value != "second" {
		t.Fatalf("cached value after Delete = %q, %v; want second", value, err)
	}
	if _, err := store.GetFromDB(ctx, "key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetFromDB after Delete error = %v; want ErrRecordNotFound", err)
	}
	if err := store.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Get after refresh error = %v; want ErrRecordNotFound", err)
	}
	unchanged := store.cache.Load()
	if err := store.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if store.cache.Load() != unchanged {
		t.Fatal("refresh replaced the snapshot without a marker change")
	}
	store.Close()
	if store.cache.Load() != nil || (*second)["key"].Value != `{"value":"second"}` {
		t.Fatal("Close did not clear the cache while preserving existing snapshots")
	}
}

func TestDBKVConcurrentSnapshotAccess(test *testing.T) {
	ctx := context.Background()
	store := newInstanceTestStore(test, openInstanceTestDatabase(test))
	keys := []string{"first", "second", "third", "fourth"}
	for _, key := range keys {
		if err := store.SetInt64(ctx, key, 0, ""); err != nil {
			test.Fatal(err)
		}
	}
	if err := store.refresh(ctx); err != nil {
		test.Fatal(err)
	}
	start := make(chan struct{})
	writesDone := make(chan struct{})
	var writers sync.WaitGroup
	for _, key := range keys {
		writers.Go(func() {
			<-start
			for value := int64(1); value <= 30; value++ {
				if err := store.SetInt64(ctx, key, value, ""); err != nil {
					test.Error(err)
					return
				}
			}
		})
	}
	var observers sync.WaitGroup
	for _, key := range keys {
		observers.Go(func() {
			<-start
			for {
				select {
				case <-writesDone:
					return
				default:
					if _, err := store.GetInt64(ctx, key); err != nil {
						test.Error(err)
						return
					}
				}
			}
		})
	}
	observers.Go(func() {
		<-start
		for {
			select {
			case <-writesDone:
				return
			default:
				if err := store.refresh(ctx); err != nil {
					test.Error(err)
					return
				}
			}
		}
	})
	close(start)
	writers.Wait()
	close(writesDone)
	observers.Wait()
	if err := store.refresh(ctx); err != nil {
		test.Fatal(err)
	}
	for _, key := range keys {
		if value, err := store.GetInt64(ctx, key); err != nil || value != 30 {
			test.Fatalf("final value for %q = %d, %v; want 30", key, value, err)
		}
	}
}

func TestDBKVConcurrentClose(test *testing.T) {
	ctx := context.Background()
	store := newInstanceTestStore(test, openInstanceTestDatabase(test))
	start := make(chan struct{})
	var operations sync.WaitGroup
	for range 4 {
		operations.Go(func() {
			<-start
			store.Close()
		})
	}
	operations.Go(func() {
		<-start
		for value := int64(0); value < 30; value++ {
			if err := store.SetInt64(ctx, "key", value, ""); err != nil {
				test.Error(err)
				return
			}
			if _, err := store.GetInt64(ctx, "key"); err != nil && !errors.Is(err, ErrNotInitialized) && !errors.Is(err, gorm.ErrRecordNotFound) {
				test.Error(err)
				return
			}
		}
	})
	close(start)
	operations.Wait()
	if _, err := store.Get(ctx, "key"); !errors.Is(err, ErrNotInitialized) {
		test.Fatalf("Get after concurrent Close error = %v; want ErrNotInitialized", err)
	}
}

func TestDBKVNewRejectsInvalidInput(t *testing.T) {
	database := openInstanceTestDatabase(t)
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	for _, testCase := range []struct {
		name      string
		ctx       context.Context
		database  *gorm.DB
		wantError string
	}{
		{name: "nil context", database: database, wantError: "context is required"},
		{name: "canceled context", ctx: canceledContext, database: database, wantError: context.Canceled.Error()},
		{name: "nil database", ctx: context.Background(), wantError: "database is not initialized"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store, err := New(testCase.ctx, testCase.database)
			if store != nil {
				store.Close()
				t.Fatal("New returned an instance for invalid input")
			}
			if err == nil || err.Error() != testCase.wantError {
				t.Fatalf("New error = %v; want %q", err, testCase.wantError)
			}
		})
	}
}

func TestDBKVNewLoadsExistingSnapshot(t *testing.T) {
	ctx := context.Background()
	database := openInstanceTestDatabase(t)
	if err := database.Migrator().CreateTable(&Entry{}); err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Key: LastUpdatedKey, Value: "123", Description: "Existing marker"},
		{Key: "existing", Value: `{"value":"saved"}`, Description: "Existing value", Visible: true},
	}
	if err := database.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	store := newInstanceTestStore(t, database)
	marker, err := store.Get(ctx, LastUpdatedKey)
	if err != nil {
		t.Fatal(err)
	}
	if *marker != entries[0] {
		t.Fatalf("New changed the existing marker: got %+v; want %+v", *marker, entries[0])
	}
	if value, err := store.GetString(ctx, "existing"); err != nil || value != "saved" {
		t.Fatalf("initial snapshot value = %q, %v; want saved", value, err)
	}
}

func TestDBKVNewRejectsInvalidSnapshot(t *testing.T) {
	ctx := context.Background()
	database := openInstanceTestDatabase(t)
	healthy := newInstanceTestStore(t, database)
	if err := database.Create(&Entry{Key: "invalid", Value: "not JSON"}).Error; err != nil {
		t.Fatal(err)
	}
	store, err := New(ctx, database)
	if store != nil {
		store.Close()
		t.Fatal("New returned an instance for an invalid snapshot")
	}
	if err == nil {
		t.Fatal("New accepted invalid stored JSON")
	}
	if _, err := healthy.Get(ctx, LastUpdatedKey); err != nil {
		t.Fatalf("failed New disturbed an existing cache: %v", err)
	}
}

func TestDBKVTypedMethods(t *testing.T) {
	ctx := context.Background()
	store := newInstanceTestStore(t, openInstanceTestDatabase(t))
	if err := store.SetString(ctx, "string", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetInt64(ctx, "integer", -42, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBool(ctx, "boolean", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetFloat64(ctx, "float", 3.5, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if value, err := store.GetString(ctx, "string"); err != nil || value != "" {
		t.Fatalf("GetString = %q, %v; want empty string", value, err)
	}
	if value, err := store.GetInt64(ctx, "integer"); err != nil || value != -42 {
		t.Fatalf("GetInt64 = %d, %v; want -42", value, err)
	}
	if value, err := store.GetBool(ctx, "boolean"); err != nil || value {
		t.Fatalf("GetBool = %v, %v; want false", value, err)
	}
	if value, err := store.GetFloat64(ctx, "float"); err != nil || value != 3.5 {
		t.Fatalf("GetFloat64 = %g, %v; want 3.5", value, err)
	}
}

func TestDBKVMarkerFailureRollsBackWrites(test *testing.T) {
	ctx := context.Background()
	database := openInstanceTestDatabase(test)
	store := newInstanceTestStore(test, database)
	if err := store.SetString(ctx, "existing", "original", "Original description"); err != nil {
		test.Fatal(err)
	}
	if err := store.refresh(ctx); err != nil {
		test.Fatal(err)
	}
	snapshot := store.cache.Load()
	if err := database.Exec(`CREATE TRIGGER reject_marker_update
		BEFORE UPDATE OF value ON dbkv
		WHEN OLD.key = '__last_updated__'
		BEGIN SELECT RAISE(ABORT, 'marker update rejected'); END`).Error; err != nil {
		test.Fatal(err)
	}
	for _, operation := range []struct {
		name string
		run  func() error
	}{
		{name: "insert", run: func() error { return store.SetString(ctx, "new", "value", "") }},
		{name: "update", run: func() error { return store.SetString(ctx, "existing", "changed", "Changed") }},
		{name: "delete", run: func() error { return store.Delete(ctx, "existing") }},
	} {
		test.Run(operation.name, func(test *testing.T) {
			if err := operation.run(); err == nil {
				test.Fatal("write succeeded despite marker update failure")
			}
			for _, key := range []string{"existing", LastUpdatedKey} {
				entry, err := store.GetFromDB(ctx, key)
				if err != nil {
					test.Fatal(err)
				}
				if *entry != (*snapshot)[key] {
					test.Fatalf("failed write changed %q: got %+v; want %+v", key, *entry, (*snapshot)[key])
				}
			}
			if _, err := store.GetFromDB(ctx, "new"); !errors.Is(err, gorm.ErrRecordNotFound) {
				test.Fatalf("failed insert error = %v; want ErrRecordNotFound", err)
			}
			if store.cache.Load() != snapshot {
				test.Fatal("failed write changed the local cache")
			}
		})
	}
}

func TestDBKVSetOverwritesUpdateMarker(test *testing.T) {
	ctx := context.Background()
	database := openInstanceTestDatabase(test)
	store := newInstanceTestStore(test, database)
	for _, previous := range []string{"9223372036854775807", "not a timestamp"} {
		if err := database.Model(&Entry{}).Where("key = ?", LastUpdatedKey).
			Updates(map[string]any{"value": previous, "visible": true}).Error; err != nil {
			test.Fatal(err)
		}
		before := time.Now().UnixNano()
		if err := store.SetString(ctx, "key", "value", ""); err != nil {
			test.Fatal(err)
		}
		marker, err := store.GetFromDB(ctx, LastUpdatedKey)
		if err != nil {
			test.Fatal(err)
		}
		var timestamp int64
		if err := json.Unmarshal([]byte(marker.Value), &timestamp); err != nil {
			test.Fatal(err)
		}
		if marker.Visible || timestamp < before || timestamp > time.Now().UnixNano() {
			test.Fatalf("updated marker = %+v; want hidden marker with current Unix nanoseconds", marker)
		}
	}
	marker, err := store.GetFromDB(ctx, LastUpdatedKey)
	if err != nil {
		test.Fatal(err)
	}
	if err := store.Delete(ctx, "missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		test.Fatalf("Delete missing key error = %v; want ErrRecordNotFound", err)
	}
	unchanged, err := store.GetFromDB(ctx, LastUpdatedKey)
	if err != nil {
		test.Fatal(err)
	}
	if *unchanged != *marker {
		test.Fatal("missing-key delete changed the marker")
	}
}

func TestDBKVRefreshRetainsLastGoodSnapshot(test *testing.T) {
	ctx := context.Background()
	store := newInstanceTestStore(test, openInstanceTestDatabase(test))
	if err := store.SetString(ctx, "valid", "saved", ""); err != nil {
		test.Fatal(err)
	}
	if err := store.refresh(ctx); err != nil {
		test.Fatal(err)
	}
	snapshot := store.cache.Load()
	if err := store.withUpdateMarker(ctx, func(transaction *gorm.DB) error {
		return transaction.Create(&Entry{Key: "invalid", Value: "not JSON", Visible: true}).Error
	}); err != nil {
		test.Fatal(err)
	}
	if err := store.refresh(ctx); err == nil {
		test.Fatal("refresh accepted invalid stored JSON")
	}
	if store.cache.Load() != snapshot {
		test.Fatal("failed refresh replaced the last good snapshot")
	}
	if err := store.Delete(ctx, "invalid"); err != nil {
		test.Fatal(err)
	}
	if err := store.refresh(ctx); err != nil {
		test.Fatal(err)
	}
	if store.cache.Load() == snapshot {
		test.Fatal("refresh did not recover after removing invalid data")
	}
	if value, err := store.GetString(ctx, "valid"); err != nil || value != "saved" {
		test.Fatalf("value after recovery = %q, %v; want saved", value, err)
	}
}

func TestDBKVWorkerLifecycle(t *testing.T) {
	ctx := context.Background()
	database := openInstanceTestDatabase(t)
	writer := newInstanceTestStore(t, database)
	readerContext, cancelReader := context.WithCancel(ctx)
	t.Cleanup(cancelReader)
	reader, err := newWithInterval(readerContext, database, 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)

	waitForValue := func(want string) {
		t.Helper()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		deadline := time.NewTimer(3 * time.Second)
		defer deadline.Stop()
		for {
			value, err := reader.GetString(ctx, "shared.key")
			if err == nil && value == want {
				return
			}
			select {
			case <-ticker.C:
			case <-deadline.C:
				t.Fatalf("reader did not refresh: got %q, %v; want %q", value, err, want)
			}
		}
	}

	if err := writer.SetString(ctx, "shared.key", "first", ""); err != nil {
		t.Fatal(err)
	}
	waitForValue("first")
	writer.Close()
	replacement := newInstanceTestStore(t, database)
	if err := replacement.SetString(ctx, "shared.key", "replacement", ""); err != nil {
		t.Fatal(err)
	}
	waitForValue("replacement")

	worker := reader.refreshWorker
	workerDone := make(chan struct{})
	go func() {
		worker.Wait()
		close(workerDone)
	}()
	cancelReader()
	select {
	case <-workerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("canceling the constructor context did not stop the worker")
	}
	if err := replacement.SetString(ctx, "shared.key", "after cancellation", ""); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.GetString(ctx, "shared.key"); err != nil || value != "replacement" {
		t.Fatalf("canceled reader snapshot = %q, %v; want replacement", value, err)
	}
	entry, err := reader.GetFromDB(ctx, "shared.key")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Value != `{"value":"after cancellation"}` {
		t.Fatalf("direct read = %q; want latest database value", entry.Value)
	}
}
