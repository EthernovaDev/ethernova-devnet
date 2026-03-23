#!/bin/bash
# Ethernova Devnet Benchmark
# Measures gas savings with adaptive gas vs standard EVM

NODE="${1:-http://192.168.1.34:8552}"
echo "=== Ethernova Gas Savings Benchmark ==="
echo "Node: $NODE"
echo ""

# Get current settings
SETTINGS=$(curl -s --max-time 5 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' $NODE)

DISCOUNT=$(echo "$SETTINGS" | grep -o '"discountPercent":[0-9]*' | cut -d: -f2)
PENALTY=$(echo "$SETTINGS" | grep -o '"penaltyPercent":[0-9]*' | cut -d: -f2)
ENABLED=$(echo "$SETTINGS" | grep -o '"enabled":[a-z]*' | cut -d: -f2)

echo "Adaptive Gas: $ENABLED (discount=${DISCOUNT}%, penalty=${PENALTY}%)"
echo ""

# Get profiling data
PROFILE=$(curl -s --max-time 5 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' $NODE)

TOTAL_OPS=$(echo "$PROFILE" | grep -o '"totalOps":[0-9]*' | cut -d: -f2)
TOTAL_GAS=$(echo "$PROFILE" | grep -o '"totalGas":[0-9]*' | cut -d: -f2)

echo "--- Profiling Data ---"
echo "  Total opcodes executed: $TOTAL_OPS"
echo "  Total gas consumed: $TOTAL_GAS"

if [ "$TOTAL_OPS" -gt 0 ] 2>/dev/null; then
  AVG_GAS=$(( TOTAL_GAS / TOTAL_OPS ))
  echo "  Average gas per opcode: $AVG_GAS"
fi

# Get optimizer data
OPTIMIZER=$(curl -s --max-time 5 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_optimizer","params":[],"id":1}' $NODE)

REDUNDANT=$(echo "$OPTIMIZER" | grep -o '"redundantOps":[0-9]*' | cut -d: -f2)
GAS_REFUNDED=$(echo "$OPTIMIZER" | grep -o '"gasRefunded":[0-9]*' | cut -d: -f2)

echo ""
echo "--- Optimizer ---"
echo "  Redundant ops detected: ${REDUNDANT:-0}"
echo "  Gas refunded: ${GAS_REFUNDED:-0}"

# Get cache data
CACHE=$(curl -s --max-time 5 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_callCache","params":[],"id":1}' $NODE)

HITS=$(echo "$CACHE" | grep -o '"hits":[0-9]*' | cut -d: -f2)
MISSES=$(echo "$CACHE" | grep -o '"misses":[0-9]*' | cut -d: -f2)
HIT_RATE=$(echo "$CACHE" | grep -o '"hitRate":[0-9.]*' | cut -d: -f2)

echo ""
echo "--- Call Cache ---"
echo "  Cache hits: ${HITS:-0}"
echo "  Cache misses: ${MISSES:-0}"
echo "  Hit rate: ${HIT_RATE:-0}%"

# Get fast mode data
EXEC=$(curl -s --max-time 5 -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_executionMode","params":[],"id":1}' $NODE)

MODE=$(echo "$EXEC" | grep -o '"mode":"[^"]*"' | cut -d'"' -f4)
FAST_EXEC=$(echo "$EXEC" | grep -o '"fastExecutions":[0-9]*' | cut -d: -f2)
SKIPPED=$(echo "$EXEC" | grep -o '"skippedChecks":[0-9]*' | cut -d: -f2)

echo ""
echo "--- Execution Mode ---"
echo "  Mode: $MODE"
echo "  Fast executions: ${FAST_EXEC:-0}"
echo "  Skipped checks: ${SKIPPED:-0}"

# Calculate estimated savings
echo ""
echo "=== Estimated Gas Savings vs Standard EVM ==="

TOTAL=${TOTAL_GAS:-0}
REFUNDED=${GAS_REFUNDED:-0}

if [ "$TOTAL" -gt 0 ]; then
  # Discount savings (for pure contracts)
  DISCOUNT_SAVINGS=$(( TOTAL * ${DISCOUNT:-0} / 100 ))
  echo "  Adaptive gas discount (${DISCOUNT}%): ~${DISCOUNT_SAVINGS} gas saved"
  echo "  Optimizer refunds: ${REFUNDED} gas saved"

  TOTAL_SAVED=$(( DISCOUNT_SAVINGS + REFUNDED ))
  PERCENT_SAVED=$(( TOTAL_SAVED * 100 / TOTAL ))
  echo ""
  echo "  Total estimated savings: ~${TOTAL_SAVED} gas (${PERCENT_SAVED}%)"
  echo "  Standard EVM would cost: ${TOTAL} gas"
  echo "  Ethernova costs: ~$(( TOTAL - TOTAL_SAVED )) gas"
else
  echo "  Not enough data yet. Run more transactions."
fi

echo ""
echo "Benchmark complete."
