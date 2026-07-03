---
title: Overview
section: Documentation
order: 10
summary: Learn what Gefahr is, where it fits, and which docs to read first.
---

# Gefahr documentation

![Gefahr turtle mascot and wordmark](gefahr_turtle_v3.png)

Gefahr is a configurable Go reverse proxy for teams that want an explicit,
file-based edge component they can test, review, deploy, and roll back like
the rest of their infrastructure.

Gefahr is not an ingress controller, service mesh, CDN, or all-purpose API
gateway. It is a focused reverse proxy with:

- Host and path-prefix routing.
- Round-robin and least-connections load balancing.
- Active health probes and passive failure ejection.
- Static request policy guardrails.
- Per-route, per-client rate limiting.
- Bounded shared response caching.
- Static TLS termination and HTTPS upstream trust controls.
- Private admin endpoints for health, readiness, and metrics.
- Strict YAML configuration with atomic reload.

## Who this documentation is for

Use these docs if you are:

- Running Gefahr in Kubernetes, systemd, a VM, or a small controlled edge.
- Reviewing whether the proxy is ready for production traffic.
- Debugging routing, readiness, health checks, rate limits, or upstream TLS.
- Writing operational procedures around release, rollback, and recovery.

If you are learning how the codebase evolved, use the source repository notes.
This site is for using and operating the product.

## Recommended paths

New users should read:

1. [Quickstart](#/documentation/quickstart)
2. [Install Gefahr](#/getting-started/install)
3. [Create your first proxy](#/getting-started/first-proxy)
4. [Configuration model](#/getting-started/configuration-model)

Operators should read:

1. [Deploy on Kubernetes](#/tasks/deploy-on-kubernetes)
2. [Run multiple Kubernetes replicas](#/tasks/kubernetes-multi-replica)
3. [Service discovery and shared state](#/tasks/service-discovery-shared-state)
4. [Scaling and sharding](#/tasks/scaling-sharding)
5. [Observability](#/operate/observability)
6. [Reloads and rollbacks](#/operate/reloads-and-rollbacks)
7. [Troubleshooting](#/operate/troubleshooting)
8. [Production checklist](#/operate/production-checklist)

Security reviewers should read:

1. [Request protection](#/concepts/request-protection)
2. [Security model](#/reference/security-model)
3. [Compatibility](#/reference/compatibility)
4. [Configuration reference](#/reference/configuration)

## What to keep outside Gefahr

Gefahr deliberately leaves some responsibilities to surrounding infrastructure:

| Responsibility | Expected owner |
|---|---|
| Internet DDoS absorption | Cloud load balancer, CDN, provider edge |
| Full WAF rule engine | Dedicated WAF or API gateway |
| ACME certificate automation | Ingress, load balancer, cert manager, or deployment pipeline |
| Dynamic service discovery | Kubernetes Service, load balancer, or config delivery system |
| User authentication | Application, identity proxy, or API gateway |
| Distributed cache coherence | CDN or application cache layer |
| Global rate limits | Ingress, API gateway, or future shared counter backend |
| Session affinity and sharding | Ingress, load balancer, or deployment topology |

This split keeps the proxy understandable. The configuration file should answer
what traffic is accepted, where it goes, how it is bounded, and how operators
know it is healthy.

## Release status

Gefahr should be treated as a production candidate only after the release
artifact, deployment path, configuration, dashboards, alerts, and rollback
procedure have all passed staging. The proxy is only one part of the production
system.
