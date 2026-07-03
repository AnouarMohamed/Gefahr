---
title: Security model
section: Reference
order: 250
summary: Security boundaries, controls, and known limitations for production review.
---

# Security model

Gefahr treats HTTP parsing and framing as a security boundary and delegates
that boundary to Go's standard library. Gefahr owns the proxy-specific policy
around that foundation.

## Built-in controls

- Ambiguous path rejection before routing.
- Maximum header and body sizes.
- Public read, write, idle, and upstream deadlines.
- Bounded backend retries.
- Bounded cache memory.
- Static per-route request policy.
- Per-route rate limits with bounded client-identity state.
- Bounded graceful shutdown.
- Full-scope and scoped admin bearer token support.
- Admin audit logs.
- Upstream TLS CA, SNI, and mTLS support.

## Trusted proxy boundary

Forwarding headers are ignored unless the direct peer is in
`client_ip.trusted_proxies`. Only configure CIDRs for infrastructure that
sanitizes and sets those headers, and configure `client_ip.headers` with the
exact trusted header order.

## Admin boundary

Admin endpoints are for private operations networks. `admin.auth_token_env`
grants full admin access. `admin.tokens[]` can grant named `health`, `metrics`,
`read`, or `admin` credentials so monitoring does not need the operator token.
Startup validation rejects unauthenticated non-loopback admin listeners. Even
with bearer auth, the admin listener should not be directly internet-facing.

## Known limitations

Gefahr does not include:

- Role-based admin authorization.
- External identity provider integration.
- Full WAF rule engine.
- Adaptive bot classification.
- ACME client.
- Dynamic service discovery.
- Runtime configuration mutation API.
- Distributed cache coherence.
- Global rate-limit counters.
- Shared backend health coordination.

Place external controls around Gefahr where those features are required.
