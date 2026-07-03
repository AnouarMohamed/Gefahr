# Operations runbook

## Probes

- `/livez` reports process lifecycle.
- `/readyz` succeeds only when every configured pool has a healthy backend.
- `/metrics` uses Prometheus text exposition with request, duration, cache,
  request-policy denial, rate-limit decision, retry, backend health, and
  active-request series.

The container image checks `/readyz` with `goproxy -healthcheck URL`. This mode
uses direct HTTP connections, rejects redirects and non-200 responses, and
exits within five seconds.

Bind the admin listener to loopback or a private management network. Set
`admin.auth_token_env` in production so `/livez`, `/readyz`, and `/metrics`
require `Authorization: Bearer <token>`. If the token is stored in
`GOPROXY_ADMIN_TOKEN`, the built-in `goproxy -healthcheck` command and
`make load-check` metrics scrape send it automatically. Use `admin.tokens[]`
for lower-privilege monitoring credentials; `read` grants health and metrics,
while `health` or `metrics` grants only that endpoint class. Every admin
request is logged as `admin request completed` with method, path, status, auth
result, principal, remote address, and duration; authorization headers are
never logged.
Startup validation rejects unauthenticated admin listeners that are not bound to
loopback.

## Reload

Validate edits in a separate process where practical, then atomically replace
the configuration file and send `SIGHUP`. A rejected reload is logged and the
previous snapshot remains active. Certificate files are parsed before any new
certificate is published.

## Shutdown

Send `SIGTERM` or `SIGINT`. New connections stop, health workers are canceled,
and in-flight requests drain up to `timeouts.shutdown`. Container orchestrators
should provide a termination grace period longer than that value.

## Diagnosis

Each JSON request log includes request ID, route, backend, status, latency,
attempt count, and cache result. Use the returned `X-Request-ID` to correlate a
client failure. A `503` means no healthy backend was eligible or the configured
request-admission limit was reached; inspect the JSON error `code` to distinguish
those cases. `429` indicates a configured route rate limit. `502` indicates a
transport failure; `504` indicates the upstream deadline elapsed.

Monitor backend health, retry rate, 5xx rate, cache outcomes, request latency,
`goproxy_policy_denials_total`,
`goproxy_rate_limit_decisions_total{decision="limited"}`, process memory, and
goroutine count. Repeated backend flapping usually means probe thresholds are
too aggressive or the health endpoint is not representative.

In Kubernetes, scrape every pod rather than only a Service aggregate. Backend
health, cache outcomes, rate-limit decisions, and process metrics are local to
each pod.

See [Kubernetes deployment](deployment-kubernetes.md),
[systemd deployment](deployment-systemd.md), and
[operations runbooks](runbooks.md) for production rollout and incident guidance.
