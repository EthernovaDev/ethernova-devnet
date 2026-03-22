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

GENESIS="$ROOT_DIR/genesis-${MODE}.json"
if [[ "$MODE" == "mainnet" ]]; then
  GENESIS="$ROOT_DIR/genesis-mainnet.json"
fi

if [[ ! -f "$GENESIS" ]]; then
  echo "ERROR: genesis not found: $GENESIS"
  exit 1
fi

PYTHON_BIN="${PYTHON_BIN:-}"
if [[ -z "$PYTHON_BIN" ]]; then
  if command -v python3 >/dev/null 2>&1; then PYTHON_BIN="python3"
  elif command -v python >/dev/null 2>&1; then PYTHON_BIN="python"
  else
    echo "ERROR: python3 (or python) is required to read genesis chainId"
    exit 1
  fi
fi

chain_id="$("$PYTHON_BIN" - "$GENESIS" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    g = json.load(f)
print(int(g["config"]["chainId"]))
PY
)"

if [[ "$MODE" == "mainnet" && "$chain_id" != "121525" ]]; then
  echo "ERROR: mainnet mode expects chainId 121525, got $chain_id (wrong genesis?)"
  exit 1
fi

if [[ "$MODE" == "dev" && "$chain_id" != "77778" ]]; then
  echo "ERROR: dev mode expects chainId 77778, got $chain_id (wrong genesis?)"
  exit 1
fi

DATADIR="${DATADIR:-$ROOT_DIR/data-${MODE}}"

if [[ ! -d "$DATADIR/geth/chaindata" ]]; then
  mkdir -p "$DATADIR"
  echo "Init genesis ($MODE) -> $DATADIR"
  "$BIN" --datadir "$DATADIR" init "$GENESIS"
else
  echo "Datadir already initialized: $DATADIR"
fi

static_src="$ROOT_DIR/networks/${MODE}/static-nodes.json"
static_dst="$DATADIR/geth/static-nodes.json"
if [[ -f "$static_src" ]]; then
  mkdir -p "$(dirname "$static_dst")"
  cp "$static_src" "$static_dst"
  echo "Placed static-nodes.json -> $static_dst"
fi

echo "Done."
echo "Mode:    $MODE"
echo "chainId: $chain_id"
echo "Datadir: $DATADIR"
echo "Next:    $SCRIPT_DIR/ethernova-run.sh $MODE"
