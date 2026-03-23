#!/bin/bash
# Ethernova Devnet Security Audit
# Tests edge cases in adaptive gas, fast mode, and call cache

NODE="${1:-http://192.168.1.34:8552}"
echo "=== Ethernova Security Audit ==="
echo "Node: $NODE"
echo ""

PASS=0
FAIL=0

check() {
  local name=$1 expected=$2 actual=$3
  if [ "$expected" = "$actual" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] $name (expected=$expected actual=$actual)"
    FAIL=$((FAIL+1))
  fi
}

# 1. Adaptive gas discount cannot exceed 50%
echo "--- Adaptive Gas Bounds ---"
R=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetDiscount","params":[99],"id":1}' \
  $NODE | grep -o '"result":[0-9]*' | cut -d: -f2)
check "Discount capped at 50" "50" "$R"

R=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetPenalty","params":[99],"id":1}' \
  $NODE | grep -o '"result":[0-9]*' | cut -d: -f2)
check "Penalty capped at 50" "50" "$R"

# Reset to normal values
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetDiscount","params":[25],"id":1}' $NODE > /dev/null
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetPenalty","params":[10],"id":1}' $NODE > /dev/null

# 2. Execution mode bounds
echo ""
echo "--- Execution Mode Bounds ---"
R=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_executionModeSet","params":[99],"id":1}' \
  $NODE | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
check "Invalid mode defaults to standard" "standard" "$R"

curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_executionModeSet","params":[2],"id":1}' $NODE > /dev/null

# 3. Call cache toggle
echo ""
echo "--- Call Cache ---"
R=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_callCacheToggle","params":[true],"id":1}' \
  $NODE | grep -o '"result":[a-z]*' | cut -d: -f2)
check "Cache enable" "true" "$R"

R=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_callCacheToggle","params":[false],"id":1}' \
  $NODE | grep -o '"result":[a-z]*' | cut -d: -f2)
check "Cache disable" "false" "$R"

curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_callCacheToggle","params":[true],"id":1}' $NODE > /dev/null

# 4. Consensus check
echo ""
echo "--- Consensus ---"
BLOCKS=()
for port_ip in "192.168.1.15:8545" "192.168.1.34:8552" "192.168.1.134:8553" "192.168.1.16:8554"; do
  block=$(curl -s --max-time 3 -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://$port_ip 2>/dev/null | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
  if [ -n "$block" ]; then
    dec=$(printf '%d' "$block" 2>/dev/null)
    BLOCKS+=($dec)
    echo "  Node $port_ip: block $dec"
  else
    echo "  Node $port_ip: OFFLINE"
  fi
done

if [ ${#BLOCKS[@]} -ge 2 ]; then
  MIN=${BLOCKS[0]}; MAX=${BLOCKS[0]}
  for b in "${BLOCKS[@]}"; do
    [ "$b" -lt "$MIN" ] && MIN=$b
    [ "$b" -gt "$MAX" ] && MAX=$b
  done
  SPREAD=$((MAX-MIN))
  if [ $SPREAD -le 5 ]; then
    check "Block spread <= 5" "true" "true"
  else
    check "Block spread <= 5" "true" "false (spread=$SPREAD)"
  fi
fi

# 5. RPC endpoints respond
echo ""
echo "--- RPC Health ---"
for method in ethernova_nodeHealth ethernova_adaptiveGas ethernova_executionMode ethernova_callCache ethernova_evmProfile ethernova_optimizer ethernova_autoTuner ethernova_parallelStats; do
  R=$(curl -s --max-time 3 -X POST -H "Content-Type: application/json" \
    -d "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":[],\"id\":1}" \
    $NODE 2>/dev/null | grep -c '"result"')
  check "RPC $method responds" "1" "$R"
done

echo ""
echo "==========================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================="
