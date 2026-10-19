# LZApp

LZApp is a Go HTTPS backend for user accounts, referrals, and administration.
It includes a CLI, interactive API documentation, MySQL persistence, Redis
caching, background routine management, and statistics.

The web dashboard is a separate application. This repository runs the API.

## Getting started

### 1. Prepare the services and configuration

You need:

- Go 1.25.13 or newer, as specified in [go.mod](go.mod).
- MySQL and an account with permissions to create the required tables.
- Redis with an ACL username and password.
- SMTP credentials and a sender address for registration and password-reset email.
- A TLS certificate and matching private key for the API.

**Configure these before running the commands below.** The checked-in profiles
are not a ready-to-deploy production configuration.

Configuration is Go code, not a JSON file or an automatically loaded `.env` file:

| File | Purpose |
| --- | --- |
| [config/profile.go](config/profile.go) | Shared defaults and configuration types |
| [config/debug.go](config/debug.go) | Debug profile overrides |
| [config/release.go](config/release.go) | Release profile overrides |
| `local/config.go` | Optional, Git-ignored local configuration overrides |

Local overrides are compiled into the binary too. Check which profile they
modify: an override of `config.ReleaseAppConfig` does not affect
`config.DebugAppConfig`. The executable imports the `local` package to load these
overrides; [local/package.go](local/package.go) keeps builds working when the
optional files are absent. Rebuild after changing configuration.

Before using a profile, review:

- **MySQL:** host, port, database name, username, and password.
- **Redis:** addresses, cluster/standalone mode, ACL credentials, key prefix, and
  TLS settings. Standalone mode requires exactly one address. The key prefix
  must end in `:`. Enable certificate verification in production.
- **SMTP:** host, port, username, password, and sender address. Sending uses
  mandatory TLS.
- **HTTP:** HTTPS port and certificate/key PEM contents.
- **HMAC:** use your own persistent signing key, identical across instances.
  Changing it invalidates previously signed API and referral tokens. An empty
  key creates a temporary key that does not survive a restart.

Keep credentials and private keys out of version control. Do not deploy the
repository's example signing key, TLS material, or bootstrap account unchanged.

### 2. Build

Run these commands from this repository's root:

```bash
go mod download
go build -o LZApp .
```

### 3. Initialize the database

First review the bootstrap account in
[users.InitData](app/users/user.go). Choose suitable initial account details
before exposing the service.

```bash
./LZApp --config debug db init
```

This command **writes to the selected profile's database**. It creates the
database if missing, creates the users table if missing, and inserts the
bootstrap account if its email is not already present. Creating the database
requires the corresponding MySQL permission.

It does not migrate an existing users table or reset an existing bootstrap
account's password. Plan schema changes separately; do not drop production
tables to apply an update.

### 4. Start the API

```bash
./LZApp --config debug start --https-port 8443
```

Use a non-privileged port such as `8443` for local development. The shared
default is `443`, which may require additional operating-system permissions.
The override applies only to this invocation.

Open:

- Health check: `https://localhost:8443/health`
- Interactive API reference: `https://localhost:8443/docs`
- OpenAPI JSON: `https://localhost:8443/openapi.json`

Use the hostname covered by your certificate. For a local self-signed certificate,
trust it explicitly in your browser. A development-only connectivity check is:

```bash
curl --insecure https://localhost:8443/health
```

Do not use `--insecure` for production checks.

Starting the service also initializes DBKV, routine coordination, and statistics.

## Everyday commands

`--config` selects `debug` or `release`; it is not a file path. The default is
**release**, and omitting the command starts the service.

```bash
# Show help without starting services.
./LZApp --help
./LZApp start --help

# Start with the configured release settings.
./LZApp --config release start

# Display recent logs using the selected profile's log directory.
./LZApp --config debug showlog --tail 50
./LZApp --config release showlog --level err --tail 20
```

Log level filters accept `debug`, `info`, `warn`, and `err`. If `--tail` is
omitted, the configured `ShowLogTail` is used; its shared default is 10.

Stop the process with Ctrl+C or SIGTERM for graceful HTTP shutdown.

## HTTP and logging settings

These are the shared defaults in [config/profile.go](config/profile.go).
Profile and local overrides may change them.

