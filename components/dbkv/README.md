# Database Key-Value Store

`dbkv` and `users` expose context-aware package functions backed by the shared
database in the components runtime. No repository instance is required.
The `dbkv` table has four columns: an auto-incrementing `id`, a unique `key`,
a JSON-encoded text `value`, and a text `description`.

`Entry.Value` is a plain Go `string` containing JSON text, stored in a `TEXT`
column. `Set` JSON-encodes values before writing. Cache loading and `GetFromDB`
reject invalid stored JSON. Direct GORM or SQL writes bypass this validation. Database-specific text
size limits apply (MySQL `TEXT` holds at most 65,535 bytes).

## Initialization

App startup calls `dbkv.Init(ctx)` after the components database has been
initialized. For standalone use, initialize `components.InitDB` first, then:

```go
if err := dbkv.Init(ctx); err != nil {
    return err
}
defer dbkv.Close()
```

`Init` creates the table if missing, initializes the reserved update marker if
absent, and loads the whole table before returning. It then starts a SafeGo
worker that checks the database marker every 60 seconds. A changed marker
causes a complete cache replacement, including removal of deleted keys.

Initial-load failures return an error. Later refresh failures are logged and
leave the last good snapshot intact; the next poll retries. Normal reads use
only that local snapshot, so they can continue during a database outage.

Canceling the initialization context stops the worker. `Close()` stops and
waits for it and clears the cache, without closing the shared database.
Repeated `Init` calls stop the previous worker and reload the cache before
starting a replacement. Call `Init` once at startup with the application
context, not once per request.

Only missing tables are created; existing schemas are not upgraded. This
pre-release project does not support schema migrations. When models change,
manually recreate the affected development tables before restarting the app;
recreating tables deletes their data.

`components.GetComponents()` returns a pointer to the shared runtime. Initialize
or replace its database only when no operations are running, after stopping
the cache worker; runtime mutation is not synchronized. Normal concurrent
operations share GORM's connection pool. `dbkv` is initialized from the app
layer because importing it into `components` itself would create an import cycle.

## Update Marker

`LastUpdatedKey` (`__last_updated__`) is reserved. Its value is a bare JSON
integer containing Unix nanoseconds, initially `0` before the first mutation.
Every successful `Set` and `Delete` changes the row and marker in one
transaction. The marker row serializes writers, and the new value is at least
one greater than the previous value even when writes share a clock tick or
the clock moves backward. Failed writes and missing-key deletes do not advance it.

The marker cannot be written or deleted through public setters or `Delete`.
Read it with `Get` or `GetFromDB`, then decode `entry.Value` into an `int64`;
it is not a typed-helper `{"value":...}` wrapper.

Committed local writes immediately update the local cache. Changes from other
processes appear on the next successful poll. Local writes do not advance the
full-snapshot version, so they cannot hide earlier remote changes. External
writers must update the marker in the same transaction using the same locking
and monotonicity rules; direct SQL changes without a marker update are not
detected reliably. Prefer this package's setters and `Delete`.

## Set, Get, Delete

- `Set(ctx, name, value, description)` JSON-encodes a Go value and atomically
  inserts or updates the key. Structs, maps, slices, and JSON primitives are
  supported, including `nil` as JSON `null`. Invalid JSON, unsupported Go
  values, and non-finite floats return an error without writing a row.
- `Get(ctx, name)` returns a copy of the locally cached `Entry`, with no database
    query or fallback. Before initialization or after `Close`, it returns
    `ErrNotInitialized`. `entry.Value` contains JSON text; decode it using
    `json.Unmarshal([]byte(entry.Value), &destination)`.
- `GetFromDB(ctx, name)` is the former direct-read `Get`. It reads the current
    database value without reading or changing the cache.
- `Delete(ctx, name)` deletes that key transactionally and removes it from the
    local cache after commit. `Get`, `GetFromDB`, and `Delete` return
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
if err := dbkv.Set(ctx, "service.options", map[string]any{
    "enabled": true,
    "retries": 3,
}, "Service settings"); err != nil {
    return err
}

entry, err := dbkv.Get(ctx, "service.options")
if err != nil {
    return err
}
var options map[string]any
if err := json.Unmarshal([]byte(entry.Value), &options); err != nil {
    return err
}

if err := dbkv.Delete(ctx, "service.options"); err != nil {
    return err
}
```

For JSON that is already encoded, pass `json.RawMessage`:

```go
if err := dbkv.Set(ctx, "options", json.RawMessage(`{"enabled":true}`), ""); err != nil {
    return err
}
```

Passing an ordinary string JSON-encodes it as a string, not as an object.

## Typed Helpers

The paired helpers are `SetString`/`GetString`,
`SetInt64`/`GetInt64`, `SetBool`/`GetBool`, and `SetFloat64`/`GetFloat64`.
All setters require the same description argument. All typed getters read
from the local cache through `Get`.

```go
if err := dbkv.SetInt64(ctx, "service.retries", 3, "Retry limit"); err != nil {
    return err
}
retries, err := dbkv.GetInt64(ctx, "service.retries")
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
`dbkv.Set(ctx, "service.label", dbkv.StringValue{Value: "LZApp"}, "")` is equivalent
to `dbkv.SetString(ctx, "service.label", "LZApp", "")`.

## Tests

```sh
go test -race -count=1 ./components/dbkv
```

Tests temporarily install and restore isolated in-memory SQLite databases in
the runtime. They must not run in parallel within a package, and never connect to
the application's configured MySQL database. Both SQLite and MySQL store JSON
as text, preserving the JSON representation without numeric coercion.
The SQLite test driver requires CGO to be enabled and a working C compiler.
MySQL column types and upsert SQL are also checked in GORM dry-run mode.
Tests cover transaction rollback, marker-based reloads, last-good-snapshot
retention, concurrent reads and writes, and worker polling and cancellation.
The polling test uses a short internal interval; production uses 60 seconds.