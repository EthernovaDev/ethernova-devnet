#!/bin/bash
# Ethernova Devnet Stress Test
# Sends a high volume of transactions to test adaptive gas under load

NODE="http://192.168.1.15:8551"
CONTRACT="0x740d3b1fb8f03c37a047b404d613c38051dd683b"
ACCOUNT_PASSWORD="devnet123"
TOTAL_TXS=${1:-500}
BATCH=50

echo "=== Ethernova Devnet Stress Test ==="
echo "Node: $NODE"
echo "Contract: $CONTRACT"
echo "Total txs: $TOTAL_TXS"
echo ""

# Get starting block
START_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  $NODE | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
echo "Starting block: $(printf '%d' $START_BLOCK)"

# Unlock account
GETH_PATH="$HOME/ethernova-devnet/build/bin/geth"
$GETH_PATH attach --exec "personal.unlockAccount(eth.accounts[0], '$ACCOUNT_PASSWORD', 3600)" $NODE 2>/dev/null | grep -v "INFO\|WARN"

SENT=0
FAILED=0

echo "Sending transactions..."
while [ $SENT -lt $TOTAL_TXS ]; do
  # Mix of tx types
  JS_CMD="personal.unlockAccount(eth.accounts[0], '$ACCOUNT_PASSWORD', 60);"

  # 60% inc() calls (light)
  for i in $(seq 1 $((BATCH * 60 / 100))); do
    JS_CMD="$JS_CMD eth.sendTransaction({from: eth.accounts[0], to: '$CONTRACT', data: '0xd09de08a', gas: 100000});"
  done

  # 30% loop(20) calls (heavy)
  for i in $(seq 1 $((BATCH * 30 / 100))); do
    JS_CMD="$JS_CMD eth.sendTransaction({from: eth.accounts[0], to: '$CONTRACT', data: '0xe5c19b2d0000000000000000000000000000000000000000000000000000000000000014', gas: 500000});"
  done

  # 10% plain transfers
  for i in $(seq 1 $((BATCH * 10 / 100))); do
    JS_CMD="$JS_CMD eth.sendTransaction({from: eth.accounts[0], to: '0x3333333333333333333333333333333333333333', value: 1000, gas: 21000});"
  done

  JS_CMD="$JS_CMD 'batch done';"

  RESULT=$($GETH_PATH attach --exec "$JS_CMD" $NODE 2>/dev/null | tail -1)
  SENT=$((SENT + BATCH))
  echo "  Sent: $SENT / $TOTAL_TXS"
  sleep 1
done

echo ""
echo "Waiting for blocks to process..."
sleep 30

# Get ending block
END_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  $NODE | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
echo "Ending block: $(printf '%d' $END_BLOCK)"

# Check consensus
echo ""
echo "=== Consensus Check ==="
for port_ip in "192.168.1.15:8551" "192.168.1.34:8552" "192.168.1.134:8553" "192.168.1.16:8554"; do
  block=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://$port_ip 2>/dev/null | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
  echo "  $port_ip: block=$(printf '%d' $block 2>/dev/null)"
done

# Check adaptive gas patterns
echo ""
echo "=== Adaptive Gas Patterns ==="
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' \
  $NODE

echo ""
echo ""
echo "=== Profiling ==="
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' \
  $NODE

echo ""
echo "Stress test complete."
