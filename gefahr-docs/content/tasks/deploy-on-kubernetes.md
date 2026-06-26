---
title: Deploy on Kubernetes
section: Tasks
order: 110
summary: Run Gefahr with a hardened Deployment, private admin Service, probes, NetworkPolicy, and safe rollout practices.
---

# Deploy on Kubernetes

Use the Kubernetes deployment when you want orchestrated rollout, health
checks, PodDisruptionBudget, non-root runtime, and private admin access.

## Deployment shape

A production deployment should include:

- A Deployment for the proxy.
- A ConfigMap or mounted config file for `proxy.yaml`.
- Secrets for admin bearer token and TLS material.
- A public Service for data-plane traffic.
- A private Service for admin endpoints.
- Readiness and liveness probes.
- A NetworkPolicy that limits admin access.
- A PodDisruptionBudget.

## Pin the image

Use an immutable digest:

```yaml
image: ghcr.io/anrorg/gefahr@sha256:<digest>
```

Avoid `latest`. Avoid mutable tags for production.

## Validate before apply

Run the repository deployment gate before promoting a manifest:

```sh
make deploy-check
```

For environment-specific config, validate the rendered file too:

```sh
goproxy -config ./rendered/proxy.yaml -check-config
```

The repository gate extracts the `proxy.yaml` payload from
`deploy/kubernetes/goproxy.yaml` and validates it with the same config loader
used by the running proxy.

## Admin token

Set `admin.auth_token_env` in config:

```yaml
admin:
  address: "0.0.0.0:9090"
  auth_token_env: GOPROXY_ADMIN_TOKEN
```

Mount the token as an environment variable from a Secret. The admin Service
should remain private.

## Probes

Use `/readyz` when you want orchestration to remove the pod from service if
any pool has no healthy backend.

Use `/livez` when you only want to know whether the process should be restarted.

When admin auth is enabled, probes must include the bearer token. The container
healthcheck mode reads `GOPROXY_ADMIN_TOKEN` automatically:

```sh
goproxy -healthcheck http://127.0.0.1:9090/readyz
```

## Rollout procedure

1. Apply config and image changes in staging.
2. Run a smoke test through the same ingress path used by production.
3. Deploy one batch.
4. Watch readiness, 5xx rate, retries, policy denials, and rate limits.
5. Continue only after metrics match the previous baseline.

## Rollback

Keep the previous image digest and config checksum available.

```sh
kubectl -n goproxy rollout undo deployment/goproxy
kubectl -n goproxy rollout status deployment/goproxy
```

After rollback, check `/readyz`, public request success, backend health gauges,
and error logs.
