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

Tokens are excluded from JSON with `json:"-"` and are not changed by ordinary
user updates or repeated initialization. Treat the stored token as a secret.
This adds token storage only; it does not change HTTP authentication.

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