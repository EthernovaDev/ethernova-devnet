#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-mainnet}" # mainnet | dev
ENDPOINT="${2:-http://127.0.0.1:8545}"

case "$MODE" in
  mainnet|dev) ;;
  *)
    echo "Usage: $0 <mainnet|dev> [http://127.0.0.1:8545]"
    exit 2
    ;;
esac

expected="0x1dab5" # 121525
if [[ "$MODE" == "dev" ]]; then
  expected="0x12fd2" # 77778
fi

payload='{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'

out="$(curl -sS -H "Content-Type: application/json" --data "$payload" -w "\n%{http_code}" "$ENDPOINT" || true)"
code="${out##*$'\n'}"
body="${out%$'\n'*}"

if [[ "$code" == "403" ]]; then
  echo "FAIL: HTTP 403 from RPC ($ENDPOINT)"
  echo "DIAG: geth/core-geth is blocking by --http.vhosts / Host header (\"invalid host specified\")."
  echo "Fix:"
  echo "  --http.vhosts=localhost,127.0.0.1,host.docker.internal"
  echo "Dev-only:"
  echo "  --http.vhosts=*"
  exit 2
fi

if [[ "$code" != "200" ]]; then
  echo "FAIL: HTTP $code from RPC ($ENDPOINT)"
  echo "Body: $body"
  exit 2
fi

PYTHON_BIN="${PYTHON_BIN:-}"
if [[ -z "$PYTHON_BIN" ]]; then
  if command -v python3 >/dev/null 2>&1; then PYTHON_BIN="python3"
  elif command -v python >/dev/null 2>&1; then PYTHON_BIN="python"
  else
    echo "ERROR: python3 (or python) is required to parse JSON-RPC response"
    exit 1
  fi
fi

got="$("$PYTHON_BIN" - <<PY
import json, sys
try:
    j = json.loads(sys.stdin.read())
    print(j.get("result",""))
except Exception:
    print("")
PY
<<<"$body")"

if [[ -z "$got" ]]; then
  echo "WARN: HTTP 200 but could not parse JSON-RPC result"
  echo "Body: $body"
  exit 0
fi

if [[ "${got,,}" != "${expected,,}" ]]; then
  echo "WARN: eth_chainId mismatch: got=$got expected=$expected"
  exit 0
fi

echo "OK: eth_chainId=$got ($MODE)"
