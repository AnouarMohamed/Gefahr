---
title: Observability
section: Operate
order: 150
summary: Use request logs, admin audit logs, health endpoints, readiness, and Prometheus metrics to operate Gefahr.
---

# Observability

Gefahr exposes operational state through JSON logs and private admin
endpoints.

## Admin endpoints

| Endpoint | Purpose |
|---|---|
| `/livez` | Process liveness |
| `/readyz` | Pool readiness |
| `/metrics` | Prometheus text metrics |

Protect admin endpoints with `admin.auth_token_env` and private networking.
Use a scoped `admin.tokens[]` credential with `read` or `metrics` for
monitoring systems.

## Request logs

Each completed public request includes:

- Request ID.
- Route.
- Backend.
- Status.
- Attempts.
- Cache result.
- Duration.

The response includes `X-Request-ID` so client failures can be correlated with
logs.

## Admin audit logs

Admin requests are logged with:

- Method.
- Path.
- Status.
- Auth result.
- Principal.
- Remote address.
- Duration.

Authorization headers are not logged.

## Metrics to dashboard

At minimum, dashboard:

- `goproxy_requests_total`
- `goproxy_request_duration_seconds_count`
- `goproxy_request_duration_seconds_sum`
- `goproxy_backend_healthy`
- `goproxy_backend_active_requests`
- `goproxy_retries_total`
- `goproxy_cache_requests_total`
- `goproxy_policy_denials_total`
- `goproxy_rate_limit_decisions_total`
- `go_goroutines`
- `go_memstats_alloc_bytes`

In Kubernetes, scrape every pod rather than only a Service aggregate. Backend
health, cache outcomes, rate-limit decisions, and process metrics are local to
each pod.

## Alert candidates

Alert on:

- `/readyz` failure.
- Elevated 5xx by route.
- `no_healthy_upstream` errors.
- Rising retry rate.
- High active backend requests.
- Unexpected policy denials.
- Unexpected rate limiting.
- Unauthorized admin requests.
- Memory or goroutine growth that does not return to baseline.

Alert thresholds depend on traffic volume. Start with conservative warnings in
staging and tighten after baseline observation.
