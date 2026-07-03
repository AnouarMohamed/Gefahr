---
title: Configuration model
section: Getting started
order: 50
summary: Understand how listeners, routes, pools, limits, policy, cache, and reload fit together.
---

# Configuration model

Gefahr is configured with one strict YAML file. Unknown fields and invalid
values fail startup or reload. This makes configuration reviewable and keeps
runtime behavior predictable.

## Main objects

| Object | Role |
|---|---|
| `listeners` | Public addresses that receive proxied traffic |
| `admin` | Private operational listener for health, readiness, and metrics |
| `routes` | Host and path-prefix matches that point to backend pools |
| `pools` | Groups of interchangeable upstream servers |
| `timeouts` | Public, upstream, idle, and shutdown deadlines |
| `limits` | Header, body, connection, and concurrency bounds |
| `client_ip` | Trusted proxy CIDRs and forwarding-header rules |
| `cache` | Process-wide response-cache bounds |
| `logging` | Structured log level |

## Routes point to pools

A route does not contain backend URLs. It names a pool:

```yaml
routes:
  - name: api
    host: api.example.test
    path_prefix: /api
    pool: api
    strategy: least_connections
```

The pool contains upstreams and health policy:

```yaml
pools:
  api:
    backends:
      - name: api-1
        url: https://api-1.internal:8443
    health:
      path: /health
      interval: 5s
      timeout: 1s
      unhealthy_threshold: 2
      healthy_threshold: 1
```

This separation lets you review matching behavior separately from upstream
transport behavior.

## Reload behavior

`SIGHUP` builds a complete replacement runtime snapshot before publishing it.
If validation fails, the old snapshot keeps serving.

Reloadable:

- Routes and pools.
- Upstream TLS files.
- Cache policy.
- Body limits and concurrency limits.
- Logging level.

Restart-only:

- Public listener addresses.
- Listener TLS mode.
- Admin address and admin token environment variable.
- Public server timeouts.
- Maximum header size.
- Per-listener connection limit.

## Validation style

Gefahr reports all configuration errors it can find at once. Common failures
include:

- Duplicate route names.
- Route pool references that do not exist.
- Path prefixes that do not start with `/`.
- Ambiguous paths with dot segments or encoded separators.
- Backend URLs with embedded credentials or fragments.
- Incomplete upstream mTLS certificate/key pairs.
- Invalid or incomplete trusted proxy policy.
- Non-positive limits or timeouts.

Treat config validation as part of CI, not just startup.
