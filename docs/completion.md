# Product readiness status

Gefahr has a working reverse-proxy core, but it is not yet a finished product.
The current implementation follows the accepted
[proxy foundation ADR](adr/0001-proxy-foundation.md), not the original
from-scratch lab constraints archived under [`docs/legacy-guide`](legacy-guide/README.md).

## Shipped core

| Area | Status | Evidence |
|---|---|---|
| Routing | Shipped | Exact host matching, catch-all routes, longest boundary-safe path prefixes, and ambiguous path rejection are implemented and tested in `internal/routing` and `internal/proxy`. |
| Forwarding | Shipped | The data plane uses Go's maintained `httputil.ReverseProxy`; forwarding headers are discarded and rebuilt from trusted socket or trusted-proxy metadata before backend dispatch. |
| Load balancing | Shipped | Round-robin and least-connections balancers are implemented with no-healthy-backend handling and race-tested unit coverage. |
| Health | Shipped | Active probes and passive transport-failure ejection update backend eligibility; readiness requires every pool to have a healthy backend. |
| Caching | Shipped | The shared cache is TTL-based, LRU-bounded by entry count and bytes, and rejects unsafe methods, personalized requests, `Set-Cookie`, `private`, `no-store`, `no-cache`, malformed freshness directives, and `Vary` responses. |
| Request policy | Shipped | Per-route static guardrails can allowlist methods, deny path prefixes, require or deny headers, cap query-string bytes, and emit bounded denial metrics before backend admission. |
| TLS | Shipped | Public listeners can terminate static PEM certificates with TLS 1.2 minimum; HTTPS upstreams support custom CA files, SNI override, and client certificates for mTLS. |
| Limits and timeouts | Shipped | Public servers enforce header, body, concurrency, connection, idle, read, write, upstream dial, upstream response, trusted-client route rate-limit, and shutdown bounds. |
| Reload | Shipped | `SIGHUP` validates and stages a complete replacement snapshot before atomic publication; rejected reloads retain the previous snapshot. |
| Observability | Shipped | JSON request logs, admin audit logs, `/livez`, `/readyz`, `/metrics`, request metrics, cache metrics, policy-denial metrics, rate-limit decision metrics, retry metrics, and backend health/active gauges are implemented. |
| Test coverage | Shipped | `make coverage` and CI enforce an 85% repository coverage floor; the current measured total is 88.1%. |
| Admin auth | Shipped | Admin endpoints can require a full-scope bearer token via `admin.auth_token_env` or named scoped tokens via `admin.tokens[]`; unauthenticated non-loopback admin listeners are rejected; admin audit logs record auth result and principal without logging tokens. |
| Compatibility | Shipped | The real-socket integration suite covers cleartext HTTP/1.1 clients, TLS HTTP/2 clients, cleartext HTTP/1.1 upstreams, and HTTPS HTTP/2 upstreams; the documented matrix records supported and unsupported paths. |
| Kubernetes baseline | Shipped | A hardened manifest includes secret-backed admin auth, exec probes, read-only non-root pods, restricted admin networking, Service DNS backend discovery, no API credentials, services, PodDisruptionBudget, rollout safety settings, and documented multi-replica/shared-state contracts. |
| systemd baseline | Shipped | A hardened service unit, VPS production config, and environment-file template cover VM and bare-metal deployments with non-root execution, host-level sandboxing, config preflight, TLS paths, and secret-backed admin auth. |
| Deployment validation | Shipped | `goproxy -check-config` validates config without starting listeners, `make deploy-check` validates repository configs and deployment assets, and CI runs the deployment gate. |
| Release integrity | Shipped | Release workflow publishes archives, Debian packages, a Homebrew formula artifact, images, checksums, SBOMs, GitHub provenance/SBOM attestations, and keyless cosign signatures. |
| Operations | Shipped | Docker Compose, Kubernetes and systemd baselines, cloud load-balancer notes, distroless runtime image, executable health check mode, release acceptance, load-check instructions, production-transition checklist, incident runbooks, and disaster-recovery drill templates exist. |

## Product gaps

| Gap | Why it matters |
|---|---|
| Traffic protection | Static per-route request guardrails and rate limiting exist, but there is no full WAF rule engine, bot classification, or adaptive abuse-control policy. |
| Access-control model | Scoped admin bearer tokens exist for health, metrics, read, and admin access, but there is no external identity-provider integration. |
| Distributed cluster state | Kubernetes replicas are supported as stateless data-plane pods with Service DNS backend discovery, but cache entries, route rate-limit counters, backend health observations, balancer counters, reload snapshots, and process metrics are per pod. There is no distributed cache, global rate-limit backend, EndpointSlice watcher, external discovery integration, or shared health coordinator. |
| Package repositories | Release workflow can publish to a Homebrew tap and signed apt repository when the required repositories, tokens, and apt signing key are configured. |

## Superseded legacy requirements

The legacy guide intentionally taught a manual HTTP/1.1 proxy built on `net`,
`bufio`, `tls`, and `io`. ADR 0001 supersedes that constraint for the current
product: manual request/response parsers and the bare TCP proxy phase are not
version 1 requirements. The project delegates HTTP framing and parser security
boundaries to Go's standard `net/http` stack, while keeping routing, balancing,
health, cache policy, limits, reloads, telemetry, and lifecycle behavior in
Gefahr-owned code.

## Acceptance command

Run the current engineering gate from a clean worktree:

```sh
make acceptance
```

That command verifies formatting, `go vet`, all unit tests under the race
detector, deployment asset validation, and the real-socket integration suite
under the race detector. Run `make coverage` to verify the repository coverage
floor locally.
