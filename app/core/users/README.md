# User Access

`User.Name` is an optional, non-unique display name (`*string`), with a maximum
length of 100 characters. A nil pointer represents SQL `NULL` and JSON `null`;
a pointer to an empty string represents `""`. Setting `Name` to nil in `Update`
clears the stored name.

There is no separate username field. Database initialization seeds a user named
`admin` and uses the unique email `admin@lzapp.local` to find it on subsequent
runs. Multiple users can have the same name or no name.

`User.Access` is a Go string containing a JSON array of access identifiers,
for example `["viewall","am"]`. It is stored as `TEXT`, not a native JSON
column, and serializes as a string in the user JSON response.

The built-in constants in `access.go` are:

| Constant | Value |
| --- | --- |
| `ACCESS_ADMIN` | `admin` |
| `ACCESS_VIEWALL` | `viewall` |
| `ACCESS_AM` | `am` |

The package's `accessList` slice contains these constants. Go does not support
constant slices, so the slice is kept private. Both catalog getters use this
same list; `GetAccessList()` returns a copy so callers cannot modify it.

## API Tokens

`User.ApiToken` is a string stored in the `api_token` column with a maximum
length of 64 characters, a unique index, and a `NOT NULL` constraint.
Tokens are supplied by callers; there is no automatic token-generation hook.
Supplied tokens are preserved, and duplicates are rejected by the database.
The `NOT NULL` constraint does not reject an empty string.

Tokens are included in JSON as `api_token` and are not changed by ordinary user
updates or repeated initialization. This adds token storage only; it does not
change HTTP authentication.

## User Lookups

`GetByID`, `GetByEmail`, and `GetByApiToken` check the local cache, Redis, then
the database. Successful lookups cache the user for 15 seconds locally and
30 minutes in Redis.

Database `gorm.ErrRecordNotFound` results, including wrapped errors, are cached
as a nil `*User` locally for 15 seconds and JSON `null` in Redis for one minute.
A negative cache hit returns `nil, gorm.ErrRecordNotFound`. Other database errors
are returned without being cached.

After a successful database commit, `Update` invalidates the ID, old email, new
email, and stored API-token keys in both caches. `Delete` invalidates the ID,
stored email, and stored API-token keys. Both read the persisted identifiers
under a row lock rather than using cached users or the caller's API token.
Redis keys are deleted individually so this works across cluster hash slots.

Database failures leave cache entries untouched. Redis cleanup failures do not
stop attempts to clear the remaining keys. `Update` returns an error indicating
that the database update committed but cache cleanup failed. `Delete` logs cache
cleanup failures and still returns success for a committed database deletion.

`Create` does not invalidate negative entries, so a newly created user can remain
missing until those entries expire. Redis hits populate the local cache for
another 15 seconds. Invalidation does not synchronize in-flight reads or clear
other processes' local caches; those local entries expire normally.

Local-cache regression tests run with `go test ./app/core/users`. To also test
Redis, database-result handling, and List filters using the debug configuration:

```sh
LZAPP_USER_CACHE_INTEGRATION=1 go test -count=1 ./app/core/users
```

The cache integration tests use isolated Redis keys and simulated GORM reads
and writes. List filter tests execute real SQL against a connection-local
temporary table. Neither modifies application user rows. `LZAPP_TEST_DB_USER`,
`LZAPP_TEST_DB_PASSWORD`, and `LZAPP_TEST_DB_NAME` can override the test's MySQL
settings. List tests require an existing database and permission to create
temporary tables; they do not create a database.

## Listing Users

```go
found, err := users.List(ctx, users.ListFilter{
        Name:   "Alice",
        Email:  "@example.com",
        Access: `["am"]`,
}, 100, 0)
```

Every supplied condition is combined with `AND`:

- `ID`, `ApiToken`, and `Access` use SQL equality (`=`).
- `Name` and `Email` use substring matching (`LIKE '%value%'`). Search text is
    parameterized, and `%`, `_`, and the escape character `!` are treated literally.
- Zero `ID` and empty strings omit their conditions. Use `ListFilter{}` for an
    unfiltered list.

`Access` matches the entire stored JSON string, not membership of a permission.
For example, `["am"]` does not match `["am","admin"]`. Filter values are not
normalized; string comparisons follow the database column's collation.

Results remain ordered by ascending ID. Limit defaults to 100 when less than 1
and is capped at 1000; negative offsets become 0. No matches return an empty
result without `gorm.ErrRecordNotFound`.

## Initialization

```sh
go run . --config debug db init
```

Database initialization creates missing user and key-value tables, then seeds
the admin user. The access catalog is static and does not require a `dbkv`
entry or a separate catalog initializer.

Existing tables and user rows are left unchanged by table creation. This
pre-release project does not support schema upgrades or compatibility backfills.
When models change, manually recreate the affected development tables before
running `db init`; recreating tables deletes their data.

Ordinary users created with empty access default to `[]`. A newly seeded admin
receives all permissions from `GetAccessListJsonStr()`. Repeated initialization
preserves an existing user's access instead of granting permissions again.

## Reading the Catalog

```go
access := users.GetAccessList()
accessJSON := users.GetAccessListJsonStr()
```

`GetAccessList()` returns `[]string`. `GetAccessListJsonStr()` returns the same
catalog encoded as a JSON string array, currently `["admin","viewall","am"]`.
Neither function needs a context or database connection.

## Updating a User

```go
encoded, err := json.Marshal([]string{users.ACCESS_VIEWALL, users.ACCESS_AM})
if err != nil {
    return err
}
user.Access = string(encoded)
if err := users.Update(ctx, user); err != nil {
    return err
}
```

`Create` and `Update` normalize access before writing it:

- Empty input becomes `[]`.
- Input must be a JSON array of strings. JSON null, objects, scalar values,
    non-string entries, and malformed JSON are rejected.
- Every entry must exactly match a permission in `GetAccessListJsonStr()`.
    Empty strings, unknown names, different casing, and padded names are rejected.
- Duplicate permissions are removed while keeping their first-occurrence order.
- Output is compact JSON. For example, `["am","admin","am"]` becomes
    `["am","admin"]`.

Rejected values return an error without writing to the database or replacing
the caller's access string with a partial result.

This stores access assignments and the catalog only. It does not add HTTP
authorization checks or define what the identifiers allow.