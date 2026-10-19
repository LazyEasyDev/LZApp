# Server APIs

This directory separates LZApp's built-in APIs from your application's business
APIs. Use the built-in account and administration features as a foundation, and
add project-specific endpoints without mixing them into the base handlers.

For setup, configuration, and deployment, see the
[project README](../../README.md).

## Directory structure

```text
server/
  base/
    api_handler/       Built-in health, documentation, account, and admin APIs
    captcha/           CAPTCHA and email-code storage
    middleware/        Authentication, client IPs, cache policy, and rate limits
  api_handler/         Your application's business API handlers
```

These are source-code folders, not URL prefixes. For example, the built-in
user-list endpoint is `/admin/users/list`, not `/base/admin/users/list`.

## Built-in features

The [base](base/) directory contains the functionality shared across applications:

| Feature | What it provides | Main endpoints |
| --- | --- | --- |
| Health | A health response containing the connection's remote IP and server time | `GET /health` |
| API documentation | An interactive API reference and generated OpenAPI schema | `GET /docs`, `GET /openapi.json` |
| User accounts | Login, registration, logout, account details, password reset, CAPTCHA, and email verification codes | `/user/*`, `GET /user` |
| Referrals | Signed referral-token generation and attribution during registration | `POST /user/ref` |
| User administration | User search, account creation, and updates | `/admin/users/*` |
| DBKV administration | Listing, setting, and deleting database-backed configuration values | `/admin/dbkv/*` |
| EasyRoutine administration | Routine snapshots, retained logs, and deletion of stored routine state/history | `/admin/easyroutine/*` |
| EasyStatistics queries | Registered statistics definitions, batch summaries, and stored record queries | `/admin/easystatistics/*` |
| AM referral statistics | Paginated registration counts grouped by referrer and channel | `GET /am/refstatistics/list` |

### What is the docs feature?

Open `/docs` on the running API server to browse endpoint descriptions, inputs,
responses, and authentication options. Huma generates the API schema from the
registered operations and their Go input/output types.

The [docs handlers](base/api_handler/docs/) provide the interactive page and its
same-origin login-token integration. The schema is available at `/openapi.json`.
This is developer-facing API documentation, not a separate business service.

### Permissions

Authorization is enforced by middleware on each protected operation:

- Account and referral-token endpoints use `user` access where required.
- Admin read operations allow `admin` or `viewall`.
- User/DBKV writes and routine deletion require `admin`.
- AM referral statistics allow `admin`, `viewall`, or `am`. AM-only accounts
  are restricted to their own referrals.

Login, registration, and other explicitly public operations do not require an
existing session. Check each handler's registration for its exact access and
rate-limit policy.

## Adding your own business APIs

1. Create a feature package under `app/server/api_handler/`, such as
   `app/server/api_handler/orders/`.
2. Define its Huma input/output types and handlers. Follow the existing
   [base handlers](base/api_handler/) for validation and error-response patterns.
3. Expose a `RegisterRoutes(api huma.API)` function that registers the feature's
   operations with unique operation IDs and appropriate URL paths.
4. Explicitly attach the required authentication/authorization and rate-limit
   middleware. An OpenAPI security declaration alone does not protect a handler.
5. Import the package and call its `RegisterRoutes` function in
   [app/router.go](../router.go). Adding a folder does not register routes
   automatically.

Use [base middleware](base/middleware/) rather than duplicating token validation
or client-IP handling. The router already applies the no-store and client-IP
middleware globally; access requirements and rate limits are operation-specific.

Keep HTTP parsing, validation, and response mapping in handlers. Reusable domain
logic and data access belong in application packages outside the handler layer,
as demonstrated by [users](../users/) and [dbkv](../dbkv/).

After registering a Huma operation, check `/docs` for its generated documentation
and verify both permitted and denied requests.

## Where the server is wired

- [app/router.go](../router.go) registers built-in and business routes.
- [app/server.go](../server.go) registers routes and starts the HTTP service.
- [components/httpserver](../../components/httpserver/) owns HTTPS configuration,
  OpenAPI setup, and graceful HTTP shutdown.
- [app/app.go](../app.go) initializes application services and starts the HTTP
  service through the routine coordinator.

Keep `/health` registered for infrastructure monitoring, and keep built-in
handlers under `base/api_handler/` separate from business handlers under
`server/api_handler/`.