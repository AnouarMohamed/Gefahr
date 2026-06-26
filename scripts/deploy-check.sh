#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

check_config() {
  local path="$1"
  echo "validating config: $path"
  go run ./cmd/goproxy -config "$path" -check-config >/dev/null
}

check_systemd_unit() {
  local root="$TMPDIR/systemd-root"
  local executable
  executable="$(command -v env)"
  mkdir -p "$root/usr/local/bin" "$root/bin" "$root/etc/goproxy" "$root/etc/systemd/system" "$root/usr/lib/systemd/system"
  install -m 0755 "$executable" "$root/usr/local/bin/goproxy"
  install -m 0755 "$executable" "$root/bin/kill"
  install -m 0644 deploy/systemd/goproxy.service "$root/etc/systemd/system/goproxy.service"
  for target in sysinit.target basic.target multi-user.target network.target network-online.target; do
    printf '[Unit]\nDescription=%s\n' "$target" >"$root/usr/lib/systemd/system/$target"
  done
  : >"$root/etc/goproxy/proxy.yaml"
  : >"$root/etc/goproxy/goproxy.env"
  systemd-analyze verify --root="$root" goproxy.service
}

extract_kubernetes_config() {
  local manifest="$1"
  local output="$2"
  awk '
    /^  proxy.yaml: \|$/ {
      in_config = 1
      next
    }
    in_config && /^---$/ {
      exit
    }
    in_config && /^  [[:alnum:]_.-]+:/ {
      exit
    }
    in_config {
      if ($0 ~ /^    /) {
        sub(/^    /, "")
      }
      print
    }
  ' "$manifest" >"$output"
  if ! [ -s "$output" ]; then
    echo "failed to extract proxy.yaml from $manifest" >&2
    exit 1
  fi
}

check_config configs/proxy.example.yaml
check_config configs/proxy.compose.yaml

extract_kubernetes_config deploy/kubernetes/goproxy.yaml "$TMPDIR/kubernetes-proxy.yaml"
check_config "$TMPDIR/kubernetes-proxy.yaml"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  echo "validating compose model"
  docker compose config >/dev/null
else
  echo "skipping compose model validation: docker compose is unavailable"
fi

if command -v kubectl >/dev/null 2>&1 && kubectl config current-context >/dev/null 2>&1; then
  echo "validating kubernetes manifest"
  kubectl apply --dry-run=client --validate=false -f deploy/kubernetes/goproxy.yaml >/dev/null
else
  echo "skipping kubernetes manifest validation: kubectl context is unavailable"
fi

if command -v systemd-analyze >/dev/null 2>&1; then
  echo "validating systemd unit"
  check_systemd_unit
else
  echo "skipping systemd unit validation: systemd-analyze is unavailable"
fi

echo "deployment checks passed"
