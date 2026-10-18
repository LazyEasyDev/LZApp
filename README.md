# LZApp

## API metadata

The HTTP configuration in [config/profile.go](config/profile.go) defines
`APITitle` (`api_title`, default `LZApp API`) and `APIVersion`
(`api_version`, default `1.0.0`). These values populate the OpenAPI API title
and version. Both must be non-empty; changing them does not change route paths.

## AM referral statistics

`GET /am/refstatistics/list` returns paginated registration counts grouped by
`ref_userid` and case-sensitive `ref_channel`. Results always exclude null or
zero referring user IDs. Registration guarantees a valid channel for each
attributed referring user ID.

- Access requires `admin`, `viewall`, or `am`.
- AM-only accounts are restricted to their own user ID. Requesting another
  referring user ID returns HTTP 403.
- `admin` and `viewall` accounts may filter by any referring user ID or omit
  it to include all attributed referrers, including when they also have `am`.
- An empty `ref_channel` includes all channels; otherwise matching is exact
  and case-sensitive.
- `page` defaults to 1; `page_size` defaults to 20 and is limited to 100.
- Each item contains `ref_userid`, `ref_channel`, and `count`. `total` counts
  matching groups, not individual registrations.

## Deployment: direct client IP and rate limiting

LZApp is designed to receive HTTPS connections directly, without an
HTTP reverse proxy such as Nginx or an AWS Application Load Balancer (ALB)
in front of the API.

For load-balanced deployments, use a connection-level load balancer such as
an AWS Network Load Balancer (NLB), configured to preserve the client source IP.

```text
Client -- HTTPS --> NLB (TCP listener, client IP preserved) --> LZApp HTTPS
```

With a TCP listener, TLS passes through the NLB and terminates in LZApp.
Configure the listener and target group to forward to the application's
configured HTTPS port.

### Client IP handling

[ClientIPMiddleware](app/base/base_api/middleware/client_ip.go) reads the
connection's remote address, extracts the IP, and normalizes IPv4 and IPv6
addresses. This IP is used for:

- Per-IP rate limiting on routes with a rate-limit policy.
- The registration IP stored for a user.
- The remote IP returned by the health endpoint.

LZApp does not trust `X-Forwarded-For`, `X-Real-IP`, or `Forwarded` headers.
There is no `TrustedProxyCIDRs` configuration. Sending a forwarding header
does not change the client IP seen by the application.

If the remote address cannot be parsed, the middleware logs an error and
returns HTTP 500 without running the endpoint handler. Normal TCP connections
provide a valid remote address.

### NLB configuration requirements

- Verify that client IP preservation is enabled and supported for the chosen
  target group and network topology. Do not assume every NLB configuration
  preserves the client IP by default.
- Use a TCP listener for the HTTPS passthrough layout shown above.
- Do not enable PROXY protocol v2 for the current application listener:
  LZApp does not parse PROXY protocol headers.
- Configure security groups and network access for the intended deployment.
- Verify the reported client IP through the health endpoint from an external
  client before relying on per-IP rate limits.

If client IP preservation is disabled, LZApp may see an NLB node's IP instead.
The address will still parse successfully, but different visitors may share
the same rate-limit bucket and registration IP.

### Rate-limit scope

[SpeedLimitMiddleware](app/base/base_api/middleware/speed_limit.go) counts
requests by client IP and route (HTTP method plus route path). Exceeding a
route's configured limit returns HTTP 429 with a `Retry-After` header.

Counters are held in process-local memory. With multiple application instances,
each instance enforces its own limits; they are not a shared, cluster-wide quota.

### If the deployment changes

Putting an ALB, Nginx, CloudFront, or another HTTP reverse proxy in front of
LZApp requires revisiting client IP handling. Otherwise, rate limiting and
registration records will use the proxy's source IP.

Do not simply trust arbitrary forwarding headers. Any future proxy support
must explicitly identify trusted proxies and correctly resolve their forwarding
chain, with network access restricted accordingly.
