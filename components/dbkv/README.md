# Database Key-Value Store

`dbkv.New(ctx, database)` returns a `*dbkv.DBKV` with context-aware methods.
Each instance owns its database reference, local cache, and refresh
worker. Instances can use different databases or share a GORM connection pool.
The `dbkv` table has five columns: an auto-incrementing `id`, a unique `key`,
a JSON-encoded text `value`, a text `description`, and a boolean `visible`.
Setters mark normal entries visible; the reserved update marker is hidden.
Visibility is metadata, not access control: reads and snapshots include all entries.

`Entry.Value` is a plain Go `string` containing JSON text, stored in a `TEXT`
column. `Set` JSON-encodes values before writing. Cache loading and `GetFromDB`
reject invalid stored JSON. Direct GORM or SQL writes bypass this validation. Database-specific text
size limits apply (MySQL `TEXT` holds at most 65,535 bytes).

## Initialization

`components.Init` creates the application's store after initializing the
database and exposes it as `components.GetComponents().DBKV`. Component
shutdown closes that store before closing the database. For standalone use,
pass an existing `*gorm.DB` to the constructor:

```go
store, err := dbkv.New(ctx, database)
if err != nil {
    return err
}
defer store.Close()
```

`New` creates the table if missing, initializes the reserved update marker if
absent, and loads the whole table before returning. It then starts a SafeGo
worker that checks the database marker every 60 seconds. A changed marker
causes a complete cache replacement, including removal of deleted keys.

Initial-load failures return an error. Later refresh failures are logged and
leave the last good snapshot intact; the next poll retries. Normal reads use
only that local snapshot, so they can continue during a database outage.

Cache snapshots are immutable and published with an atomic pointer swap.
After the initial load, only the single refresh worker publishes snapshots.
Writes never read or change the cache. There are no application mutexes:
readers load the current map atomically, and each refresh builds and swaps in
a complete replacement.

Canceling the initialization context stops the worker. `Close()` stops and
waits for it and clears that instance's cache, without closing the supplied
database or affecting other instances. Repeated `Close` calls are safe. Each
`New` call creates an independent store; reuse the instance across requests
and close it when its owner shuts down.

Only missing tables are created; existing schemas are not upgraded. This
pre-release project does not support schema migrations. When models change,
manually recreate the affected development tables before restarting the app;
recreating tables deletes their data. Existing tables without the new
`visible` column must be recreated before using this version.

`components.GetComponents()` returns a pointer to the shared runtime. Initialize
or replace its fields only when no operations are running; runtime mutation
is not synchronized. A store's database reference is fixed at construction.
To switch databases, close the old store and construct a new one. Normal
concurrent operations on a store share its GORM connection pool.

## Update Marker

`LastUpdatedKey` (`__last_updated__`) is reserved. Its value is a bare JSON
integer containing Unix nanoseconds, initially `0` before the first mutation.
Every successful `Set` and `Delete` changes the row and marker in one
transaction. The marker is overwritten with `time.Now().UnixNano()` and
`Visible: false`; the previous value is not read. It is a timestamp, not a
strictly increasing version counter. Failed writes roll back both changes,
and missing-key deletes do not update it.

The marker cannot be written or deleted through public setters or `Delete`.
Read it with `Get` or `GetFromDB`, then decode `entry.Value` into an `int64`;
it is not a typed-helper `{"value":...}` wrapper.

All writes, including those from this instance, appear in the local cache on
the next successful poll. Every minute, the worker compares the database
marker with the marker in the cached map. Different values trigger a full
table reload and atomic map replacement; equal values skip the reload.
Use `GetFromDB` when an immediate read of a committed write is needed.
External writers must update the marker in the same transaction; direct SQL
changes without a marker update are not detected reliably.

## Set, Get, Delete

