# Security and known limitations

Gefahr treats HTTP parsing and framing as a security boundary and delegates it
to Go's standard library. It ignores client-provided forwarding chains unless
the direct peer matches a configured trusted proxy CIDR, and then only uses
the explicitly configured headers from that trusted hop. Error responses do not
expose transport details.
Ambiguous paths containing encoded or double-encoded separators, backslashes,
or dot segments are rejected before routing so the proxy and backend cannot
normalize a route boundary differently.

Resource controls include maximum headers and bodies, public read/write/idle
deadlines, upstream dial and response-header deadlines, bounded retries, a
bounded cache, static per-route request policy guardrails, bounded per-route
rate limiting, bounded client-identity state, and bounded graceful shutdown.
Route policies can allowlist methods, deny path prefixes, require or deny
headers, and cap raw query-string bytes before traffic reaches rate limiting or
an upstream.

Admin endpoints can require bearer-token authentication with
`admin.auth_token_env` or named scoped tokens under `admin.tokens[]`. Scoped
tokens can grant `health`, `metrics`, `read`, or `admin`. They are still
designed for private management networks rather than direct internet exposure.
Admin requests are audited with path, status, auth result, principal, remote
address, and duration, without logging bearer tokens.

TLS listeners require a certificate/key pair and TLS 1.2 or newer. Private keys
must be mounted read-only with least-privilege filesystem permissions. HTTPS
upstreams can use the host trust store, configured CA files, SNI overrides, and
client certificates for mTLS. Backend connections are direct and do not inherit
ambient `HTTP_PROXY` or `HTTPS_PROXY` settings.

The cache deliberately bypasses every response containing `Vary` because
variant keying is not implemented. It also lacks revalidation, stale serving,
and distributed coherence. Cache contents are process-local and disappear on
restart.

Gefahr has scoped admin bearer tokens, but no external identity provider
integration, adaptive bot classification, full WAF rule engine, HTTP/3, ACME
client, dynamic service discovery, distributed cache, global rate-limit
counters, shared backend health coordinator, or configuration mutation API.
Place the admin listener on a trusted network and add external controls where
those capabilities are required.
