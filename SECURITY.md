# Security Policy

Gefahr is a reverse proxy and should be treated as security-sensitive
infrastructure.

## Supported versions

Security fixes target the latest released minor version and `main`. Older
versions may receive fixes when a maintainer can produce and validate a patch
without delaying protection for current users.

## Report a vulnerability

Do not report vulnerabilities in public issues or discussions.

Use GitHub private vulnerability reporting:

https://github.com/ANRorg/Gefahr/security/advisories/new

Include:

- Affected version, commit, image digest, or package checksum.
- Configuration needed to reproduce the issue.
- Request, response, log, or metric evidence with secrets removed.
- Whether the issue affects confidentiality, integrity, availability, or
  operator control.
- Any known exploitability constraints.

## Response targets

| Stage | Target |
|---|---:|
| Initial acknowledgement | 3 business days |
| Triage decision | 7 business days |
| Fix plan for confirmed high impact issues | 14 business days |

These are targets, not guarantees. Availability, maintainer capacity, and
third-party dependency timelines can affect delivery.

## Disclosure

Maintainers coordinate disclosure through GitHub Security Advisories when a
private report is confirmed. Public advisories should include affected versions,
impact, mitigation, fixed versions, and verification guidance.

## Security boundaries

Gefahr does not claim to be a WAF, bot detector, identity provider, ACME
certificate manager, or dynamic service-discovery plane. See
[docs/security.md](docs/security.md) and
[docs/compatibility.md](docs/compatibility.md) before treating the proxy as a
control boundary.
