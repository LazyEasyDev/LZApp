# Application Docs Routes

`app/httpapi/router.go` registers these HTML/browser routes through Huma's
low-level adapter, outside the API operation list:

- `GET /docs`: serves the existing Huma-generated Scalar page with the embedded
  `docs_auth.js` bootstrap.
- `POST /docs_token`: validates an existing API token cookie and returns
  the original full token plus its docs visibility settings. It never creates
  or refreshes tokens.

The generic HTTP component disables its automatic docs route and continues to
serve `/openapi.json`. `NewDocsHandler` uses an isolated Huma renderer to retain
the pinned Scalar URL, integrity attribute, OpenAPI URL, and CSP. That private
renderer is delegated to only by the public `/docs` handler; it is not mounted
as another public router. There is no `SetDocsTokenHandler` callback.

## Cookie Contract

The application token handler reads exactly one cookie named by
`config.GetConfig().HTTP.APITokenCookieName` (default: `api_token`). Login and
logout code should use this same setting when creating or deleting the cookie.
Its value must be the raw full token recognized by
`components.GetComponents().Security.Verify`, without a `Bearer ` prefix.
Missing, duplicate, empty, or invalid cookies return `401`. An unavailable
runtime signer or missing cookie-name configuration returns `503`.
A session ID is not an API token and will be
rejected; adding session-store validation is a separate application concern.

When an authenticated application flow sets this cookie, use HTTPS with
`Secure`, `HttpOnly`, `SameSite=Strict` (or an appropriate deliberate alternative),
and `Path=/` so the browser sends it to `/docs_token`. This feature does not
create a login endpoint or set an authentication cookie for anonymous visitors.

The endpoint requires `X-LZApp-Docs: 1` and same-origin request evidence, and
must not be exposed through permissive credentialed CORS. Cookies are carried
by the browser; JavaScript does not read them. Responses use `Cache-Control:
no-store`, and failures do not echo credentials. Behind a proxy, preserve the
trusted request scheme and host; the origin check does not trust forwarded
headers supplied by arbitrary clients.

## Browser Behavior

The bootstrap waits up to three seconds for `/docs_token`, places a successful
token response in Scalar's `bearerAuth` configuration, and fetches `/openapi.json`
with a separate five-second timeout. It filters a local copy of that schema,
passes it to Scalar as `content`, and removes the original document URL so Scalar
does not reload an unfiltered list. The original Scalar loader and its integrity
attribute are preserved.

Tokens are not placed in shared HTML or persisted in browser storage
(`persistAuth: false`). On a successful JSON response, visibility settings are
applied independently of the token: an empty token does not discard an explicit
allowlist or `showAll: true`. Only a nonempty, well-formed token is prefilled in
Scalar. Failed policy requests or missing visibility settings mean no operations
are displayed; schema-fetch failure also displays no operations. The docs shell
and manual Bearer entry remain available. Entering a token manually does not
change the visibility list; reload the page to request a new policy. Requests to
other protected API routes continue to use the `Authorization` header, not cookie
authentication.

## Operation Visibility

The successful `/docs_token` response has this shape:

```json
{
  "token": "<original full token>",
  "allowedOperations": ["GET /health", "GET /setAuth", "GET /auth_check"],
  "showAll": false
}
```

Edit `AllowedOperations` and `ShowAll` in `docs_token_view.go` to choose the
response. A future server-side token/user lookup can choose different lists or
set `ShowAll: true` for full-docs permission. Returning `ShowAll: true` for an
empty or invalid token makes the entire docs list visible for that case; it does
not indicate that the requester is authenticated or an administrator.

Entries must exactly match an uppercase HTTP method, a space, and the OpenAPI
path, such as `GET /users/{id}`. There are no wildcard rules. GET and POST on the
same path are filtered separately. Shared path parameters are preserved. Empty
paths and restricted-view webhooks are removed. `showAll`
must be the boolean `true` to skip filtering; an empty list does not mean all.

In restricted views, the bootstrap also removes unused component definitions
from the local schema copy. It follows references from the remaining operations
through schemas, responses, request bodies, parameters, headers, and other
reusable components. Nested, shared, and circularly referenced models are kept,
as are discriminator mapping targets and referenced security schemes. Models
used only by hidden routes no longer appear in Scalar's Models section.

If a reachable reference is external, anchor-based, or cannot be resolved locally,
component pruning is skipped to avoid breaking schema dependencies. Operation
filtering still applies. `showAll: true` leaves the complete document unchanged.

Scalar labels operations using their summary rather than the exact allowlist
entry. At narrow viewport widths the left sidebar is behind `Open Menu`.

This is UI-only filtering, not a confidentiality or authorization boundary.
The complete OpenAPI document is still downloaded and remains available through
the schema exports. Removing unused components from the displayed copy is not
redaction of the source document; retained models and examples may also describe
hidden functionality. Users can modify the JavaScript or call hidden endpoints
directly. Enforce permissions in the API middleware and handlers independently
of these docs settings.

Validation currently proves only the HMAC signature. It does not check a user
record, expiry, revocation, or permissions. Use a stable, strong configured key
to retain valid tokens across restarts; empty configured keys are per-instance.
Never put the signing key or full tokens into logs or URLs.