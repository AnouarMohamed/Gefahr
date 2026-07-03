---
title: Scaling and sharding
section: Tasks
order: 117
summary: Scale Gefahr with stateless replicas today and understand what requires a future sharding or shared-state design.
---

# Scaling and sharding

Gefahr currently scales vertically by giving one process more resources, and
horizontally by running multiple stateless replicas behind Kubernetes Services,
an ingress controller, a cloud load balancer, or another external traffic
distributor.

Gefahr does not implement built-in sharding, tenant ownership, global session
affinity, distributed cache coherence, global rate limits, or cross-pod backend
health propagation.

## Supported scale model

| Layer | Current owner |
|---|---|
| Public traffic distribution | Ingress, Service, load balancer, or host firewall |
| Route selection | Each Gefahr process using the same config |
| Backend membership | Kubernetes Service, external load balancer, or static URL |
| Pool balancing counters | Local Gefahr process |
| Request limits | Local Gefahr process |
| Route rate limits | Local Gefahr process |
| Cache entries | Local Gefahr process |
| Metrics | Local process, aggregated by monitoring |

This model is safe when any request can land on any replica and receive the
same policy and routing decision.

## Capacity planning

Treat these values as per-pod capacity in Kubernetes:

- `limits.max_concurrent_requests`
- `limits.max_connections`
- cache bounds
- route rate limits
- backend health observations
- balancer counters

Cluster-wide capacity is approximately per-pod capacity multiplied by ready
replicas only when traffic is evenly distributed.

## Session affinity

Gefahr does not provide sticky sessions or cookie-based affinity. If an
application requires affinity, configure it in the ingress/load balancer or
remove the application dependency on affinity.

## Sharding

Do not add sharding as an implementation detail. Sharding is a product decision
because it changes routing semantics, dashboards, failure domains, rollout
strategy, and incident response.

Possible future ownership models:

- Route shard.
- Tenant or hostname shard.
- Backend-pool shard.
- Region or provider shard.

Each model needs explicit config shape, validation, metrics labels, rollback
behavior, and operator docs before code changes.

## What stays outside Gefahr today

- Global rate limits: ingress, API gateway, or future shared counter backend.
- Shared cache: CDN, application cache, or future external cache backend.
- Backend membership: Kubernetes Services or external load balancers.
- Session affinity: ingress or load balancer.
- Region or tenant partitioning: deployment topology.
