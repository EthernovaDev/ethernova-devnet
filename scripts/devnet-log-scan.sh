#!/usr/bin/env bash
set -euo pipefail

# Scans active devnet node logs for current critical errors only.
# Required for local VM nodes: DEVNET_SSH_KEY, defaulting to the usual local key path.
DEVNET_SSH_KEY=${DEVNET_SSH_KEY:-/Users/pilotair/Documents/Dev/SSH Keys/devnet}
VPS_SSH_KEY=${VPS_SSH_KEY:-$HOME/.ssh/rpcandexplorer}
LOCAL_NODES=${LOCAL_NODES:-$'node1 novanode1@192.168.1.15\nnode2 novanode2@192.168.1.34\nnode3 novanode3@192.168.1.134\nnode4 novanode4@192.168.1.16'}
VPS_HOST=${VPS_HOST:-root@207.180.230.125}
PATTERN='BAD BLOCK|Fatal|panic|Failed to insert|invalid merkle|state root'

scan_local() {
  local name=$1 host=$2
  printf '\n===== %s =====\n' "$name"
  ssh -n -i "$DEVNET_SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=8 "$host" \
    "for f in \"\$HOME/ethernova-devnet/logs/ethernova.log\" \"\$HOME/logs/ethernova.log\"; do if [ -f \"\$f\" ]; then echo LOG=\$f; wc -c \"\$f\"; grep -Ei '$PATTERN' \"\$f\" || true; fi; done"
}

while read -r name host; do
  [ -z "${name:-}" ] && continue
  scan_local "$name" "$host"
done <<< "$LOCAL_NODES"

printf '\n===== vps =====\n'
ssh -n -i "$VPS_SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=8 "$VPS_HOST" \
  "f=/opt/ethernova-devnet/logs/devnet.log; echo LOG=\$f; wc -c \"\$f\"; grep -Ei '$PATTERN' \"\$f\" || true"
