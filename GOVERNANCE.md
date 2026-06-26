# Governance

Gefahr is maintained as production infrastructure. Project decisions should be
boring, documented, reversible when possible, and backed by tests or operational
evidence.

## Maintainer responsibilities

Maintainers are responsible for:

- Reviewing changes for correctness, operability, and rollback behavior.
- Keeping CI, release, docs, and deployment gates meaningful.
- Coordinating security triage and private advisories.
- Rejecting features that create an unsupported production surface.
- Recording known limitations instead of implying unsupported guarantees.

## Decision process

Small fixes can merge after maintainer review and passing checks. Larger
changes should have an issue or ADR covering:

- User or operator problem.
- Supported and unsupported behavior.
- Runtime, deployment, and rollback impact.
- Observability changes.
- Test and release acceptance plan.

When there is disagreement, maintainers should prefer the option that is easier
to operate, easier to test, and easier to roll back.

## Release authority

A release should not be tagged until:

- `make acceptance` passes.
- `make coverage` passes the enforced floor.
- `make deploy-check` passes.
- Release notes identify new behavior, compatibility changes, and known risks.
- Rollback evidence exists for the deployment path being promoted.

## Security authority

Security fixes can bypass the normal public issue process. Maintainers may
prepare private patches, temporary branches, and coordinated advisories before
opening public discussion.

## Ownership

Repository administrators own access control, branch protection, package
publishing credentials, and GitHub organization settings. Maintainers own code
review and release readiness. Operators own production configuration and
environment-specific validation.
