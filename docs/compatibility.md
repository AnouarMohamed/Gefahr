# Compatibility matrix

Gefahr delegates HTTP parsing and protocol maintenance to Go's `net/http`
stack, and keeps product-specific behavior in routing, limits, forwarding
headers, retries, caching, observability, and lifecycle code. The acceptance
suite tests the following compatibility paths over real sockets:

| Path | Status | Evidence |
|---|---|---|
| Client HTTP/1.1 to cleartext public listener | Tested | `TestRoutingBalancingAndCachingOverRealSockets` uses `httptest.NewServer` and regular `net/http` clients. |
| Client HTTP/2 over TLS to public listener | Tested | `TestHTTP2ClientOverTLS` starts a TLS public server with HTTP/2 enabled and verifies the client negotiated HTTP/2. |
| Proxy to cleartext HTTP/1.1 upstream | Tested | The real-socket routing, balancing, caching, reload, and retry tests use cleartext upstreams. |
| Proxy to HTTPS upstream with HTTP/2 | Tested | `TestHTTP2UpstreamOverTLS` uses a TLS upstream with HTTP/2 enabled and a configured CA file. |
| Forwarding headers behind trusted ingress | Tested | Unit coverage verifies untrusted forwarding headers are replaced, and trusted proxy CIDRs plus explicit header order are required before forwarded client identity is used. |
| Admin health and metrics | Tested | Unit and Compose smoke coverage verify `/livez`, `/readyz`, `/metrics`, bearer auth, and executable health checks. |

## Supported Expectations

- Public TLS listeners require TLS 1.2 or newer and advertise `h2` and
  `http/1.1` with ALPN.
- HTTPS upstream transports use TLS 1.2 or newer when pool TLS policy is
  configured and attempt HTTP/2 where the upstream supports it.
- Incoming hop-by-hop and forwarding headers are rebuilt before backend
  dispatch; route matching does not trust client-supplied forwarding chains
  unless the direct peer matches `client_ip.trusted_proxies` and the header is
  listed in `client_ip.headers`.
- The proxy is intended to sit behind a cloud load balancer, ingress
  controller, or host firewall that owns internet-facing TLS automation,
  DDoS/WAF policy, and source-network restrictions.

## Not Claimed

- HTTP/3 and QUIC are not supported.
- WebSocket-specific behavior is not part of the acceptance target.
- gRPC is not a declared compatibility target even though the underlying Go
  HTTP/2 stack handles HTTP/2 framing.
- Provider-specific behavior for every managed load balancer is not guaranteed
  by the generic test suite. Validate your exact cloud load balancer, ingress
  controller, TLS policy, idle timeout, and forwarding-header behavior before a
  production cutover.

## Load Balancer Checklist

For any external load balancer or ingress:

1. Forward only intended public listener ports to Gefahr.
2. Keep the admin listener private and separately restricted.
3. Set `client_ip.trusted_proxies` to the source CIDRs of the load balancer or
   ingress hops that sanitize and set the explicit `client_ip.headers` order,
   such as `X-Forwarded-For`.
4. Make the load balancer health check match the deployment model:
   public-data-plane checks should hit a route backed by an upstream, while
   private management checks should hit `/readyz` with the admin bearer token.
5. Align load balancer idle timeout with `timeouts.idle` and expected upstream
   response behavior.
6. Run `make acceptance` and a deployment-specific smoke test after every
   change to listener TLS, trusted proxy CIDRs, timeout policy, or backend TLS.
