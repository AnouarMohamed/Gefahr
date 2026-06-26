# Disaster recovery drills

Gefahr is stateless at runtime: configuration, TLS material, admin tokens, and
deployment manifests are the durable assets. Response cache contents and
in-memory rate-limit buckets are intentionally disposable.

## Recovery Objectives

Set explicit targets for each environment before production cutover:

| Asset | RPO target | RTO target | Source of truth |
|---|---:|---:|---|
| Release binary/image | 0 releases | 15 minutes | GitHub release, GHCR image, attestations |
| Proxy config | 1 committed or approved config change | 15 minutes | Git, ConfigMap, `/etc/goproxy/proxy.yaml` backup |
| Admin token | Last rotation | 30 minutes | Secret manager, Kubernetes Secret, `/etc/goproxy/goproxy.env` |
| TLS certificates and upstream CA/client certs | Last issued certificate | Certificate rotation SLA | Secret manager, Kubernetes Secret, host secret store |
| Metrics/log history | Monitoring retention policy | Monitoring platform SLA | Prometheus/log backend |

Do not treat local cache contents as recoverable data. A restart or failover may
reduce cache hit rate temporarily, but it must not affect correctness.

## Required Backups

Keep these recoverable outside the failed host or cluster:

- Exact release tag, image digest, and binary checksum.
- Last known-good proxy config.
- Admin token secret name and rotation procedure.
- Public TLS certificate/key references.
- Upstream CA bundle and client certificate/key references, when enabled.
- Kubernetes manifests or systemd unit files used for the running deployment.
- Load balancer listener/routing policy and trusted proxy CIDR settings.

## Drill 1: Bad Config Rollback

Purpose: prove a bad config cannot replace the active runtime and that a known
good config can be restored quickly.

1. Record the current release tag, config checksum, and `/readyz` result.
2. Apply a deliberately invalid config in staging.
3. Send `SIGHUP` for systemd or roll the pod/config mechanism used in your
   environment.
4. Confirm the reload is rejected and existing traffic still succeeds.
5. Restore the known-good config.
6. Confirm `/readyz`, request success rate, retry rate, and 5xx rate return to
   baseline.

Evidence to capture:

```text
date:
environment:
operator:
previous release:
previous config checksum:
invalid config change:
reload rejection log:
readyz before/after:
request success evidence:
rollback duration:
follow-up items:
```

## Drill 2: Binary or Image Rollback

Purpose: prove a bad release can be replaced by the previous known-good release.

Kubernetes:

```sh
kubectl -n goproxy rollout history deployment/goproxy
kubectl -n goproxy set image deployment/goproxy goproxy=ghcr.io/anrorg/gefahr:<previous-version>
kubectl -n goproxy rollout status deployment/goproxy
```

systemd:

```sh
sudo install -o root -g root -m 0755 goproxy.previous /usr/local/bin/goproxy
sudo systemctl restart goproxy
sudo systemctl status goproxy
```

Evidence to capture:

```text
date:
environment:
operator:
bad release:
rollback release:
artifact verification command:
rollout/restart command:
readyz before/after:
5xx/retry-rate before/after:
customer impact:
rollback duration:
follow-up items:
```

## Drill 3: Secret or Certificate Recovery

Purpose: prove admin auth and TLS material can be restored or rotated after
loss, expiry, or suspected compromise.

1. Rotate `GOPROXY_ADMIN_TOKEN` in staging.
2. Confirm `/readyz` and `/metrics` reject the old token and accept the new
   token.
3. Rotate a public TLS certificate or upstream CA/client certificate reference
   using the same deployment path production uses.
4. Confirm public handshakes or upstream HTTPS health checks succeed.
5. Confirm admin audit logs show expected authorized and unauthorized attempts
   without logging token values.

Evidence to capture:

```text
date:
environment:
operator:
secret/certificate rotated:
old token rejected:
new token accepted:
TLS validation command:
admin audit log evidence:
rotation duration:
follow-up items:
```

## Drill 4: Region or Provider Failure

Purpose: prove traffic can move away from a failed cluster, VM group, region, or
load balancer.

1. Identify the traffic steering control: DNS, global load balancer, ingress
   controller, or manual route change.
2. Disable one staging region/provider path or remove it from the load balancer
   backend set.
3. Confirm traffic drains to the surviving path.
4. Confirm trusted proxy CIDRs still match the active ingress path.
5. Restore the failed path and confirm traffic can return without config drift.

Evidence to capture:

```text
date:
environment:
operator:
failed component:
traffic steering control:
drain command/change:
readyz by location:
request success by location:
trusted proxy CIDR validation:
restore command/change:
total failover duration:
follow-up items:
```

## Completion Criteria

A production environment is not disaster-recovery ready until:

- Every drill has been run in staging.
- Evidence is stored with the release or operations record.
- RTO/RPO targets are documented and accepted by the owner.
- The rollback owner and communication path are clear.
- Any manual step has an explicit command or console path.
