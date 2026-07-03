---
title: Put Gefahr behind a load balancer
section: Tasks
order: 130
summary: Configure trusted proxy CIDRs, health checks, idle timeouts, and forwarding-header ownership.
---

# Put Gefahr behind a load balancer

Most production deployments should place Gefahr behind a cloud load balancer,
host firewall, ingress, or CDN. That layer owns internet exposure. Gefahr owns
application routing and upstream forwarding.

## Forwarding headers

Gefahr ignores client-provided forwarding headers unless the direct peer
matches `client_ip.trusted_proxies` and the header is listed in
`client_ip.headers`.

Configure only the source CIDRs of load balancers or ingress hops that sanitize
and set the explicit header order:

```yaml
client_ip:
  trusted_proxies:
    - 10.0.0.0/8
  headers:
    - X-Forwarded-For
    - X-Real-IP
```

Do not put `0.0.0.0/0` in `trusted_proxies`. That would allow clients to spoof
their own identity.

## Health checks

Choose the health check based on what you want to prove:

| Check target | Proves |
|---|---|
| Public route | Load balancer, proxy, route, and upstream work together |
| `/readyz` on admin | Proxy process and pool readiness |
| `/livez` on admin | Process is alive |

When checking admin endpoints, keep them private and include the bearer token
if `admin.auth_token_env` is enabled.

## Idle timeout alignment

Align load balancer idle timeout with:

- `timeouts.idle`
- Expected upstream response behavior.
- Client retry behavior.

If the load balancer times out first, clients may see errors before Gefahr can
produce a controlled gateway response.

## TLS placement

Common patterns:

- Load balancer terminates public TLS, Gefahr receives HTTP.
- Load balancer passes TCP/TLS through, Gefahr terminates TLS.
- Load balancer terminates public TLS, Gefahr uses HTTPS to upstreams.

Document the chosen pattern. It affects certificates, forwarded proto headers,
upstream trust, and incident ownership.
