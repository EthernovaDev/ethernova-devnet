#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR"
if [[ "$(basename "$ROOT_DIR")" == "scripts" ]]; then
  ROOT_DIR="$(cd "$ROOT_DIR/.." && pwd)"
fi

BIN_SRC="$ROOT_DIR/bin/ethernova"
if [[ ! -x "$BIN_SRC" ]]; then
  BIN_SRC="$ROOT_DIR/ethernova"
fi

if [[ ! -x "$BIN_SRC" ]]; then
  echo "ERROR: ethernova binary not found. Build first or use the release bundle." >&2
  exit 1
fi

SERVICE_SRC="$ROOT_DIR/systemd/ethernova.service"
if [[ ! -f "$SERVICE_SRC" ]]; then
  echo "ERROR: systemd/ethernova.service not found." >&2
  exit 1
fi

if [[ "$(id -u)" -ne 0 ]]; then
  echo "ERROR: run as root (sudo ./install.sh)" >&2
  exit 1
fi

if ! id ethernova >/dev/null 2>&1; then
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home /var/lib/ethernova --shell /usr/sbin/nologin ethernova
  else
    echo "WARNING: useradd not found; create a system user 'ethernova' manually." >&2
  fi
fi

mkdir -p /var/lib/ethernova
chown -R ethernova:ethernova /var/lib/ethernova || true

install -m 0755 "$BIN_SRC" /usr/local/bin/ethernova
install -m 0644 "$SERVICE_SRC" /etc/systemd/system/ethernova.service

mkdir -p /etc/ethernova
if [[ ! -f /etc/ethernova/ethernova.env ]]; then
  cat >/etc/ethernova/ethernova.env <<'EOF'
# Optional extra flags for the node.
# Example: ETHERNOVA_OPTS="--bootnodes enode://... --mine"
ETHERNOVA_OPTS=""
EOF
fi

systemctl daemon-reload
systemctl enable ethernova

echo "Installed /usr/local/bin/ethernova and systemd unit."
echo "Start with: systemctl start ethernova"