| Setting | Default | Meaning |
| --- | --- | --- |
| `APITitle` | `LZApp API` | OpenAPI title; must not be empty |
| `APIVersion` | `1.0.0` | OpenAPI version; must not be empty |
| `HTTPSPort` | `443` | HTTPS listening port |
| `APITokenCookieName` | `api_token` | Login cookie name |
| `CookieMaxAgeSeconds` | `2592000` (30 days) | Login and dashboard referral cookie lifetime |
| `ReadHeaderTimeoutSeconds` | `10` | Time allowed to read request headers |
| `IdleTimeoutSeconds` | `60` | Idle keep-alive connection timeout |
| `ReadTimeoutSeconds` | `0` | No overall HTTP request-read timeout |
| `WriteTimeoutSeconds` | `0` | No HTTP response-write timeout |
| `ShutdownTimeoutSeconds` | `60` | Grace period for active HTTP requests when stopping |

After the shutdown grace period expires, remaining HTTP connections are forcibly
closed. This setting is not a normal request timeout or a login expiry setting.
Changing API metadata does not change endpoint paths.

EasyLog creates a `logs/` subdirectory inside the configured `BaseDirectory`,
with optional terminal output. `BaseDirectoryRelative` controls how a relative
base path is resolved:

- `"app"`: relative to the executable's directory; the release profile default.
- `"cwd"`: relative to the current working directory; the debug profile default.
- Empty: the same as `"cwd"`.
- An absolute `BaseDirectory` is used directly as the base path.

The shared `BaseDirectory` is `.`. This puts logs in `logs/` beside the executable
for `"app"`, or in the working directory's `logs/` for `"cwd"`. The configuration
keys are `base_directory` and `base_directory_relative`.
Previously the default base was `logs`, resulting in `logs/logs/`. Existing log
files are not moved; to read them, explicitly set `BaseDirectory` to `"logs"`.
For release logging, prefer a built executable over `go run`, whose executable is
placed in a temporary build directory. Use the same profile and path settings
for `start` and `showlog`.

## Authentication and referrals

Forced cache refreshes first set existing local entries to a 5-second TTL and
existing Redis entries to a 60-second TTL, then load fresh data. Missing entries
are allowed; Redis expiry errors stop the refresh. Successful loads restore the
normal configured TTLs. This limits typical stale-cache duration after a refresh
failure, but is not a strict cross-instance revocation guarantee.

Protected endpoints accept a bearer API token or the configured login cookie.
The login cookie is `Secure`, `HttpOnly`, and `SameSite=Strict`; use HTTPS.
For a separate dashboard origin, browser requests must include credentials.
For cookie-based deployment, keep the frontend and API on the same site, for
example `app.example.com` and `api.example.com`.

The configured cookie lifetime must be between 1 second and 400 days. Cookie
expiry does **not** expire the underlying API token: API tokens currently have
no built-in time-based expiry.

Referral flow:

1. An authenticated account with `user` access submits a channel to
   `POST /user/ref` to generate a signed referral token.
2. The dashboard captures the referral and uses `GET /user/config` to obtain
   the configured cookie lifetime.
3. Registration verifies the signature, referring user ID, channel, and referrer
   existence before storing `ref_userid` and `ref_channel`.

Channels contain 3-32 ASCII letters, digits, hyphens, or underscores, and must
start and end with a letter or digit. Case is preserved and significant.
Attribution comes from the signed token, not an editable channel label in a URL.
An invalid referral is ignored for attribution rather than blocking registration;
service failures are not treated as invalid referrals.

Consult `/docs` for request bodies, CAPTCHA/email-code requirements, and the
complete endpoint list. Notable route groups are:

| Route group | Purpose |
| --- | --- |
| `/user/*` and `GET /user` | Authentication, account details, email codes, and referrals |
| `/admin/users/*` | User listing, creation, and updates |
| `/admin/dbkv/*` | Database-backed configuration values |
| `/admin/easyroutine/*` | Routine snapshots, logs, and deletion |
| `/admin/easystatistics/*` | Statistics definitions, batch summaries, and record queries |
| `/am/refstatistics/list` | Registration counts by referrer and channel |

Admin read operations require `admin` or `viewall`; user/DBKV writes and routine
deletion require `admin`. Permissions are enforced by the API, not just the UI.

