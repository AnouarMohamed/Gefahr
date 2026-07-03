---
title: Configuration reference
section: Reference
order: 200
summary: Complete reference for top-level config fields, route policy, rate limiting, pools, TLS, cache, and client IP handling.
---

# Configuration reference

Gefahr config is strict YAML. Unknown fields fail validation.

## Top-level fields

| Field | Required | Purpose |
|---|---:|---|
| `listeners` | yes | Public HTTP or HTTPS listeners |
| `admin` | yes | Private operations listener |
| `routes` | yes | Host/path matches to pools |
| `pools` | yes | Backend groups |
| `timeouts` | yes | Network and shutdown deadlines |
| `limits` | yes | Header, body, connection, and concurrency bounds |
| `client_ip` | no | Trusted proxy identity extraction |
| `cache` | yes | Process-wide cache bounds |
| `logging` | yes | Log level |

## Admin

```yaml
admin:
  address: "127.0.0.1:9090"
  auth_token_env: GOPROXY_ADMIN_TOKEN
  tokens:
    - name: monitor
      env: GOPROXY_MONITOR_TOKEN
      scopes:
        - read
```

`auth_token_env` is a backward-compatible full-scope admin token. Use
`admin.tokens[]` for named scoped credentials. Startup validation rejects an
admin listener that is not bound to loopback unless `admin.auth_token_env` or
`admin.tokens[]` is configured.

Supported scopes:

| Scope | Grants |
|---|---|
| `health` | `GET /livez` and `GET /readyz` |
| `metrics` | `GET /metrics` |
| `read` | Health and metrics endpoints |
| `admin` | All admin endpoints |

Changing `admin.address`, `admin.auth_token_env`, or `admin.tokens[]` requires
a restart.

## Client IP

By default, Gefahr uses the direct socket peer as the client identity and
ignores incoming forwarding headers. Configure `client_ip.trusted_proxies` only
for ingress or load balancer CIDRs that sanitize forwarded client identity, and
always configure the exact header order with `client_ip.headers`.

```yaml
client_ip:
  trusted_proxies:
    - 10.0.0.0/8
  headers:
    - X-Forwarded-For
    - X-Real-IP
```

Supported headers are `X-Forwarded-For` and `X-Real-IP`. Startup validation
rejects `client_ip.trusted_proxies` without an explicit `client_ip.headers`
list.

## Route

```yaml
routes:
  - name: api
    host: api.example.test
    path_prefix: /api
    pool: api
    strategy: round_robin
    rewrite_host: false
    cache:
      enabled: true
    policy:
      allowed_methods: [GET, HEAD, POST]
      denied_path_prefixes: [/api/internal]
      required_headers: []
      denied_headers: [X-Debug-Bypass]
      max_query_bytes: 4096
    rate_limit:
      enabled: true
      requests: 300
      window: 1m
      max_keys: 10000
```

`strategy` must be `round_robin` or `least_connections`.

## Pool

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
    retry:
      max_attempts: 2
    tls:
      ca_file: /etc/goproxy/upstream-ca.pem
      server_name: api.internal
      client_cert_file: /etc/goproxy/client.crt
      client_key_file: /etc/goproxy/client.key
```

`retry.max_attempts` must be `1` or `2`.

## Timeouts

| Field | Applies to |
|---|---|
| `read_header` | Time to read public request headers |
| `read_body` | Time to read request body |
| `write` | Time to write response |
| `idle` | Idle public connection lifetime |
| `shutdown` | Graceful shutdown drain window |
| `dial` | Upstream TCP dial |
| `response_header` | Upstream response header wait |

## Limits

| Field | Purpose |
|---|---|
| `max_header_bytes` | Maximum public request headers |
| `max_body_bytes` | Maximum request body |
| `max_concurrent_requests` | Admitted in-flight public requests |
| `max_connections` | Connections per public listener |

## Logging

```yaml
logging:
  level: info
```

Allowed values: `debug`, `info`, `warn`, `error`.
