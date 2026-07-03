---
title: Request lifecycle
section: Concepts
order: 60
summary: Follow one request through listener admission, routing, policy, rate limiting, cache, balancing, and upstream forwarding.
---

# Request lifecycle

Every public request moves through the same ordered path.

```text
listener
  -> size and path safety checks
  -> route match
  -> request policy
  -> rate limit
  -> concurrency admission
  -> cache lookup
  -> backend selection
  -> ReverseProxy
  -> upstream
```

## Listener admission

The public server enforces:

- Maximum header bytes.
- Read-header timeout.
- Read-body timeout.
- Write timeout.
- Idle timeout.
- Per-listener connection limit.

Requests with declared bodies larger than `limits.max_body_bytes` are rejected
before routing. Streaming bodies are wrapped so clients cannot exceed the body
limit while the proxy reads them.

## Safe path check

Gefahr rejects ambiguous request paths before route matching. This includes:

- Encoded slash and backslash separators.
- Double-encoded separator sequences.
- Dot segments such as `.` and `..`.
- Literal backslashes.

The goal is to prevent the proxy and upstream from interpreting the route
boundary differently.

## Route match

Routing uses exact normalized host matching and longest path-prefix matching.
An empty route host is an explicit catch-all.

If no route matches, Gefahr returns:

```json
{"code":"route_not_found","message":"route not found"}
```

## Request policy

Route policy runs before rate limiting and concurrency admission. A denied
request does not consume backend capacity and does not spend rate-limit budget.

Policy can reject:

- Methods outside an allowlist.
- Requests under denied path prefixes.
- Requests missing required headers.
- Requests carrying denied headers.
- Raw query strings above a configured byte limit.

## Rate limiting

Per-route rate limits use the trusted client identity. By default, that is the
direct socket peer. If `client_ip.trusted_proxies` is configured, a trusted
load balancer can supply only the explicitly configured `client_ip.headers`,
such as `X-Forwarded-For` or `X-Real-IP`.

Denied requests return `429` with `Retry-After`.

## Backend forwarding

Gefahr selects a healthy backend, rebuilds forwarding headers, and delegates
streaming to Go's maintained `httputil.ReverseProxy`.

Safe replayable requests may be retried once before a response is committed.
Unsafe request methods are not retried.
