---
title: Deploy on systemd
section: Tasks
order: 120
summary: Run Gefahr on a VM or bare-metal host with a locked-down service unit and explicit config paths.
---

# Deploy on systemd

Use systemd when Gefahr runs directly on a VM or bare-metal host.

## Files

Recommended layout:

```text
/usr/local/bin/goproxy
/etc/goproxy/proxy.yaml
/etc/goproxy/goproxy.env
/etc/goproxy/tls/
/var/lib/goproxy/
```

Use `configs/proxy.vps.yaml` as the starting config for VPS deployments. It
binds public TLS on `:443`, keeps admin on `127.0.0.1:9090`, requires
`GOPROXY_ADMIN_TOKEN`, and leaves trusted proxy headers disabled until you
explicitly configure a load balancer or reverse proxy.

The environment file should contain secrets such as:

```sh
GOPROXY_ADMIN_TOKEN=replace-with-a-real-token
```

Use restrictive permissions:

```sh
sudo chown root:root /etc/goproxy/goproxy.env
sudo chmod 0600 /etc/goproxy/goproxy.env
```

Validate the config before copying it into place or reloading the service:

```sh
goproxy -config /etc/goproxy/proxy.yaml -check-config
```

Repository deployment assets can be checked with:

```sh
make deploy-check
```

## Service expectations

The service should:

- Run as the non-root `goproxy` user.
- Use `NoNewPrivileges=true`.
- Restrict writable paths.
- Mount private keys read-only.
- Restart on failure.
- Run `goproxy -check-config` before startup.
- Use `ExecReload` to send `SIGHUP`.

## Reload config

```sh
sudo systemctl reload goproxy
```

A failed reload leaves the previous snapshot active. Always inspect logs after
reload:

```sh
journalctl -u goproxy -n 100 --no-pager
```

## Restart-only changes

Restart for listener addresses, listener TLS mode, admin address, public server
timeouts, maximum header size, and connection limit changes:

```sh
sudo systemctl restart goproxy
```

## Host firewall

Expose only the public listener to clients. Restrict the admin listener to
loopback or a private management network, and keep bearer authentication
enabled. For the shipped VPS config, expose `443/tcp` publicly and do not expose
`9090/tcp`.
