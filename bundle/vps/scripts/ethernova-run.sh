#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODE="${1:-mainnet}" # mainnet | dev

case "$MODE" in
  mainnet|dev) ;;
  *)
    echo "Usage: $0 <mainnet|dev>"
    exit 2
    ;;
esac

BIN="$ROOT_DIR/ethernova"
if [[ ! -f "$BIN" ]]; then
  echo "ERROR: binary not found: $BIN"
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  chmod +x "$BIN" 2>/dev/null || true
fi

# Ensure initialized (non-destructive).
"$SCRIPT_DIR/ethernova-init.sh" "$MODE"

PYTHON_BIN="${PYTHON_BIN:-}"
if [[ -z "$PYTHON_BIN" ]]; then
  if command -v python3 >/dev/null 2>&1; then PYTHON_BIN="python3"
  elif command -v python >/dev/null 2>&1; then PYTHON_BIN="python"
  else
    echo "ERROR: python3 (or python) is required to read genesis networkId"
    exit 1
  fi
fi

GENESIS="$ROOT_DIR/genesis-${MODE}.json"
if [[ "$MODE" == "mainnet" ]]; then
  GENESIS="$ROOT_DIR/genesis-mainnet.json"
fi

network_id="$("$PYTHON_BIN" - "$GENESIS" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    g = json.load(f)
cfg = g["config"]
chain_id = int(cfg["chainId"])
network_id = int(cfg.get("networkId", chain_id))
print(network_id)
PY
)"

if [[ "$MODE" == "mainnet" ]]; then
  network_id="121525"
fi

DATADIR="${DATADIR:-$ROOT_DIR/data-${MODE}}"

HTTP_ADDR="${HTTP_ADDR:-127.0.0.1}"
HTTP_PORT="${HTTP_PORT:-8545}"
WS_ADDR="${WS_ADDR:-127.0.0.1}"
WS_PORT="${WS_PORT:-8546}"
P2P_PORT="${P2P_PORT:-30303}"

VHOSTS_DEFAULT="localhost,127.0.0.1,host.docker.internal"
HTTP_VHOSTS="${HTTP_VHOSTS:-$VHOSTS_DEFAULT}"

VERBOSITY="${VERBOSITY:-3}"

APIS_MAINNET="${APIS_MAINNET:-eth,net,web3}"
APIS_DEV="${APIS_DEV:-eth,net,web3,personal,miner,txpool,admin,debug}"
APIS="$APIS_MAINNET"
if [[ "$MODE" == "dev" ]]; then
  APIS="$APIS_DEV"
fi

bootnodes_file="$ROOT_DIR/networks/${MODE}/bootnodes.txt"
bootnodes=""
if [[ -f "$bootnodes_file" ]]; then
  bootnodes="$(grep -vE '^[[:space:]]*(#|$)' "$bootnodes_file" | paste -sd, - || true)"
fi

args=(
  "--datadir" "$DATADIR"
  "--networkid" "$network_id"
  "--port" "$P2P_PORT"
  "--authrpc.addr" "127.0.0.1"
  "--authrpc.port" "8551"
  "--http"
  "--http.addr" "$HTTP_ADDR"
  "--http.port" "$HTTP_PORT"
  "--http.vhosts" "$HTTP_VHOSTS"
  "--http.api" "$APIS"
  "--ws"
  "--ws.addr" "$WS_ADDR"
  "--ws.port" "$WS_PORT"
  "--ws.api" "$APIS"
  "--verbosity" "$VERBOSITY"
)

if [[ -n "$bootnodes" ]]; then
  args+=( "--bootnodes" "$bootnodes" )
fi

MINE="${MINE:-false}"
if [[ "$MINE" == "true" || "$MINE" == "1" ]]; then
  if [[ -z "${ETHERBASE:-}" ]]; then
    echo "ERROR: MINE=true requires ETHERBASE=0x... (miner address)"
    exit 1
  fi

  args+=( "--mine" "--miner.threads" "${MINER_THREADS:-1}" "--miner.etherbase" "$ETHERBASE" )

  if [[ "$MODE" == "dev" ]]; then
    args+=( "--allow-insecure-unlock" "--miner.gasprice" "0" "--txpool.pricelimit" "0" "--txpool.pricebump" "0" )
  else
    args+=( "--miner.gasprice" "${MINER_GASPRICE:-1000000000}" )
  fi
fi

if [[ -n "${EXTRA_ARGS:-}" ]]; then
  # shellcheck disable=SC2206
  extra=( $EXTRA_ARGS )
  args+=( "${extra[@]}" )
fi

echo "Starting ethernova ($MODE)"
echo "Datadir:  $DATADIR"
echo "Network:  $network_id"
echo "HTTP RPC: http://$HTTP_ADDR:$HTTP_PORT"
echo "WS RPC:   ws://$WS_ADDR:$WS_PORT"
echo "P2P:      $P2P_PORT (tcp/udp)"
echo

exec "$BIN" "${args[@]}"