### AM referral statistics

`GET /am/refstatistics/list` groups registrations by `ref_userid` and
case-sensitive `ref_channel`, excluding null or zero referring user IDs.

| Parameter | Behavior |
| --- | --- |
| `ref_userid` | Optional positive referring user ID |
| `ref_channel` | Optional exact, case-sensitive channel; empty includes all channels |
| `page` | 1-based page number; default 1 |
| `page_size` | Records per page; default 20, maximum 100 |

Access requires `admin`, `viewall`, or `am`:

- AM-only accounts see their own referrals. Omitting `ref_userid` uses their
  account ID; explicitly requesting another ID returns HTTP 403.
- Accounts with `admin` or `viewall` may select any referrer or omit the ID to
  include all attributed referrers, even if they also have `am`.

Each item contains `ref_userid`, `ref_channel`, and `count`. The response's
`total` counts matching **groups**, not individual registrations.

## Deployment and client IPs

LZApp terminates HTTPS itself. For an AWS NLB deployment, use:

```text
Client -- HTTPS --> NLB (TCP listener, client IP preserved) --> LZApp HTTPS
```

Forward to the configured HTTPS port and verify that client IP preservation is
enabled and supported for the target group and network topology. A TCP listener
passes TLS through to LZApp; it does not terminate TLS at the NLB.

[ClientIPMiddleware](app/server/base/middleware/client_ip.go) uses the connection's
remote IP for rate limits, registration IPs, and the health response. It ignores
`X-Forwarded-For`, `X-Real-IP`, and `Forwarded`. There is no `TrustedProxyCIDRs`
setting or PROXY protocol support: **do not enable PROXY protocol v2** on the NLB.
An invalid remote address is logged and returns HTTP 500.

If the NLB does not preserve source IPs, visitors may share a load balancer IP
and rate-limit bucket. Verify `/health` from an external client before relying
on IP-based behavior.

An HTTP reverse proxy such as Nginx, ALB, or CloudFront changes this model:
LZApp will see the proxy's source IP. Supporting forwarded client IPs requires
an explicit trusted-proxy design and appropriate network restrictions, not
simply accepting arbitrary forwarding headers.

[Rate limits](app/server/base/middleware/speed_limit.go) apply per client IP and
route (HTTP method plus path). Exceeding a limit returns HTTP 429 with
`Retry-After`. Counters are process-local; multiple instances do not share a
cluster-wide quota.

## Project layout

```text
app/
  app.go                    Application startup and service wiring
  db/                       Database/bootstrap initialization command
  dbkv/                     Database-backed key-value store
  users/                    User model, access rules, referral aggregation
  server/
    router.go               API route registration
    server.go               HTTP service startup
    api_handler/            Application-specific business API handlers
    base/
      api_handler/          Health, docs, user, admin, and AM handlers
      captcha/              CAPTCHA and email-code storage
      middleware/           Authentication, client IPs, cache policy, rate limits
cli/                        CLI commands and flags
common/                     Shared application helpers
components/                 Infrastructure integrations and lifecycle
config/                     Configuration types, defaults, and profiles
local/                      Optional local customizations
main.go                     Process entry point and signal handling
```

Add routes in [app/server/router.go](app/server/router.go) and wire application services in
[app/app.go](app/app.go).

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| Database connection or missing-table errors | Selected profile, MySQL credentials, and whether `db init` ran against that database |
| Redis startup failure | ACL credentials, key prefix, standalone/cluster mode, addresses, and TLS settings |
| SMTP configuration or email-send failure | Sender address, credentials, server reachability, and TLS support |
| Browser rejects HTTPS | Certificate hostname, certificate chain, and local trust |
| Login cookie is not sent | HTTPS, credentials-enabled requests, and same-site frontend/API hosting |
| All visitors appear to have the same IP | NLB client IP preservation or an unexpected HTTP proxy |
| Logs are missing from the expected directory | Profile, `BaseDirectory`, `BaseDirectoryRelative`, executable location, and working directory |
| HTTP 429 | The route's per-IP limit; honor `Retry-After` |

## Development checks

```bash
go build ./...
go vet ./...
go test ./...
```

Run checks before deploying, and review configuration and bootstrap data
separately for each environment.
