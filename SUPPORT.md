# Support

Gefahr is open-source infrastructure software. Support is community-driven
unless a separate commercial agreement exists outside this repository.

## Where to ask

- Usage questions and design discussions: GitHub Discussions.
- Reproducible bugs: GitHub Issues.
- Security vulnerabilities: private reporting through
  [SECURITY.md](SECURITY.md).
- Release or deployment readiness questions: include the output of
  `make acceptance`, `make coverage`, `make deploy-check`, and the relevant
  staging smoke evidence.

## What maintainers need

For proxy behavior issues, include:

- Gefahr version, commit, binary checksum, or image digest.
- Deployment mode: Kubernetes, systemd, Compose, or other.
- Redacted config.
- Request method, host, path, and key headers.
- Expected response and actual response.
- Relevant logs, metrics, and backend health state.

For deployment issues, include:

- Manifest, service unit, or package version.
- Exact command and output.
- Platform versions.
- Whether the same artifact passed staging.

## Scope

Maintainers can help diagnose project behavior and documentation gaps. They do
not operate user clusters, tune third-party load balancers, manage secrets, or
guarantee emergency response from public issues.
