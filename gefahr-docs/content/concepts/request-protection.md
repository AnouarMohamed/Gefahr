---
title: Request protection
section: Concepts
order: 90
summary: Use static request policy, body limits, trusted client identity, and rate limits to reduce bad traffic before it reaches upstreams.
---

# Request protection

Gefahr provides static request guardrails. These controls are useful at the
proxy layer, but they are not a full WAF or bot-detection system.

## Protection layers

| Layer | What it blocks |
|---|---|
| Path safety | Ambiguous encoded paths and dot segments |
| Body limit | Oversized request bodies |
| Route policy | Disallowed methods, path prefixes, headers, and query strings |
| Rate limit | Too many requests per trusted client identity |
| Concurrency limit | Too many admitted in-flight requests |

## Method allowlists

Restrict a route to methods the upstream actually supports:

```yaml
policy:
  allowed_methods:
    - GET
    - HEAD
    - POST
```

Disallowed methods return `405` and include an `Allow` header.

## Denied path prefixes

Block paths that should never be externally reachable:

```yaml
policy:
  denied_path_prefixes:
    - /internal
    - /debug
```

Prefix boundaries are respected. `/debug` matches `/debug` and `/debug/pprof`,
not `/debugger`.

## Required and denied headers

Require a header added by trusted ingress:

```yaml
policy:
  required_headers:
    - X-Verified-Client
```

Deny headers that should not cross the proxy boundary:

```yaml
policy:
  denied_headers:
    - X-Debug-Bypass
```

Do not use required headers as authentication by themselves. They are useful
when the ingress path is private and controlled.

## Query-string cap

```yaml
policy:
  max_query_bytes: 4096
```

This rejects unusually large raw query strings before they reach application
parsers, caches, or upstream logs.

## Rate limits

```yaml
rate_limit:
  enabled: true
  requests: 300
  window: 1m
  max_keys: 10000
```

Rate limits are per route and per trusted client identity. In Kubernetes with
multiple replicas, counters are also per pod; they are not global cluster
limits. Keep `max_keys` bounded so an attacker cannot create unbounded
client-identity state.

## Metrics

Watch:

- `goproxy_policy_denials_total{route,reason}`
- `goproxy_rate_limit_decisions_total{route,decision="limited"}`
- `goproxy_requests_total{status="413"}`
- `goproxy_requests_total{status="503"}`

Unexpected increases should be investigated before raising limits. They may
indicate an abusive client, a wrong ingress rewrite, or an API contract change.
