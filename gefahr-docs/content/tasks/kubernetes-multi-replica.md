---
title: Run multiple Kubernetes replicas
section: Tasks
order: 115
summary: Understand what remains local per pod, what is shared by Kubernetes, and when distributed state is required.
---

# Run multiple Kubernetes replicas

Gefahr can run as multiple Kubernetes pods when you treat each pod as a
stateless data-plane replica. The current product does not provide distributed
cache coherence, global rate-limit counters, shared backend health, or direct
EndpointSlice discovery.

## Deployment contract

- All replicas run the same image and rendered `proxy.yaml`.
- Config changes are applied through a Deployment rollout.
- Secrets provide admin tokens and TLS material.
- Backend URLs should normally point at stable Kubernetes Service DNS names.
- Runtime state is local to each pod unless a future feature explicitly adds
  shared state.

For the detailed discovery contract, see
[Service discovery and shared state](service-discovery-shared-state.md).

## Local state

These values are per pod:

- Active requests and connection counts.
- Backend health observations.
- Round-robin and least-connections balancer counters.
- Cache entries.
- Route rate-limit counters.
- Process and request metrics.
- Reload snapshots.

That means a route limit of `300/min` is `300/min` per pod, not a global
cluster limit. Enabling route cache improves per-pod hit rate only; cache
entries are not shared across pods.

## Shared operator state

These values are shared through Kubernetes or the deployment pipeline:

- ConfigMap or mounted config file for `proxy.yaml`.
- Secret-backed admin tokens.
- Secret-backed public TLS and upstream trust material.
- Image digest and rollout version.
- Service DNS names used for backend routing.

Kubernetes does not restart pods just because a ConfigMap changed. Bump a
pod-template config version or checksum annotation when config changes should
roll out.

## Rollout rules

- Use `maxUnavailable: 0`.
- Keep a PodDisruptionBudget.
- Let readiness fail before traffic is admitted.
- Keep old and new configs compatible during the rollout window.
- Prefer Deployment rollouts over per-pod `SIGHUP` in Kubernetes.

If one pod loses backend reachability, that pod should fail readiness while
other ready pods continue serving. If one pod is stale during rollout, traffic
can still reach it until Kubernetes drains it.

## When to add shared state

Add an external backend only when production requirements need:

- Global rate limits across replicas.
- Shared cache coherence.
- Cross-pod backend health decisions.
- Leader election or coordinated control-plane behavior.
- Direct EndpointSlice or external service-discovery integration.

Until those features exist, dashboards and runbooks should label rate limits,
cache, backend health, and metrics as per-pod behavior.
