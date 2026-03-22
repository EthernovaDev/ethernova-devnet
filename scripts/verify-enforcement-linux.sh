#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXPECTED_GENESIS="0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"
EXPECTED_LOG="Ethernova fork enforcement block=105,000"

BIN="${ETHE_BIN:-$ROOT_DIR/dist/ethernova-v1.2.8-linux-amd64}"
GENESIS="${GENESIS_PATH:-$ROOT_DIR/genesis-mainnet.json}"
PORT="${HTTP_PORT:-8545}"

if [[ ! -x "$BIN" ]]; then
  if [[ -x "$ROOT_DIR/dist/ethernova" ]]; then
    BIN="$ROOT_DIR/dist/ethernova"
  elif [[ -x "$ROOT_DIR/ethernova" ]]; then
    BIN="$ROOT_DIR/ethernova"
  else
    echo "Ethernova binary not found. Set ETHE_BIN or build dist/ethernova-v1.2.8-linux-amd64." >&2
    exit 1
  fi
fi

if [[ ! -f "$GENESIS" ]]; then
  echo "Genesis not found: $GENESIS" >&2
  exit 1
fi

EMBEDDED_GENESIS="$ROOT_DIR/params/ethernova/genesis-121525-alloc.json"
if [[ -f "$EMBEDDED_GENESIS" ]]; then
  cp -f "$EMBEDDED_GENESIS" "$(dirname "$BIN")/genesis-121525-alloc.json" || true
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required." >&2
  exit 1
fi

port_in_use() {
  local p="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn | awk '{print $4}' | grep -qE ":${p}$"
    return $?
  elif command -v netstat >/dev/null 2>&1; then
    netstat -ltn 2>/dev/null | awk '{print $4}' | grep -qE ":${p}$"
    return $?
  fi
  return 1
}

for try in $(seq 0 19); do
  if ! port_in_use "$PORT"; then
    break
  fi
  PORT=$((PORT + 1))
done

DATADIR="$(mktemp -d -t ethernova-enforce-XXXXXX)"
LOG="$DATADIR/ethernova.log"
ENDPOINT="http://127.0.0.1:${PORT}"

cleanup() {
  if [[ -n "${PID:-}" ]] && kill -0 "$PID" >/dev/null 2>&1; then
    kill "$PID" >/dev/null 2>&1 || true
    sleep 0.3
    kill -9 "$PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"$BIN" --datadir "$DATADIR" init "$GENESIS" >/dev/null

echo "== Ethernova Enforcement Verify =="
echo "bin=$BIN"
echo "datadir=$DATADIR"
echo "endpoint=$ENDPOINT"

"$BIN" --datadir "$DATADIR" --networkid 121525 --http --http.addr 127.0.0.1 --http.port "$PORT" \
  --http.api eth,net,web3,debug --nodiscover >"$LOG" 2>&1 &
PID=$!

rpc() {
  local method="$1"
  local params="$2"
  curl -s -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}" \
    "$ENDPOINT"
}

client=""
for _ in $(seq 1 60); do
  client="$(rpc web3_clientVersion "[]")"
  if echo "$client" | grep -q "\"result\""; then
    break
  fi
  sleep 0.5
done
if ! echo "$client" | grep -q "\"result\""; then
  echo "RPC did not respond on $ENDPOINT" >&2
  exit 1
fi

chain_json="$(rpc eth_chainId "[]")"
chain_hex="$(echo "$chain_json" | sed -n 's/.*"result":"\([^"]*\)".*/\1/p')"
if [[ -z "$chain_hex" ]]; then
  echo "Failed to read chainId" >&2
  exit 1
fi
chain_hex_lc="$(echo "$chain_hex" | tr 'A-F' 'a-f')"
chain_dec=$((16#${chain_hex_lc#0x}))
if [[ "$chain_dec" -ne 121525 ]]; then
  echo "Unexpected chainId: $chain_hex" >&2
  exit 1
fi

block0="$(rpc eth_getBlockByNumber "[\"0x0\",false]")"
genesis_hash="$(echo "$block0" | sed -n 's/.*"hash":"\(0x[0-9a-fA-F]*\)".*/\1/p' | head -n1)"
if [[ -z "$genesis_hash" ]]; then
  echo "Failed to read genesis hash" >&2
  exit 1
fi
genesis_lc="$(echo "$genesis_hash" | tr 'A-F' 'a-f')"
if [[ "$genesis_lc" != "$EXPECTED_GENESIS" ]]; then
  echo "Unexpected genesis hash: $genesis_hash" >&2
  exit 1
fi

found="false"
for _ in $(seq 1 40); do
  if grep -Fq "$EXPECTED_LOG" "$LOG"; then
    found="true"
    break
  fi
  sleep 0.25
done
if [[ "$found" != "true" ]]; then
  echo "Log line not found: $EXPECTED_LOG" >&2
  exit 1
fi

echo "clientVersion=$client"
echo "chainId=$chain_hex"
echo "genesis=$genesis_hash"
echo "log=$EXPECTED_LOG"
echo "OK: enforcement verification passed."
