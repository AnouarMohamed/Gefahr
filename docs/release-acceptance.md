# Release acceptance

Run the final gate from a clean worktree on the release commit.

```sh
make acceptance
make coverage
make deploy-check
docker compose up --build -d
make load-check
docker compose stop --timeout 35
```

`make acceptance` verifies formatting, `go vet`, all unit tests under the race
detector, deploy asset validation, and the real-socket integration suite under
the race detector. The integration suite covers routing, balancing, caching,
atomic reload publication, rejected-reload retention, HTTP/2 frontend and
upstream compatibility, and retry after a real upstream connection failure.

`make coverage` verifies the repository coverage floor. The CI workflow enforces
the same 85% minimum after running race-enabled coverage.

`make deploy-check` validates repository config examples, extracts and validates
the Kubernetes `proxy.yaml` ConfigMap payload, and runs host-specific validators
for Docker Compose, Kubernetes manifests, and systemd units when those tools are
available.

The load check performs an unmeasured warm-up, records process metrics, sends a
concurrent cache-bypassing workload, closes idle client connections, and samples
the process again after a settling interval. Acceptance requires zero failed
requests and settled goroutine growth no greater than the configured threshold.
Treat sustained heap growth across repeated runs as a failure requiring
investigation; a single before/after heap delta is not proof of a leak because
Go retains heap spans for reuse.

After stopping the stack, verify that the proxy exits within the configured
shutdown timeout and that no Gefahr Compose containers remain. Record the command
output in the release or CI evidence rather than committing machine-specific
throughput and memory numbers to this document.

Pushing an annotated `v*` tag runs the release workflow. It creates the GitHub
release when necessary, uploads checksummed archives for supported operating
systems, publishes Debian packages for Linux AMD64/ARM64, uploads a generated
Homebrew formula artifact, publishes AMD64/ARM64 images to GHCR, generates SPDX
SBOMs, creates GitHub artifact attestations for binaries and container images,
and adds keyless cosign signatures plus signing certificates. Confirm both
workflow jobs complete before announcing the release.

Verify release integrity from a machine with GitHub CLI access:

```sh
gh attestation verify dist/gefahr_1.0.2_linux_amd64.tar.gz -R ANRorg/Gefahr
gh attestation verify oci://ghcr.io/anrorg/gefahr:v1.0.2 -R ANRorg/Gefahr
```

For SBOM attestations, include the SPDX predicate type:

```sh
gh attestation verify dist/gefahr_1.0.2_linux_amd64.tar.gz \
  -R ANRorg/Gefahr \
  --predicate-type https://spdx.dev/Document/v2.3
```

Verify cosign signatures for release files:

```sh
cosign verify-blob dist/gefahr_1.0.2_linux_amd64.tar.gz \
  --signature dist/gefahr_1.0.2_linux_amd64.tar.gz.sig \
  --certificate dist/gefahr_1.0.2_linux_amd64.tar.gz.pem \
  --certificate-identity-regexp 'https://github.com/ANRorg/Gefahr/.github/workflows/release.yml@refs/tags/v1.0.2' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Verify the signed image digest:

```sh
cosign verify ghcr.io/anrorg/gefahr:v1.0.2 \
  --certificate-identity-regexp 'https://github.com/ANRorg/Gefahr/.github/workflows/release.yml@refs/tags/v1.0.2' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Package-manager artifacts are attached to the release but not published to a
package repository or Homebrew tap unless the publication variables and secrets
documented in [`packaging/README.md`](../packaging/README.md) are configured.
For Debian-based hosts, install the `.deb` artifact only after verifying
checksums, attestations, and cosign signatures. For Homebrew, review the
generated URLs and SHA-256 values before publishing to a tap.

Before promoting a release to production, complete the
[production transition checklist](production-transition.md) and attach disaster
recovery drill evidence from [disaster recovery drills](disaster-recovery.md).
