# Operations runbooks

These runbooks assume the admin listener is private and authenticated with
`admin.auth_token_env`.

Production cutover readiness is tracked in
[`docs/production-transition.md`](production-transition.md). Disaster recovery
drills and evidence templates are tracked in
[`docs/disaster-recovery.md`](disaster-recovery.md).

## Upgrade

1. Verify the release artifacts and attestations from
   [`docs/release-acceptance.md`](release-acceptance.md).
2. Stage the new binary, image tag, or ConfigMap without deleting the previous
   known-good version.
3. Apply to one instance or one rollout batch.
4. Watch `/readyz`, `goproxy_requests_total`, 5xx rate,
   `goproxy_retries_total`, backend health gauges,
   `goproxy_policy_denials_total`, and
   `goproxy_rate_limit_decisions_total{decision="limited"}`.
5. Continue only after readiness is stable and error/retry rates match the
   previous baseline.

## Rollback

For Kubernetes, pin the previous image tag and wait for rollout status as
documented in [`docs/deployment-kubernetes.md`](deployment-kubernetes.md).

For systemd, restore the previous binary or config from the same host and run:

```sh
sudo systemctl reload goproxy   # config-only rollback
sudo systemctl restart goproxy  # binary or restart-only config rollback
```

After rollback, confirm `/readyz` returns `200`, request errors have returned to
baseline, and no old instance is still serving traffic unexpectedly.

Record rollback evidence using the templates in
[`docs/disaster-recovery.md`](disaster-recovery.md). A rollback procedure is not
production-ready until it has been run in staging with the same deployment path
used by production.

## Elevated 5xx

1. Split proxy errors by JSON `code`: `no_healthy_upstream`,
   `proxy_overloaded`, `bad_gateway`, and `upstream_timeout` have different
   owners.
2. Check backend health gauges and active-request gauges for the affected pool.
3. Inspect `goproxy_retries_total`; a rising retry rate usually means transport
   failures before full outage.
4. Compare upstream latency and response-header timeout with the configured
   `timeouts.response_header`.
5. Roll back the last route, pool, TLS, timeout, or backend deployment change if
   the error started immediately after a release.

## Unexpected 429

1. Check `goproxy_rate_limit_decisions_total{decision="limited"}` by route.
2. Confirm `client_ip.trusted_proxies` contains only the real ingress or load
   balancer CIDRs.
3. Confirm `client_ip.headers` matches only the headers those trusted hops
   sanitize and set.
4. Verify the ingress sanitizes `X-Forwarded-For` or the configured header;
   otherwise a spoofed chain can collapse many users into the wrong identity.
5. Increase the route budget only after confirming the limit is too low, not
   masking abusive traffic.

## Unexpected Policy Denials

1. Check `goproxy_policy_denials_total` by route and reason.
2. Compare the active route `policy` with the last known-good configuration.
3. For `method_not_allowed` and `path_denied`, verify the public API contract
   and any ingress path rewrites.
4. For header-related denials, confirm the trusted ingress is still adding or
   removing the expected headers before traffic reaches Gefahr.
5. For `query_too_large`, compare legitimate client URLs with the route's
   `max_query_bytes` and raise the bound only when the upstream accepts it.

## Admin Access Anomaly

1. Search JSON logs for `msg="admin request completed"` and
   `auth="unauthorized"` or `auth="forbidden"`.
2. Confirm the source belongs to an expected monitoring, orchestration, or
   operator network.
3. Use the `principal` field to identify which scoped credential was accepted
   before a forbidden response.
4. Rotate `GOPROXY_ADMIN_TOKEN` or the relevant scoped token if the source is
   unexpected or repeated.
5. Tighten NetworkPolicy, host firewall rules, or security groups before
   restoring broad access.
