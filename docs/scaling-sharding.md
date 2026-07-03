# Scaling and sharding

Gefahr currently scales in two conservative ways:

- Vertically, by increasing CPU, memory, file-descriptor limits, and the
  configured request/connection bounds for one process.
- Horizontally, by running multiple stateless replicas behind Kubernetes
  Services, an ingress controller, a cloud load balancer, or another external
  traffic distributor.

Gefahr does not currently implement product-level sharding, tenant ownership,
global session affinity, distributed cache coherence, global rate limits, or
cross-pod backend health propagation.

## Supported scale model

| Layer | Current owner | Contract |
|---|---|---|
| Public traffic distribution | Ingress, Service, load balancer, or host firewall | External infrastructure decides which Gefahr replica receives a connection. |
| Route selection | Gefahr process | Every replica uses the same rendered config and applies the same route table. |
| Backend membership | Kubernetes Service, external load balancer, or static backend URL | Gefahr connects to configured backend URLs; it does not discover pod endpoints itself. |
| Backend selection inside a pool | Gefahr process | Round-robin and least-connections counters are local to the process. |
| Request admission limits | Gefahr process | Body, header, connection, and concurrency limits are local to the process. |
| Route rate limits | Gefahr process | Counters are local to the process unless an external/global limiter is used. |
| Cache | Gefahr process | Cache entries are local to the process. |
| Observability | Gefahr process plus monitoring system | Scrape every replica and aggregate in monitoring. |

This model is safe when any request can land on any replica and still receive
the same policy and routing decision.

## Capacity planning

Scale the deployment from observed bottlenecks:

- CPU saturation: add replicas or CPU requests/limits.
- Memory pressure: reduce cache/body/concurrency bounds or add memory.
- File descriptor pressure: raise host/container limits and verify
  `limits.max_connections`.
- Backend saturation: scale upstreams or lower per-route/request concurrency.
- Elevated local rate limiting: decide whether the intended limit is per pod,
  ingress-wide, or global before changing budgets.

For Kubernetes, treat `limits.max_concurrent_requests`,
`limits.max_connections`, cache bounds, and route rate limits as per-pod
capacity. Cluster-wide capacity is roughly the per-pod budget multiplied by the
number of ready replicas only when traffic is evenly distributed.

## Session affinity

Gefahr does not provide sticky sessions or cookie-based affinity. If an
application requires affinity, put that policy in the ingress/load balancer or
remove the application dependency on affinity. Do not rely on Gefahr's local
round-robin or least-connections counters for stable client-to-backend mapping.

## Sharding decision

Sharding is intentionally not implemented as an implicit scaling detail.
Sharding changes routing semantics, failure domains, dashboards, rollout
strategy, and incident response.

Only add sharding after the product owner chooses a clear ownership model:

- Route shard: specific routes belong to specific deployment groups.
- Tenant shard: hostnames, tenants, or path namespaces belong to specific
  deployment groups.
- Backend shard: upstream pools are partitioned and not globally visible.
- Region shard: traffic is intentionally split by location or provider.

Each model needs explicit config shape, validation, metrics labels, rollback
behavior, and operator docs before code changes.

## Future shared components

These features require new design before implementation:

- Global rate-limit backend.
- Distributed or externally backed cache.
- Shared backend health coordinator.
- EndpointSlice or external service-discovery watcher.
- Leader election or another control-plane coordination primitive.
- Built-in shard routing or shard ownership metadata.

For each future component, define authentication, failure behavior,
observability, consistency, and rollout behavior before adding dependencies.

## Operational rule

If the deployment needs global behavior, enforce it outside Gefahr today:

- Use ingress/API-gateway controls for global rate limits and affinity.
- Use a CDN or application cache for shared cache behavior.
- Use Kubernetes Services or external load balancers for backend membership.
- Use deployment topology, not hidden proxy logic, for region or tenant
  partitioning.
