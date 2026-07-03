# Service discovery and shared state

Gefahr's current Kubernetes service-discovery model is explicit and
conservative: the proxy reads backend URLs from `proxy.yaml`, and those backend
URLs should normally point at stable Kubernetes Service DNS names. Gefahr does
not watch EndpointSlices, Endpoints, pods, or any external discovery API.

This keeps the data plane independent of Kubernetes API availability and avoids
introducing a hidden control-plane dependency.

## Service discovery decision

| Model | Status | Contract |
|---|---|---|
| Static config with backend URLs | Supported | Backend URLs are declared in `proxy.yaml` and validated before startup or rollout. |
| Kubernetes Service DNS | Supported | Preferred cluster model. Point backends at stable Service names such as `http://app.default.svc.cluster.local:8080`. |
| EndpointSlice or Endpoints watch | Out of scope | Requires Kubernetes API access, RBAC, watch consistency handling, endpoint-change tests, and new operator docs before it can ship. |
| External discovery control plane | Out of scope | Requires a defined consistency, authentication, failure, and rollout model before it can ship. |

The baseline Kubernetes manifest keeps `automountServiceAccountToken: false`
because the proxy does not need to call the Kubernetes API for discovery.

## Shared-state decision

| Feature | Current scope | Shared-state requirement |
|---|---|---|
| Config snapshots | Shared operator intent through ConfigMap or mounted file | Roll out by changing the pod template annotation or checksum. |
| Admin credentials | Shared through Secrets | Roll pods after secret changes because env vars are read at process start. |
| TLS and upstream trust files | Shared through Secrets or mounted files | Roll or reload pods according to the deployment path after secret changes. |
| Backend membership | Owned by Kubernetes Service or external LB | Gefahr sees a stable Service address, not individual pod membership. |
| Backend health | Local pod state | Shared health requires a new coordinator or external health source. |
| Rate-limit counters | Local pod state | Global limits require ingress enforcement or a new external counter backend. |
| Cache entries | Local pod state | Shared cache requires a new cache backend and fail-open/fail-closed decision. |
| Load-balancer counters | Local pod state | Cross-pod balancing is owned by Kubernetes Service or ingress, not Gefahr. |
| Metrics | Local pod state | Scrape each pod and aggregate in monitoring. |
| Reload state | Local pod state | Kubernetes should use Deployment rollouts, not per-pod reload coordination. |

## Propagation model

Config and secret changes are deployment events:

1. Render and validate `proxy.yaml`.
2. Update ConfigMaps and Secrets.
3. Bump a pod-template config version or checksum annotation.
4. Let the Deployment roll pods with `maxUnavailable: 0`.
5. Watch readiness and per-pod metrics until the rollout converges.

Kubernetes can update projected ConfigMap and Secret files in place, but the
baseline does not rely on that for production changes. A rollout is easier to
audit and gives each pod a clean startup path with its config, credentials, and
TLS material.

## Failure model

- If the Kubernetes API is unavailable, existing pods continue serving because
  they do not depend on API watches.
- If a Kubernetes Service changes its selected endpoints, DNS and kube-proxy or
  the cluster dataplane own convergence. Gefahr continues to connect to the
  configured Service address.
- If one pod has stale config, traffic can reach it until Kubernetes drains it.
  Keep old and new configs compatible during rollout windows.
- If one pod loses backend reachability, that pod's local health checks should
  fail readiness while other pods can remain ready.
- If a future shared-state backend is introduced, the feature must document
  behavior when that backend is slow, unavailable, partitioned, or stale.

## Future EndpointSlice requirements

Do not add EndpointSlice or external discovery support as a small helper. It is
a control-plane feature. A safe implementation needs:

- RBAC limited to the exact watched resources.
- Watch reconnect, relist, and stale-data handling.
- Tests for endpoint add, remove, readiness changes, partial watch failure, and
  rollout races.
- Metrics that expose discovery freshness and backend membership version.
- Operator docs that explain consistency delays and failure behavior.
- A decision about whether active health checks remain local or derive from
  discovery state.

Until those pieces exist, use Kubernetes Services or a separately operated load
balancer as the backend membership boundary.

If scaling requirements need route, tenant, backend, or region ownership, use
the [scaling and sharding](scaling-sharding.md) contract before adding code.
