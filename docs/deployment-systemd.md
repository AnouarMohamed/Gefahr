# systemd deployment

The unit in [`deploy/systemd/goproxy.service`](../deploy/systemd/goproxy.service)
is a hardened VM/bare-metal baseline for running Gefahr without Kubernetes. It
assumes:

- Binary: `/usr/local/bin/goproxy`
- Config: `/etc/goproxy/proxy.yaml`
- Secret env file: `/etc/goproxy/goproxy.env`
- TLS files: `/etc/goproxy/tls/fullchain.pem` and
  `/etc/goproxy/tls/privkey.pem`, unless you replace the listener model
- Runtime user/group: `goproxy:goproxy`

Install a release archive and create the runtime identity:

```sh
sudo useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin goproxy
sudo install -o root -g root -m 0755 goproxy /usr/local/bin/goproxy
sudo install -d -o root -g goproxy -m 0750 /etc/goproxy
sudo install -d -o root -g goproxy -m 0750 /etc/goproxy/tls
sudo install -o root -g goproxy -m 0640 configs/proxy.vps.yaml /etc/goproxy/proxy.yaml
sudo install -o root -g root -m 0644 deploy/systemd/goproxy.service /etc/systemd/system/goproxy.service
```

Create `/etc/goproxy/goproxy.env` with a long random token. The VPS config
already references it with `admin.auth_token_env: GOPROXY_ADMIN_TOKEN`:

```sh
sudo install -o root -g root -m 0600 deploy/systemd/goproxy.env.example /etc/goproxy/goproxy.env
sudoedit /etc/goproxy/goproxy.env
sudoedit /etc/goproxy/proxy.yaml
```

Install TLS material referenced by `configs/proxy.vps.yaml` before starting the
service:

```sh
sudo install -o root -g goproxy -m 0640 fullchain.pem /etc/goproxy/tls/fullchain.pem
sudo install -o root -g goproxy -m 0640 privkey.pem /etc/goproxy/tls/privkey.pem
```

For direct VPS traffic, leave `client_ip` unset so rate limiting and forwarding
headers use the socket peer. If a trusted load balancer or reverse proxy sits in
front of Gefahr, set both `client_ip.trusted_proxies` and `client_ip.headers`;
Gefahr rejects a trusted-proxy policy without an explicit header order.

Validate before starting or reloading:

```sh
sudo /usr/local/bin/goproxy -config /etc/goproxy/proxy.yaml -check-config
make deploy-check
```

Start and inspect:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now goproxy
sudo systemctl status goproxy
sudo journalctl -u goproxy -f
```

Check the private admin endpoints with the bearer token:

```sh
TOKEN="$(sudo sed -n 's/^GOPROXY_ADMIN_TOKEN=//p' /etc/goproxy/goproxy.env)"
curl -fsS -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9090/readyz
curl -fsS -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9090/metrics
```

Reload validates and stages a complete replacement config before publishing it:

```sh
sudo systemctl reload goproxy
```

Rollback is file based. Restore the previous binary or config, then either
reload for config-only changes or restart for restart-only settings:

```sh
sudo cp /etc/goproxy/proxy.yaml.previous /etc/goproxy/proxy.yaml
sudo systemctl reload goproxy
sudo install -o root -g root -m 0755 goproxy.previous /usr/local/bin/goproxy
sudo systemctl restart goproxy
```

The service uses `ProtectSystem=strict`, drops write access to the host
filesystem, restricts address families, denies new privileges, and limits the
process to `CAP_NET_BIND_SERVICE` so non-root Gefahr can bind ports below 1024.
If you only bind high ports, remove both capability lines from the unit.

Operational notes:

- Keep the admin listener on loopback or a private management interface and
  keep bearer authentication enabled. Gefahr rejects unauthenticated
  non-loopback admin listeners.
- Store TLS keys under `/etc/goproxy` with group-readable permissions only when
  the `goproxy` group must read them.
- Set `TimeoutStopSec` higher than `timeouts.shutdown`.
- Use a host firewall or cloud security group to expose only intended public
  listener ports. For the shipped VPS config, expose `443/tcp` publicly and do
  not expose `9090/tcp`.
- Treat changes to listener addresses, TLS mode, admin address/auth fields,
  public server timeouts, shutdown timeout, maximum header size, and connection
  limits as restart changes. Other config changes can use `systemctl reload`.
