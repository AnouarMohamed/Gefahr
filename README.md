# Gefahr
[![CodeQL](https://img.shields.io/github/actions/workflow/status/ANRorg/Gefahr/codeql.yml?label=CodeQL&style=flat-square)](https://github.com/ANRorg/Gefahr/actions/workflows/codeql.yml)
[![CI](https://img.shields.io/github/actions/workflow/status/ANRorg/Gefahr/ci.yml?branch=main&style=flat-square)](https://github.com/ANRorg/Gefahr/actions/workflows/ci.yml) [![Go Version](https://img.shields.io/github/go-mod/go-version/ANRorg/Gefahr?style=flat-square)](go.mod) [![License](https://img.shields.io/badge/license-Apache--2.0-blue?style=flat-square)](LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/ANRorg/Gefahr?style=flat-square)](https://goreportcard.com/report/github.com/ANRorg/Gefahr) [![codecov](https://img.shields.io/codecov/c/github/ANRorg/Gefahr?style=flat-square)](https://codecov.io/gh/ANRorg/Gefahr)

<p align="center">
  <img src="assets/gefahr_turtle_v3.png" alt="Gefahr turtle mascot and wordmark" width="560">
</p>

gefahr is a configurable Go reverse proxy with host/path routing, round-robin
and least-connections balancing, active and passive health tracking, bounded
response caching, per-route request policy guardrails and rate limiting, static
TLS termination, upstream TLS controls, structured logs, and Prometheus metrics.

The data plane uses Go's maintained `httputil.ReverseProxy`; Gefahr owns the
policy around it. See [the architecture decision](docs/adr/0001-proxy-foundation.md)
and [architecture overview](docs/architecture.md). Current product-readiness
status is tracked in [completion status](docs/completion.md).

## Quick start

Requirements: Go 1.25.11 or Docker with Compose.

Run two fixture backends in separate terminals:

```sh
go run ./test/fixtures/backend -address :9001 -name backend-1
go run ./test/fixtures/backend -address :9002 -name backend-2
```

Then run the proxy:

```sh
go run ./cmd/goproxy -config configs/proxy.example.yaml
curl -H 'Host: localhost' http://127.0.0.1:8080/
curl http://127.0.0.1:9090/readyz
curl http://127.0.0.1:9090/metrics
```

Or run the complete demonstration:

```sh
docker compose up --build
curl http://localhost:8080/
```

Tagged releases publish checksummed Linux, macOS, and Windows archives, SPDX
SBOMs, and GitHub provenance/SBOM attestations. The multi-architecture
container image is available as
`ghcr.io/anrorg/gefahr:<version>` for Linux AMD64 and ARM64.

The production image runs its health check against the private `/readyz`
endpoint by invoking the `goproxy` binary's bounded `-healthcheck` mode; it does
not add a shell or HTTP utility to the distroless image.

## Configuration and operation

Configuration is strict YAML: unknown fields and unsafe values stop startup or
reject reload. Copy `configs/proxy.example.yaml`, adjust listeners, routes, and
backend URLs, then pass it with `-config`.

- `SIGHUP` validates and atomically reloads routes, pools, policies, logging,
  and TLS certificate contents. Existing requests finish on their old snapshot.
- Listener addresses, listener count, TLS mode, the admin address, public
  server timeouts, shutdown timeout, and maximum header size require a restart.
  The per-listener connection limit is also restart-only.
- `SIGINT` and `SIGTERM` stop acceptance and drain requests within
  `timeouts.shutdown`.
- The admin listener should remain private. It exposes `/livez`, `/readyz`, and
  `/metrics`.

See the [configuration reference](docs/configuration.md) and
[operations runbook](docs/operations.md). Kubernetes deployment guidance is in
[the hardened baseline](docs/deployment-kubernetes.md); VM/bare-metal guidance
is in [the systemd baseline](docs/deployment-systemd.md). Incident and upgrade
procedures are in [runbooks](docs/runbooks.md). Protocol coverage is tracked in
the [compatibility matrix](docs/compatibility.md), and managed load balancer
guidance is in [cloud load balancer notes](docs/cloud-load-balancers.md). The
production cutover checklist is in
[production transition](docs/production-transition.md), with recovery drills in
[disaster recovery](docs/disaster-recovery.md).

## Development

```sh
make test
make test-race
make coverage
make check
make deploy-check    # validates configs and deployment assets
make test-integration # requires permission to open local TCP listeners
make acceptance       # static, race, unit, and real-socket integration checks
make docs             # builds the generated documentation site
docker compose up --build -d
make load-check      # exercises the running demonstration stack
```

Validate a config without starting listeners or resolving secret environment
variables:

```sh
goproxy -config /etc/goproxy/proxy.yaml -check-config
```

Release builds can inject identity with `make build VERSION=v1.0.0` or with
`GOPROXY_VERSION` and `GOPROXY_COMMIT` for Compose. Run `goproxy -version` to
inspect the embedded values.

Every repository commit is intentionally small and independently testable.
See the [release acceptance procedure](docs/release-acceptance.md) for the
complete final gate and expected evidence.

The generated documentation portal lives in `gefahr-docs`. Product-facing docs
are authored under `gefahr-docs/content` and built into the static site.

## Project governance

- Contributions: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security reporting: [SECURITY.md](SECURITY.md)
- Support policy: [SUPPORT.md](SUPPORT.md)
- Maintainer governance: [GOVERNANCE.md](GOVERNANCE.md)
- Ownership: [MAINTAINERS.md](MAINTAINERS.md)

## Security model and limitations

- Client forwarding headers are discarded and rebuilt from the trusted
  connection metadata.
- Request headers, bodies, network operations, cache memory, and shutdown are
  bounded.
- Per-route request policies can allowlist methods, deny path prefixes, require
  or deny headers, and cap query-string bytes.
- Shared caching bypasses authenticated, cookie-bearing, personalized,
  non-200, `private`, `no-store`, `no-cache`, and `Vary` responses.
- Static PEM certificates are loaded from deployment storage and must never be
  committed.

Version 1 does not include HTTP/3, ACME, dynamic service discovery, distributed
caching, cache revalidation, `Vary` variants, a mutation API, WAF behavior, or
per-route authentication. Static request policies are not a full WAF or bot
classification system. The response write timeout limits very long-lived
streams; WebSocket-specific behavior is not an acceptance target. See
[security and limitations](docs/security.md).

## License

Gefahr is licensed under the [Apache License 2.0](LICENSE).
