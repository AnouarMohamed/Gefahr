---
title: Release artifacts
section: Reference
order: 260
summary: Understand release archives, Debian packages, Homebrew formula output, SBOMs, attestations, checksums, and cosign signatures.
---

# Release artifacts

Tagged releases publish multiple artifact types. Treat the release as usable
only after checksums, attestations, and signatures have been verified.

## Published files

| Artifact | Purpose |
|---|---|
| `gefahr_<version>_<os>_<arch>.tar.gz` | Unix binary archives |
| `gefahr_<version>_windows_amd64.zip` | Windows binary archive |
| `gefahr_<version>_linux_<arch>.deb` | Debian package for Linux AMD64 and ARM64 |
| `gefahr.rb` | Generated Homebrew formula for tap publication |
| `checksums.txt` | SHA-256 checksums for release files |
| `goproxy.spdx.json` | Source SBOM |
| `*.sig` and `*.pem` | Keyless cosign signature and signing certificate |

The container image is published to GitHub Container Registry:

```text
ghcr.io/anrorg/gefahr:<version>
```

Stable SemVer releases also publish `latest`.

## Verify a file signature

```sh
cosign verify-blob dist/gefahr_1.0.2_linux_amd64.tar.gz \
  --signature dist/gefahr_1.0.2_linux_amd64.tar.gz.sig \
  --certificate dist/gefahr_1.0.2_linux_amd64.tar.gz.pem \
  --certificate-identity-regexp 'https://github.com/ANRorg/Gefahr/.github/workflows/release.yml@refs/tags/v1.0.2' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Verify an image signature

```sh
cosign verify ghcr.io/anrorg/gefahr:v1.0.2 \
  --certificate-identity-regexp 'https://github.com/ANRorg/Gefahr/.github/workflows/release.yml@refs/tags/v1.0.2' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Verify attestations

```sh
gh attestation verify dist/gefahr_1.0.2_linux_amd64.tar.gz \
  -R ANRorg/Gefahr
```

```sh
gh attestation verify oci://ghcr.io/anrorg/gefahr:v1.0.2 \
  -R ANRorg/Gefahr
```

## Package repositories

The release workflow can publish package-manager artifacts when repository
destinations are configured.

Homebrew tap publication requires a tap repository and write token. The workflow
copies the generated formula to `Formula/gefahr.rb`.

Apt publication requires an apt repository, write token, and GPG private key.
The workflow publishes `.deb` files, `Packages`, `Packages.gz`, `Release`,
`InRelease`, and `Release.gpg`.

If those settings are absent, package repository publication is skipped and the
release still includes the `.deb` files and Homebrew formula as downloadable
release assets.
