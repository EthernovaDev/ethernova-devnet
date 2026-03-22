#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXPECTED_GENESIS="0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"
EXPECTED_CHAINID_HEX="0x1dab5"

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

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required." >&2
  exit 1
fi

EMBEDDED_GENESIS="$ROOT_DIR/params/ethernova/genesis-121525-alloc.json"
if [[ -f "$EMBEDDED_GENESIS" ]]; then
  cp -f "$EMBEDDED_GENESIS" "$(dirname "$BIN")/genesis-121525-alloc.json" || true
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

DATADIR="$(mktemp -d -t ethernova-fork-XXXXXX)"
LOG="$DATADIR/ethernova.log"
LOG_OUT="$DATADIR/ethernova.out.log"
LOG_ERR="$DATADIR/ethernova.err.log"
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

echo "== Ethernova Fork Verify =="
echo "bin=$BIN"
echo "datadir=$DATADIR"
echo "endpoint=$ENDPOINT"

"$BIN" --datadir "$DATADIR" --networkid 121525 --http --http.addr 127.0.0.1 --http.port "$PORT" \
  --http.api eth,net,web3,debug --nodiscover --ipcdisable --log.file "$LOG" >"$LOG_OUT" 2>"$LOG_ERR" &
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
if [[ "$chain_hex_lc" != "$EXPECTED_CHAINID_HEX" ]]; then
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

trace_opcode() {
  local label="$1"
  local code="$2"
  local blockhex="$3"
  local expect_invalid="$4"
  local allow_fail="${5:-false}"

  local call="{\"from\":\"0x0000000000000000000000000000000000000000\",\"gas\":\"0x2dc6c0\",\"data\":\"${code}\"}"
  local config="{"
  if [[ -n "$blockhex" ]]; then
    config="${config}\"blockOverrides\":{\"number\":\"${blockhex}\"}}"
  else
    config="${config}}"
  fi

  local resp
  resp=$(rpc debug_traceCall "[${call},\"latest\",${config}]")
  if [[ "${expect_invalid}" == "true" ]]; then
    if ! echo "${resp}" | grep -qi "\"failed\":true" && ! echo "${resp}" | grep -qi "invalid opcode"; then
      if [[ "${allow_fail}" == "true" ]]; then
        echo "WARN: ${label} did not fail as expected; continuing with explicit block overrides."
        echo "${resp}" | head -n 1
        return 0
      fi
      echo "FAIL: ${label} expected invalid opcode" >&2
      echo "${resp}" >&2
      exit 1
    fi
  else
    if echo "${resp}" | grep -qi "\"failed\":true" || echo "${resp}" | grep -qi "\"error\""; then
      if [[ "${allow_fail}" == "true" ]]; then
        echo "WARN: ${label} failed unexpectedly; continuing with explicit block overrides."
        echo "${resp}" | head -n 1
        return 0
      fi
      echo "FAIL: ${label} expected success" >&2
      echo "${resp}" >&2
      exit 1
    fi
  fi
  echo "${label}: OK"
}

shl_code="0x600160021b00"
chainid_code="0x4600"
selfbalance_code="0x4700"

trace_opcode "latest (head=0) SHL" "$shl_code" "" "true" "true"
trace_opcode "pre-fork 104999 SHL" "$shl_code" "0x19a27" "true"
trace_opcode "pre-fork 104999 CHAINID" "$chainid_code" "0x19a27" "true"
trace_opcode "pre-fork 104999 SELFBALANCE" "$selfbalance_code" "0x19a27" "true"

trace_opcode "post-fork 105000 SHL" "$shl_code" "0x19a28" "false"
trace_opcode "post-fork 105000 CHAINID" "$chainid_code" "0x19a28" "false"
trace_opcode "post-fork 105000 SELFBALANCE" "$selfbalance_code" "0x19a28" "false"

echo "clientVersion=$client"
echo "chainId=$chain_hex"
echo "genesis=$genesis_hash"
echo "OK: fork verification passed."
