#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ETHERNOVA="$ROOT_DIR/bin/ethernova"
if [[ ! -x "$ETHERNOVA" ]]; then
  ETHERNOVA="$ROOT_DIR/ethernova"
fi
if [[ ! -x "$ETHERNOVA" ]]; then
  echo "ethernova not found (expected bin/ethernova or root)." >&2
  exit 1
fi

DATA_DIR="${DATA_DIR:-$ROOT_DIR/data-mainnet}"
HTTP_PORT="${HTTP_PORT:-8545}"
WS_PORT="${WS_PORT:-8546}"
MINE="${MINE:-0}"
BOOTNODES="${BOOTNODES:-}"
BOOTNODES_FILE="${BOOTNODES_FILE:-}"
LOG_PATH="$ROOT_DIR/node.log"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bootnodes)
      BOOTNODES="${2:-}"
      shift 2
      ;;
    --bootnodes-file)
      BOOTNODES_FILE="${2:-}"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -z "$BOOTNODES" ]]; then
  if [[ -z "$BOOTNODES_FILE" ]]; then
    BOOTNODES_FILE="$ROOT_DIR/network/bootnodes.txt"
  fi
  if [[ -f "$BOOTNODES_FILE" ]]; then
    BOOTNODES="$(grep -vE '^[[:space:]]*(#|$)' "$BOOTNODES_FILE" | tr -d '\r' | paste -sd, - || true)"
  fi
fi

mkdir -p "$DATA_DIR"

GENESIS="$ROOT_DIR/genesis/genesis-mainnet.json"
if [[ ! -f "$GENESIS" ]]; then
  GENESIS="$ROOT_DIR/genesis-mainnet.json"
fi
if [[ ! -f "$GENESIS" ]]; then
  echo "genesis-mainnet.json not found." >&2
  exit 1
fi

if [[ ! -d "$DATA_DIR/geth/chaindata" ]]; then
  echo "Initializing datadir (idempotent init, no wipe)..."
  "$ETHERNOVA" --datadir "$DATA_DIR" init "$GENESIS" >/dev/null
fi

CONFIG_PATH=""
STATIC_NODES_SRC="$ROOT_DIR/network/static-nodes.json"
DEPRECATED_STATIC="$DATA_DIR/geth/static-nodes.json"
if [[ -f "$DEPRECATED_STATIC" && -f "$STATIC_NODES_SRC" ]]; then
  if cmp -s "$DEPRECATED_STATIC" "$STATIC_NODES_SRC"; then
    mv "$DEPRECATED_STATIC" "$DATA_DIR/geth/static-nodes.deprecated.json"
    echo "Moved deprecated static-nodes.json to $DATA_DIR/geth/static-nodes.deprecated.json"
  else
    echo "Warning: deprecated static-nodes.json exists at $DEPRECATED_STATIC (client ignores it)." >&2
  fi
fi

if [[ -f "$STATIC_NODES_SRC" ]]; then
  mapfile -t static_nodes < <(grep -oE 'enode://[0-9a-fA-F]+@[^"]+' "$STATIC_NODES_SRC" || true)
  if [[ ${#static_nodes[@]} -gt 0 ]]; then
    CONFIG_PATH="$DATA_DIR/config.mainnet.toml"
    joined=""
    for node in "${static_nodes[@]}"; do
      if [[ -n "$joined" ]]; then
        joined+=","
      fi
      joined+="\"$node\""
    done
    printf "[Node.P2P]\nStaticNodes = [%s]\n" "$joined" > "$CONFIG_PATH"
    echo "Static nodes (config): $CONFIG_PATH"
  else
    echo "Warning: static-nodes.json has placeholders or invalid enodes; skipping config." >&2
  fi
fi

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn | awk '{print $4}' | grep -Eq "[:.]${port}$"
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "[:.]${port}$"
    return $?
  fi
  return 1
}

if port_in_use "$HTTP_PORT"; then
  echo "ERROR: Port $HTTP_PORT is already in use. Stop the process using it or set HTTP_PORT." >&2
  exit 1
fi
if port_in_use "$WS_PORT"; then
  echo "ERROR: Port $WS_PORT is already in use. Stop the process using it or set WS_PORT." >&2
  exit 1
fi

ARGS=(
  --datadir "$DATA_DIR"
  --networkid 121525
  --http --http.addr 127.0.0.1 --http.port "$HTTP_PORT" --http.api eth,net,web3,debug
  --ws --ws.addr 127.0.0.1 --ws.port "$WS_PORT" --ws.api eth,net,web3,debug
)

if [[ -n "$BOOTNODES" ]]; then
  echo "Bootnodes: $BOOTNODES"
  ARGS+=(--bootnodes "$BOOTNODES")
else
  echo "Bootnodes: (none)"
fi
if [[ -n "$CONFIG_PATH" ]]; then
  ARGS+=(--config "$CONFIG_PATH")
fi

if [[ "$MINE" == "1" ]]; then
  ARGS+=(--mine)
fi

: > "$LOG_PATH"
echo "Logs: $LOG_PATH"

show_tail() {
  if [[ -f "$LOG_PATH" ]]; then
    echo ""
    echo "Last 50 lines from $LOG_PATH:"
    tail -n 50 "$LOG_PATH"
  fi
}
trap show_tail EXIT

"$ETHERNOVA" "${ARGS[@]}" 2>&1 | tee -a "$LOG_PATH"
