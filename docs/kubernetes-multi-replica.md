# Kubernetes multi-replica state model

The Kubernetes baseline supports multiple Gefahr pods as stateless data-plane
replicas. It does not provide distributed cache coherence, global rate-limit
counters, shared backend health, or direct EndpointSlice discovery.

Use this contract when running more than one replica.

## Supported model

- All replicas run the same image and the same rendered `proxy.yaml`.
- Config is distributed by Kubernetes ConfigMaps or mounted files, then applied
  through a Deployment rollout.
- Admin credentials and TLS material are distributed by Kubernetes Secrets or
  an equivalent secret manager.
- Backend membership is normally hidden behind Kubernetes Services. Gefahr
  points at stable Service DNS names rather than watching pod endpoints itself.
- Runtime state is local to each pod unless this document explicitly says
  otherwise.

## State decisions

| State | Scope | Current decision | Operator impact |
|---|---|---|---|
| Route, pool, listener, timeout, limit, policy config | Shared operator intent | Stored in config and rolled out by Deployment | Treat config changes as rollout changes; do not rely on ConfigMap projection alone. |
| Admin tokens | Shared secret | Loaded from Secret-backed environment variables at process start | Rotate by updating the Secret and rolling pods. |
| Public TLS and upstream trust material | Shared secret files | Mounted from Secrets when enabled | Rotate by updating the Secret and rolling or reloading pods according to the deployment path. |
| Active requests and connection counts | Local pod state | Never shared | Draining one pod does not drain other pods. |
| Backend health observations | Local pod state | Each pod probes and ejects backends independently | A pod with no healthy backend fails readiness while other pods can continue serving. |
| Load-balancer counters | Local pod state | Round-robin and least-connections state is per pod | Backend selection can differ by pod. |
| Cache entries | Local pod state | Cache is per pod and disabled in the baseline route | Enabling cache improves per-pod hit rate only; there is no cross-pod coherence. |
| Rate-limit counters | Local pod state | Route rate limits are per pod when enabled | A limit of `300/min` with two evenly used pods can allow about `600/min` cluster-wide. |
| Metrics | Local pod state | Each pod exports its own process and request metrics | Scrape every pod or aggregate through monitoring. |
| Reload snapshot | Local pod state | `SIGHUP` affects only the target process | In Kubernetes, prefer a Deployment rollout over per-pod reload signaling. |

## Service discovery decision

The supported cluster discovery model is Kubernetes Service-based discovery:
configure backend URLs as stable Service DNS names, such as
`http://app.default.svc.cluster.local:8080`.

The detailed discovery and shared-state contract is documented in
[service discovery and shared state](service-discovery-shared-state.md).

Direct EndpointSlice watching is out of scope for the current product. Use it
only after adding a clear consistency model, RBAC permissions, endpoint-change
tests, and operator documentation. External service discovery is also out of
scope unless a separate control plane owns membership and failure semantics.

## Rollout rules

- Use `maxUnavailable: 0` so old pods remain available until replacement pods
  are ready.
- Keep a PodDisruptionBudget so voluntary disruption does not remove all
  replicas.
- Bump a pod-template config version or checksum annotation for config-only
  changes. Kubernetes does not restart pods just because a ConfigMap changed.
- Treat listener addresses, listener TLS mode, admin address/auth fields,
  public server timeouts, shutdown timeout, maximum header size, and connection
  limits as restart changes.
- Let readiness fail before traffic is admitted. `/readyz` should be green only
  after every configured pool has at least one healthy backend from that pod's
  point of view.

During a rollout, old and new pods can briefly serve different config versions.
That is acceptable only when both versions are compatible with the same ingress,
backend, and client behavior. Stage incompatible changes as separate rollouts.

## Failure model

- If one pod loses backend reachability, that pod should fail readiness and be
  removed from Service endpoints while other ready pods continue.
- If one pod has stale config during a rollout, Kubernetes traffic can still
  reach it until it is drained. Keep old and new configs compatible during the
  rollout window.
- If a future external rate-limit or cache backend is unavailable, the product
  must define fail-open or fail-closed behavior before the feature ships.
- If NetworkPolicy is missing or bypassed, the admin Service is protected only
  by bearer auth and cluster network placement.

## When shared state is required

Add an external shared-state backend only for requirements that cannot be met
with local pod state:

- Global rate limits across replicas.
- Cache coherence or shared cache warming across replicas.
- Cross-pod backend health decisions.
- Leader election or coordinated control-plane behavior.
- Direct EndpointSlice or external discovery that changes backend membership
  without a normal Deployment rollout.

Until those features exist, document rate limits, cache, health, and metrics as
per-pod behavior in production runbooks and dashboards.
