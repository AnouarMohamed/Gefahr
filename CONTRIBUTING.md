# Contributing

Gefahr accepts changes that make the proxy more reliable, easier to operate, or
clearer to evaluate. Keep changes scoped and include proof.

## Before opening a PR

1. Read [README.md](README.md), [docs/completion.md](docs/completion.md), and
   [docs/compatibility.md](docs/compatibility.md).
2. Search existing issues and discussions.
3. For user-visible behavior changes, open or link an issue that explains the
   operational problem.
4. For security vulnerabilities, do not open a public issue. Follow
   [SECURITY.md](SECURITY.md).

## Development

```sh
make test
make coverage
make deploy-check
make acceptance
```

`make acceptance` is the release-quality engineering gate. It runs formatting
checks, `go vet`, race-enabled unit tests, deployment asset validation, and the
race-enabled integration suite.

## Pull request expectations

- Keep the PR focused on one problem.
- Add or update tests for code changes.
- Update docs for new config, metrics, deployment behavior, or limitations.
- Include rollback or compatibility notes when changing runtime behavior.
- Do not commit generated build output, local coverage files, or secrets.
- Explain verification commands and results in the PR body.

## Review standard

Maintainers review for correctness, operational safety, observability,
rollback behavior, and test quality. A change can be declined if it expands the
support surface without a clear production use case.

## Commit style

Use short, imperative commit subjects. Examples:

```text
feat(proxy): enforce route policy
fix(config): reject duplicate route names
docs: document Kubernetes rollout checks
```
