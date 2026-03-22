#!/usr/bin/env bash
set -euo pipefail

KEEP_RUNNING=0
if [[ "${1:-}" == "--keep-running" ]]; then
  KEEP_RUNNING=1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

log_cmd() {
  echo "Running: $*"
}

BIN_DIR="$ROOT_DIR/bin"
ETHERNOVA="$BIN_DIR/ethernova"
EVMCHECK="$BIN_DIR/evmcheck"

if [[ ! -x "$ETHERNOVA" || ! -x "$EVMCHECK" ]]; then
  if [[ -x "$ROOT_DIR/ethernova" && -x "$ROOT_DIR/evmcheck" ]]; then
    ETHERNOVA="$ROOT_DIR/ethernova"
    EVMCHECK="$ROOT_DIR/evmcheck"
  else
    if ! command -v go >/dev/null 2>&1; then
      echo "go not found in PATH. Install Go or use prebuilt binaries in bin/." >&2
      exit 1
    fi
    mkdir -p "$BIN_DIR"
    log_cmd "CGO_ENABLED=0 go build -o bin/ethernova ./cmd/geth"
    (cd "$ROOT_DIR" && CGO_ENABLED=0 go build -o bin/ethernova ./cmd/geth)
    log_cmd "CGO_ENABLED=0 go build -o bin/evmcheck ./cmd/evmcheck"
    (cd "$ROOT_DIR" && CGO_ENABLED=0 go build -o bin/evmcheck ./cmd/evmcheck)
    ETHERNOVA="$BIN_DIR/ethernova"
    EVMCHECK="$BIN_DIR/evmcheck"
  fi
fi

if [[ ! -x "$ETHERNOVA" ]]; then
  echo "ethernova not found (expected bin/ethernova or root)." >&2
  exit 1
fi
if [[ ! -x "$EVMCHECK" ]]; then
  echo "evmcheck not found (expected bin/evmcheck or root)." >&2
  exit 1
fi

GENESIS="$ROOT_DIR/genesis/genesis-devnet-fork20.json"
if [[ ! -f "$GENESIS" ]]; then
  GENESIS="$ROOT_DIR/genesis-devnet-fork20.json"
fi
if [[ ! -f "$GENESIS" ]]; then
  echo "genesis-devnet-fork20.json not found." >&2
  exit 1
fi

KEY_FILE="$ROOT_DIR/genesis/devnet-testkey.txt"
if [[ ! -f "$KEY_FILE" ]]; then
  KEY_FILE="$ROOT_DIR/devnet-testkey.txt"
fi
if [[ ! -f "$KEY_FILE" ]]; then
  echo "devnet-testkey.txt not found." >&2
  exit 1
fi

PRIV_KEY="$(grep -E '^PRIVATE_KEY=' "$KEY_FILE" | cut -d= -f2-)"
DEV_ADDR="$(grep -E '^ADDRESS=' "$KEY_FILE" | cut -d= -f2-)"
CHAINID="$(grep -E '^CHAINID=' "$KEY_FILE" | cut -d= -f2-)"

if [[ -z "$PRIV_KEY" || -z "$DEV_ADDR" || -z "$CHAINID" ]]; then
  echo "devnet-testkey.txt missing PRIVATE_KEY, ADDRESS, or CHAINID." >&2
  exit 1
fi

DATA_DIR="$ROOT_DIR/data-devnet"
RPC_URL="http://127.0.0.1:8545"
FORK_BLOCK=20
TARGET_BLOCK=$((FORK_BLOCK+2))
LOG_DIR="$ROOT_DIR/logs"
LOG_PATH="$LOG_DIR/devnet-test.log"

mkdir -p "$LOG_DIR"

mkdir -p "$DATA_DIR"
if [[ ! -d "$DATA_DIR/geth" ]]; then
  echo "Initializing devnet datadir..."
  log_cmd "$ETHERNOVA --datadir $DATA_DIR init $GENESIS"
  "$ETHERNOVA" --datadir "$DATA_DIR" init "$GENESIS" >/dev/null
fi

echo "Starting devnet node..."
log_cmd "$ETHERNOVA --datadir $DATA_DIR --http --http.addr 127.0.0.1 --http.port 8545 --http.api eth,net,web3,debug --ws --ws.addr 127.0.0.1 --ws.port 8546 --ws.api eth,net,web3,debug --nodiscover --maxpeers 0 --networkid $CHAINID --mine --fakepow --miner.threads 1 --miner.etherbase $DEV_ADDR"
"$ETHERNOVA" \
  --datadir "$DATA_DIR" \
  --http --http.addr 127.0.0.1 --http.port 8545 --http.api eth,net,web3,debug \
  --ws --ws.addr 127.0.0.1 --ws.port 8546 --ws.api eth,net,web3,debug \
  --nodiscover --maxpeers 0 \
  --networkid "$CHAINID" \
  --mine --fakepow --miner.threads 1 --miner.etherbase "$DEV_ADDR" \
  >"$LOG_PATH" 2>&1 &

NODE_PID=$!

cleanup() {
  if [[ "$KEEP_RUNNING" -eq 0 ]]; then
    echo "Stopping devnet node..."
    kill "$NODE_PID" >/dev/null 2>&1 || true
  else
    echo "Devnet left running. Logs: $LOG_PATH"
  fi
}
trap cleanup EXIT

get_block_number() {
  local resp hex
  resp="$(curl -s -X POST "$RPC_URL" -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}')"
  hex="$(echo "$resp" | sed -n 's/.*"result":"\(0x[0-9a-fA-F]*\)".*/\1/p')"
  if [[ -n "$hex" ]]; then
    echo $((16#${hex#0x}))
  else
    echo ""
  fi
}

echo "Waiting for RPC..."
deadline=$((SECONDS+180))
bn=""
while [[ $SECONDS -lt $deadline ]]; do
  bn="$(get_block_number || true)"
  if [[ -n "$bn" ]]; then
    break
  fi
  sleep 1
done

if [[ -z "$bn" ]]; then
  echo "RPC did not become ready in time. Check $LOG_PATH." >&2
  exit 1
fi

echo "Mining until block >= $TARGET_BLOCK..."
while true; do
  sleep 1
  bn="$(get_block_number || true)"
  if [[ -n "$bn" && "$bn" -ge "$TARGET_BLOCK" ]]; then
    break
  fi
done

echo "Running evmcheck..."
set +e
log_cmd "$EVMCHECK --rpc $RPC_URL --pk [redacted] --chainid $CHAINID --forkblock $FORK_BLOCK"
"$EVMCHECK" --rpc "$RPC_URL" --pk "$PRIV_KEY" --chainid "$CHAINID" --forkblock "$FORK_BLOCK"
EVMCHECK_EXIT=$?
set -e

exit "$EVMCHECK_EXIT"
