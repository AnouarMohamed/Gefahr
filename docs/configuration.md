# Configuration reference

The local example is [`configs/proxy.example.yaml`](../configs/proxy.example.yaml).
For a production VPS starting point, use
[`configs/proxy.vps.yaml`](../configs/proxy.vps.yaml).
Durations use Go syntax such as `250ms`, `30s`, and `2m`.
Configuration input is capped at 4 MiB and topology counts are bounded to
prevent pathological startup or reload memory use.

`SIGHUP` reloads routes, pools, upstream timeouts, body limits, cache policy,
logging, and certificate contents. Listener topology, TLS mode, the admin
address, admin authentication token variable, public server timeouts, shutdown
timeout, and maximum header size are restart-only, as is the per-listener
connection limit.

## Top-level fields

- `listeners`: one or more public addresses. Optional `tls.cert_file` and
  `tls.key_file` enable TLS 1.2+ for that listener.
- `admin.address`: private liveness, readiness, and metrics listener.
  `admin.auth_token_env` names an environment variable containing a full-scope
  bearer token required for all admin endpoints. Authentication is required
  whenever `admin.address` is not loopback, and it should still be enabled for
  production loopback deployments.
  `admin.tokens[]` can add named scoped bearer tokens loaded from environment
  variables. Supported scopes are `health`, `metrics`, `read` (health and
  metrics), and `admin`. Changing any admin field requires a restart.
- `routes`: ordered route definitions with unique names.
- `pools`: named backend groups referenced by routes.
- `timeouts`: public read/write, upstream dial/response, idle, and shutdown
  bounds.
- `limits`: maximum request header/body bytes, concurrent admitted requests,
  and connections per public listener.
- `client_ip`: optional trusted proxy CIDRs and header order used to identify
  the original client for rate limiting and forwarding headers.
- `cache`: process-wide maximum entries, bytes, and fallback TTL.
- `logging.level`: `debug`, `info`, `warn`, or `error`.

## Routes

`host` is an exact case-insensitive host without semantic wildcard expansion.
An empty host is a catch-all. `path_prefix` must begin with `/`; `/api` matches
`/api` and `/api/...`, not `/apix`. The longest matching prefix wins.

`strategy` is `round_robin` or `least_connections`. `rewrite_host: true` sends
the selected backend host; the default preserves the original request host.
`cache.enabled` opts the route into conservative shared response caching.
`policy` applies static request guardrails before rate limiting, cache lookup,
or backend selection. `allowed_methods` is an optional uppercase HTTP method
allowlist. `denied_path_prefixes` blocks exact path-prefix boundaries such as
`/admin` and `/admin/...`. `required_headers` requires a non-empty request
header, `denied_headers` rejects requests containing listed headers, and
`max_query_bytes` rejects oversized raw query strings when non-zero.
`rate_limit.enabled` applies a per-route, per-client fixed-window limit.
`requests` and `window` set the budget, while `max_keys` bounds the number of
tracked client identities and defaults to 10000 when omitted.

## Client IP

By default, Gefahr treats the direct socket peer as the client identity and
ignores inbound forwarding headers. Configure `client_ip.trusted_proxies` only
for ingress or load-balancer CIDRs that sanitize and set forwarding headers.
When the direct peer matches one of those CIDRs, Gefahr evaluates
`client_ip.headers` in order. Supported headers are `X-Forwarded-For` and
`X-Real-IP`. `client_ip.headers` is required whenever
`client_ip.trusted_proxies` is set so the trusted identity contract is explicit.

For `X-Forwarded-For`, Gefahr walks the chain from right to left, skips trusted
proxy hops, and uses the first untrusted address. This avoids trusting a
leftmost spoofed value appended by a client before the trusted ingress. Valid
`IP:port` and `[IPv6]:port` entries are accepted for compatibility with managed
load balancers that include client ports.

## Pools

Every backend requires a unique `name` and absolute `http` or `https` URL.
Backend URLs cannot contain embedded credentials or fragments.
Health settings define the probe path, interval, timeout, and consecutive
failure/success thresholds. `retry.max_attempts` is `1` or `2`; retries apply
only to safe replayable methods and only before response commitment.

`tls.ca_file` extends the host trust store for HTTPS upstreams. `tls.server_name`
overrides SNI and certificate hostname verification. `tls.client_cert_file` and
`tls.client_key_file` enable upstream mTLS and must be configured together.
`tls.insecure_skip_verify` disables upstream certificate verification and should
only be used for isolated diagnostics.

Unknown fields, duplicate routes/listeners, nonexistent pool references,
invalid URLs, incomplete upstream TLS pairs, invalid route policies, invalid
admin token scopes, unauthenticated non-loopback admin listeners, invalid or
incomplete trusted proxy policy, non-positive limits, and invalid durations are
rejected together with field-oriented errors. Route, pool, backend, and admin
token identifiers use at most 128 ASCII letters, digits, dots, underscores, and
hyphens.
