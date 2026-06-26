# Kubernetes deployment

The baseline manifest in [`deploy/kubernetes/goproxy.yaml`](../deploy/kubernetes/goproxy.yaml)
is intentionally conservative: non-root distroless container, read-only root
filesystem, no service account token, secret-backed admin token, exec probes,
ClusterIP services, and an admin-only NetworkPolicy for a `monitoring`
namespace.

Before applying it to a real cluster:

1. Replace the `goproxy-admin` secret value.
2. Pin the image to the exact release tag you intend to run.
3. Edit the ConfigMap route hosts, backend URLs, trusted ingress CIDRs, rate
   limits, cache policy, and any upstream TLS trust material.
4. Run `make deploy-check` and validate any rendered environment-specific
   config with `goproxy -config <path> -check-config`.
5. Ensure your cluster enforces NetworkPolicy; otherwise the admin Service is
   only protected by bearer authentication and cluster network placement.
6. Mount public TLS certificates or upstream CA/client certificates as Secrets
   when those config fields are enabled.

Apply and inspect:

```sh
kubectl apply -f deploy/kubernetes/goproxy.yaml
kubectl -n goproxy rollout status deployment/goproxy
kubectl -n goproxy get pods,svc,networkpolicy,pdb
```

The pod probes use `goproxy -healthcheck` as an exec probe. That process reads
`GOPROXY_ADMIN_TOKEN` from the container environment and sends the required
bearer token to the loopback admin listener, so probes continue to work when
`admin.auth_token_env` is enabled.

For rollback, pin the previous known-good image tag and run:

```sh
kubectl -n goproxy set image deployment/goproxy goproxy=ghcr.io/anrorg/gefahr:<previous-version>
kubectl -n goproxy rollout status deployment/goproxy
```

Operational expectations:

- Keep the admin Service restricted by NetworkPolicy and bearer authentication;
  the Kubernetes baseline binds the admin listener inside the pod so Prometheus
  or other monitoring workloads can scrape it through that restricted Service.
- Set `client_ip.trusted_proxies` to the CIDRs of your ingress/load-balancer
  hops, not to broad cluster ranges unless every source in that range is trusted
  to sanitize `X-Forwarded-For` and `X-Real-IP`.
- Use `maxUnavailable: 0` during rolling updates so at least one old pod remains
  available while new pods become ready.
- Watch `goproxy_requests_total`, `goproxy_retries_total`, backend health
  gauges, cache outcomes, and process runtime metrics during rollout.
- Treat ConfigMap changes as restart-triggering deployment changes; do not rely
  on Kubernetes automatically sending `SIGHUP`.
