---
title: Production checklist
section: Operate
order: 190
summary: Confirm artifact integrity, config review, deployment, observability, rollout, and rollback before production cutover.
---

# Production checklist

Use this checklist before routing production traffic through Gefahr.

## Build and release

- `make acceptance` passed on the release commit.
- `make coverage` passed the enforced coverage floor.
- `make deploy-check` validated configs and deployment assets.
- Binary checksums or image digest were recorded.
- SBOM and provenance attestations were generated and verified.
- Previous known-good artifact remains available.

## Configuration

- Route hosts and path prefixes match the public traffic contract.
- Production config passes `goproxy -config <path> -check-config`.
- `timeouts.*` match client, load balancer, and upstream behavior.
- `limits.*` match host capacity and expected traffic.
- Route `policy` matches allowed methods, denied paths, ingress headers, and
  query-size expectations.
- Rate limits are intentionally configured and reviewed.
- `client_ip.trusted_proxies` contains only real ingress or load balancer CIDRs,
  and `client_ip.headers` names only headers those hops sanitize and set.
- For Kubernetes replicas greater than one, cache, rate limits, backend health,
  and metrics are reviewed as per-pod behavior unless another control provides
  global behavior.
- Kubernetes backend URLs point at stable Services or explicitly approved
  external load balancers; the baseline does not use EndpointSlice or external
  discovery watchers.
- Any required session affinity, tenant routing, region routing, or shard
  ownership is implemented by ingress/load-balancer/deployment topology, not
  assumed to exist inside Gefahr.
- Upstream TLS CA, SNI, and client cert settings are tested in staging.

## Deployment

- Admin listener is private.
- Admin auth is enabled with `admin.auth_token_env` or scoped `admin.tokens[]`.
- Monitoring uses a scoped read or metrics token instead of the full operator
  token.
- Kubernetes config-only changes bump a pod-template config version or checksum
  annotation so pods roll with the new ConfigMap.
- Public TLS and upstream TLS secrets are mounted read-only.
- Probes use either a real public route or private `/readyz` with admin auth.
- Graceful shutdown is shorter than orchestrator termination grace.
- Load balancer idle timeout aligns with `timeouts.idle`.

## Observability

- Metrics are scraped from the private admin path.
- Logs are searchable by request ID.
- Admin audit logs are searchable by source, principal, and auth result.
- Dashboards include status, latency, retries, backend health, active backend
  requests, cache outcomes, policy denials, rate limits, memory, and goroutines.
- Alerts exist for readiness failure, 5xx, retries, no healthy upstream,
  overload, unexpected policy denials, unexpected 429, and unauthorized admin
  access.

## Rollout

- Staging used the same artifact and config.
- Canary or one-batch criteria are defined.
- Rollout pauses on readiness failure, 5xx, retry spikes, policy denial spikes,
  or unexpected rate limiting.
- Rollback command is known before rollout starts.
- Release owner records start time, version, config checksum, and decision
  points.
