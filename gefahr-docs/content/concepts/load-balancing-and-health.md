---
title: Load balancing and health
section: Concepts
order: 80
summary: Understand backend pools, balancing strategies, active probes, passive failure evidence, and readiness.
---

# Load balancing and health

A pool is a set of interchangeable upstream backends. Routes choose a pool,
then the pool strategy chooses a healthy backend.

## Strategies

Gefahr supports:

| Strategy | Behavior |
|---|---|
| `round_robin` | Rotates across healthy backends in order |
| `least_connections` | Picks the healthy backend with the fewest active assigned requests |

Use `round_robin` for most stateless upstreams. Use `least_connections` when
requests have uneven duration and you want to avoid piling work onto one
backend.

## Health checks

Each pool defines an active health probe:

```yaml
health:
  path: /health
  interval: 5s
  timeout: 1s
  unhealthy_threshold: 2
  healthy_threshold: 1
```

A backend becomes unhealthy only after consecutive failures reach
`unhealthy_threshold`. It becomes healthy again after consecutive successes
reach `healthy_threshold`.

In Kubernetes with multiple replicas, each pod keeps its own health
observations. One pod can fail readiness for a pool while another pod still has
healthy reachability.

## Passive failure evidence

Real transport failures can mark a backend unhealthy before the next probe.
This helps move traffic away from a backend that is failing between health
intervals.

Client body errors do not eject a backend. A client sending an oversized or
broken request is not evidence that the upstream is unhealthy.

## Readiness

`/readyz` succeeds only when every configured pool has at least one healthy
backend.

Use readiness differently depending on deployment:

- Public load balancer health checks can hit a real route if you want to prove
  the proxy and upstream are both available.
- Private orchestration checks can hit `/readyz` on the admin listener.

## No healthy backend

If a route matches but the pool has no healthy backend, Gefahr returns:

```json
{"code":"no_healthy_upstream","message":"no healthy upstream"}
```

Alert on this condition. It usually means an upstream outage, bad health path,
bad network policy, or a rollout that removed too many backends at once.
