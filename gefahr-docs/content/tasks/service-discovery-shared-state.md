---
title: Service discovery and shared state
section: Tasks
order: 116
summary: Use Kubernetes Services for backend membership and understand which state is local, shared, or intentionally out of scope.
---

# Service discovery and shared state

Gefahr's current Kubernetes service-discovery model is explicit: backend URLs
come from `proxy.yaml`, and in-cluster backends should normally be stable
Kubernetes Service DNS names.

Gefahr does not watch EndpointSlices, Endpoints, pods, or an external
discovery API. The baseline pod does not need Kubernetes API credentials for
discovery.

## Supported discovery

Use static config:

```yaml
pools:
  app:
    backends:
      - name: app
        url: http://app.default.svc.cluster.local:8080
```

Kubernetes owns pod membership behind the Service. Gefahr owns routing,
request policy, local health checks, retries, cache policy, and upstream
forwarding to the configured Service address.

## Shared state

| State | Current owner |
|---|---|
| `proxy.yaml` | ConfigMap, mounted file, or deployment pipeline |
| Admin tokens | Secret-backed environment variables |
| TLS and upstream trust files | Secrets or mounted files |
| Backend membership | Kubernetes Service or external load balancer |
| Backend health observations | Local pod |
| Rate-limit counters | Local pod |
| Cache entries | Local pod |
| Metrics | Local pod |
| Reload snapshot | Local pod |

Global rate limits require ingress enforcement or a future external counter
backend. Shared cache requires a future cache backend. Shared health requires a
future coordinator or external health source.

## Change propagation

Treat config and secret changes as rollout events:

1. Render and validate `proxy.yaml`.
2. Update ConfigMaps and Secrets.
3. Bump a pod-template config version or checksum annotation.
4. Let the Deployment roll pods with `maxUnavailable: 0`.
5. Watch readiness and per-pod metrics until the rollout converges.

## Out of scope

EndpointSlice watching and external service discovery are control-plane
features, not small runtime helpers. Before adding them, define RBAC, watch
reconnect behavior, stale-data handling, endpoint-change tests, discovery
freshness metrics, and operator documentation.

Until then, use Kubernetes Services or a separately operated load balancer as
the backend membership boundary.

If scaling requirements need route, tenant, backend, or region ownership, use
the [Scaling and sharding](scaling-sharding.md) contract before adding code.
