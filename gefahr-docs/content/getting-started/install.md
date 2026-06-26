---
title: Install Gefahr
section: Getting started
order: 30
summary: Choose a binary, container image, or source build and verify the release artifact before use.
---

# Install Gefahr

Gefahr can run as a local binary, a container image, or a source build. For
production, pin an exact release artifact or image digest.

## Install from source

Use this path for development and local validation:

```sh
git clone https://github.com/ANRorg/Gefahr.git
cd Gefahr
go build -trimpath -o bin/goproxy ./cmd/goproxy
./bin/goproxy -version
```

## Build a release binary locally

```sh
make build VERSION=v1.0.0
./bin/goproxy -version
```

The build target injects version and commit metadata into the binary.

## Use the container image

Release images are published to GitHub Container Registry:

```sh
docker pull ghcr.io/anrorg/gefahr:<version>
```

For production, deploy by digest:

```sh
ghcr.io/anrorg/gefahr@sha256:<digest>
```

Pinning by digest prevents a mutable tag from changing what runs in a cluster.

## Verify a release

Before promoting a release, record:

- Binary checksum or container image digest.
- SBOM artifact.
- GitHub provenance or SBOM attestation.
- Output of `make acceptance`.
- Output of `make coverage`.

The acceptance gate proves the repository state, not your deployment
environment. Always run a staging smoke test with the same config and artifact
you will use in production.

## File layout

A typical host layout is:

```text
/usr/local/bin/goproxy
/etc/goproxy/proxy.yaml
/etc/goproxy/tls/public.crt
/etc/goproxy/tls/public.key
/etc/goproxy/upstream-ca.pem
```

The process needs read access to the config and mounted certificates. Private
keys should be read-only and scoped to the proxy user.