- `Set(ctx, name, value, description)` JSON-encodes a Go value and atomically
  inserts or updates the key. Structs, maps, slices, and JSON primitives are
  supported, including `nil` as JSON `null`. Invalid JSON, unsupported Go
    values, and non-finite floats return an error without writing a row. It sets
    `Visible: true` and does not update the local cache.
- `Get(ctx, name)` returns a copy of the locally cached `Entry`, with no database
    query or fallback. On a zero-value `DBKV` or after `Close`, it returns
    `ErrNotInitialized`. `entry.Value` contains JSON text; decode it using
    `json.Unmarshal([]byte(entry.Value), &destination)`.
- `GetFromDB(ctx, name)` is the former direct-read `Get`. It reads the current
    database value without reading or changing the cache.
- `Delete(ctx, name)` deletes that key transactionally; the local cache keeps
    its previous entry until the next refresh. `Get`, `GetFromDB`, and `Delete` return
    `gorm.ErrRecordNotFound` for missing keys; check it using `errors.Is`.

Names are trimmed, validated as non-empty UTF-8 with at most 191 characters,
and converted to lowercase for every operation. SQL statements bind names as
parameters. Cache keys use exact normalized strings; direct database lookups
also follow the database column's collation.

Every setter requires a description string and overwrites the stored
description. Pass `""` when no description is needed. Updating a key preserves
its ID.

JSON-encoding an `Entry` serializes its `value` field as a quoted string, not
as an embedded JSON object. There is no custom JSON or SQL adapter type.

```go
if err := store.Set(ctx, "service.options", map[string]any{
    "enabled": true,
    "retries": 3,
}, "Service settings"); err != nil {
    return err
}

entry, err := store.GetFromDB(ctx, "service.options")
if err != nil {
    return err
}
var options map[string]any
if err := json.Unmarshal([]byte(entry.Value), &options); err != nil {
    return err
}

if err := store.Delete(ctx, "service.options"); err != nil {
    return err
}
```

For JSON that is already encoded, pass `json.RawMessage`:

```go
if err := store.Set(ctx, "options", json.RawMessage(`{"enabled":true}`), ""); err != nil {
    return err
}
```

Passing an ordinary string JSON-encodes it as a string, not as an object.

## Typed Helpers

The paired helpers are `SetString`/`GetString`,
`SetInt64`/`GetInt64`, `SetBool`/`GetBool`, and `SetFloat64`/`GetFloat64`.
All setters require the same description argument. All typed getters read
from the local cache through `Get`, so they reflect writes only after a refresh.

```go
if err := store.SetInt64(ctx, "service.retries", 3, "Retry limit"); err != nil {
    return err
}
```

After the next successful refresh:

```go
retries, err := store.GetInt64(ctx, "service.retries")
if err != nil {
    return err
}
```

Typed setters always store an object such as `{"value":3}`. Typed getters
require a non-null `value` field of the requested type: wrong types, missing
fields, fractional integers, and integer overflow return errors. Valid zero
values (`0`, `false`, and `""`) are preserved.

The generic wrapper is `Value[ValueType]` with a `Value ValueType` field tagged
`json:"value"`. Its concrete aliases are `StringValue`, `Int64Value`,
`BoolValue`, and `Float64Value`. For example,
`store.Set(ctx, "service.label", dbkv.StringValue{Value: "LZApp"}, "")` is equivalent
to `store.SetString(ctx, "service.label", "LZApp", "")`.

## Tests

```sh
go test -race -count=1 ./components/dbkv
```

Tests create independent in-memory SQLite databases and `DBKV` instances,
without modifying the runtime or connecting to the application's configured
MySQL database. Both SQLite and MySQL store JSON
as text, preserving the JSON representation without numeric coercion.
The SQLite test driver requires CGO to be enabled and a working C compiler.
Tests cover instance isolation, constructor failures, typed values,
database-only writes, visibility, transaction rollback, concurrent reads and
writes, marker-based snapshot replacement, last-good-snapshot retention, and
worker lifecycle including concurrent shutdown. Polling tests use a short
internal interval; production uses 60 seconds.