# Architecture

Each public request passes through an immutable runtime snapshot:

```text
listener -> limits -> route match -> policy -> rate limit -> cache -> balancer -> ReverseProxy -> backend
                                                              |                         |
                                                              +---- cached response <---+
```

Routes match an exact normalized host (or an explicit empty-host catch-all),
then the longest path-prefix boundary. A route chooses a backend pool and either
round-robin or least-connections selection. Static route policies can reject
disallowed methods, path prefixes, headers, or oversized query strings before
traffic reaches rate limiting. Optional route rate limits use the trusted client
identity: direct socket peer by default, or `client_ip.headers` only from a
trusted proxy CIDR. Active probes update health on thresholds; real
transport failures provide passive evidence and may eject a backend before the
next probe.

`httputil.ReverseProxy` streams messages and handles HTTP framing. Bounded
per-pool transports own connection pooling, upstream deadlines, and HTTPS
upstream trust/client-certificate policy. Safe replayable requests may be
retried once before response commitment. Forwarding headers are rebuilt after
Go removes inbound hop-by-hop and proxy headers, using the same trusted client
identity that powers rate limiting.

The response cache is a mutex-protected LRU bounded by entry count and accounted
bytes. Responses are captured while streaming and published only after EOF;
partial responses are never cached.

Reload builds and validates a complete replacement handler and all certificates
before one atomic pointer swap. Old requests retain the old snapshot. Health
workers are canceled and recreated for the new backend state.

Public and admin servers share coordinated startup and graceful shutdown. The
admin listener is deliberately separate so probes and metrics are not exposed
on the data plane.

In Kubernetes, multiple replicas are independent runtime snapshots behind a
Service or ingress. Config, secrets, image versions, and backend Service names
are shared by the deployment platform; active requests, health observations,
cache entries, rate-limit counters, load-balancer counters, metrics, and reload
state remain local to each pod.
