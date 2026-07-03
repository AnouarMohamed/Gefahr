---
title: Compatibility
section: Reference
order: 240
summary: Supported protocol paths, expectations, and explicit non-goals.
---

# Compatibility

Gefahr delegates HTTP parsing and protocol maintenance to Go's `net/http`
stack. The project owns routing, limits, forwarding headers, retries, caching,
observability, and lifecycle behavior.

## Tested paths

| Path | Status |
|---|---|
| Client HTTP/1.1 to cleartext public listener | Tested |
| Client HTTP/2 over TLS to public listener | Tested |
| Proxy to cleartext HTTP/1.1 upstream | Tested |
| Proxy to HTTPS upstream with HTTP/2 | Tested |
| Trusted ingress forwarding headers | Tested |
| Admin health and metrics | Tested |

## Supported expectations

- Public TLS listeners require TLS 1.2 or newer.
- HTTPS upstream transports use TLS 1.2 or newer.
- HTTPS upstreams attempt HTTP/2 where supported.
- Forwarding headers are rebuilt before backend dispatch.
- Trusted client identity requires trusted proxy CIDRs and explicit
  `client_ip.headers`.

## Not claimed

- HTTP/3 and QUIC are not supported.
- WebSocket-specific behavior is not an acceptance target.
- gRPC is not a declared compatibility target.
- Global rate limits, distributed cache coherence, shared backend health, and
  direct EndpointSlice discovery are not part of the current target.
- Built-in sharding, tenant ownership, and sticky-session affinity are not part
  of the current target.
- Provider-specific behavior for every managed load balancer is not guaranteed.
- Full WAF or bot classification behavior is not included.

Validate your exact load balancer, ingress controller, TLS policy, idle
timeout, and forwarding-header behavior before production cutover.
