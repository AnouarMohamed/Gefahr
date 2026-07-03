---
title: Troubleshooting
section: Operate
order: 170
summary: Diagnose 404, 405, 413, 429, 502, 503, 504, readiness failures, cache misses, and admin access problems.
---

# Troubleshooting

Start every investigation with the JSON error `code`, route, request ID, and
backend name from logs.

## Common status codes

| Status | Common code | First checks |
|---:|---|---|
| `400` | `ambiguous_request_path` | Client path encoding, ingress rewrite |
| `404` | `route_not_found` | Host header, path prefix, route order |
| `405` | `method_not_allowed` | Route `policy.allowed_methods` |
| `413` | `request_too_large` | `limits.max_body_bytes` |
| `414` | `query_too_large` | Route `policy.max_query_bytes` |
| `429` | `rate_limited` | Client identity, route budget, ingress forwarding |
| `502` | `bad_gateway` | Upstream transport failure |
| `503` | `no_healthy_upstream` | Backend health, network, health path |
| `503` | `proxy_overloaded` | Concurrency limit, upstream latency |
| `504` | `upstream_timeout` | `timeouts.response_header`, upstream latency |

## Route not found

Check:

1. Request `Host` header.
2. Whether the load balancer rewrites host.
3. Route `path_prefix`.
4. Whether the request path contains an unexpected prefix added by ingress.

## Unexpected rate limiting

Check:

1. `goproxy_rate_limit_decisions_total{decision="limited"}` by route.
2. In Kubernetes, whether one pod is limiting or every pod is limiting.
3. `client_ip.trusted_proxies`.
4. `client_ip.headers`.
5. Whether the load balancer sanitizes the configured forwarding header.
6. Whether many clients are collapsing to the same direct peer identity.

Do not raise limits until identity is verified.

## No healthy upstream

Check:

1. `goproxy_backend_healthy` for the pool.
2. Backend health endpoint status and latency.
3. NetworkPolicy, security group, or firewall changes.
4. Upstream TLS CA and SNI settings.
5. Recent backend rollout.

## Proxy overloaded

`proxy_overloaded` means the configured concurrent request limit was reached.
Check upstream latency and active backend requests before raising the limit.

## Cache misses

Expected cache bypass reasons include authenticated requests, cookies,
`Set-Cookie`, `Vary`, `private`, `no-store`, `no-cache`, non-200 responses,
and unsafe methods.

## Admin unauthorized

Check the admin audit log source address. If the source is unexpected, rotate
the admin token and tighten NetworkPolicy, firewall rules, or security groups.
