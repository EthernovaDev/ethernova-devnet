#!/bin/bash
# Ethernova Devnet - Deploy contracts and run gas benchmarks
# Usage: ./deploy-and-benchmark.sh [rpc_url] [private_key]

RPC=${1:-"http://127.0.0.1:9545"}
PRIVKEY=${2:-"63ad8427b96fc8f13cf7ce252fcad2f7b0659035bdc893e097fdd8cde250c37e"}
CHAINID=121526

echo "============================================"
echo "  Ethernova Devnet - Phase 6 Benchmark"
echo "  RPC: $RPC"
echo "============================================"
echo ""

# Helper: send raw signed transaction
send_raw_tx() {
    local TO=$1
    local DATA=$2
    local VALUE=${3:-"0x0"}
    local GAS=${4:-"0x200000"}

    # Get nonce
    NONCE=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionCount\",\"params\":[\"$(get_address)\",\"pending\"],\"id\":1}" \
        $RPC | grep -o '"result":"[^"]*"' | cut -d'"' -f4)

    # For deployment we need eth_sendTransaction with unlocked account or use the faucet key
    # Simplified: use eth_sendTransaction via personal API
    local PARAMS
    if [ "$TO" = "null" ] || [ -z "$TO" ]; then
        PARAMS="{\"from\":\"$(get_address)\",\"data\":\"$DATA\",\"value\":\"$VALUE\",\"gas\":\"$GAS\"}"
    else
        PARAMS="{\"from\":\"$(get_address)\",\"to\":\"$TO\",\"data\":\"$DATA\",\"value\":\"$VALUE\",\"gas\":\"$GAS\"}"
    fi

    RESULT=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[$PARAMS],\"id\":1}" \
        $RPC)
    echo "$RESULT" | grep -o '"result":"[^"]*"' | cut -d'"' -f4
}

get_address() {
    echo "0x818c1965E44A033115666F47DFF1752C656652C2"
}

wait_for_tx() {
    local TX=$1
    for i in $(seq 1 30); do
        RECEIPT=$(curl -s -X POST -H "Content-Type: application/json" \
            --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$TX\"],\"id\":1}" \
            $RPC)
        if echo "$RECEIPT" | grep -q '"gasUsed"'; then
            echo "$RECEIPT"
            return 0
        fi
        sleep 2
    done
    echo "TIMEOUT"
    return 1
}

echo "=== Enabling all Ethernova features ==="
curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasToggle","params":[true],"id":1}' $RPC > /dev/null
curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"ethernova_optimizerToggle","params":[true],"id":1}' $RPC > /dev/null
curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"ethernova_callCacheToggle","params":[true],"id":1}' $RPC > /dev/null
curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"ethernova_autoTunerToggle","params":[true],"id":1}' $RPC > /dev/null
echo "Done"
echo ""

echo "=== Sending 100 ETH transfers (parallel mode test) ==="
START_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $RPC | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
echo "Start block: $START_BLOCK"

ADDR="0x246Cbae156Cf083F635C0E1a01586b730678f5Cb"
for i in $(seq 1 100); do
    curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$(get_address)\",\"to\":\"$ADDR\",\"value\":\"0xDE0B6B3A7640000\",\"gas\":\"0x5208\"}],\"id\":$i}" \
        $RPC > /dev/null &
    if [ $((i % 20)) -eq 0 ]; then
        wait
        echo "  Sent $i/100 transfers..."
    fi
done
wait
echo "  All 100 transfers sent"
sleep 10

END_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $RPC | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
echo "End block: $END_BLOCK"
echo ""

echo "=== Collecting Results ==="
echo ""

echo "--- Adaptive Gas ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' $RPC | python3 -m json.tool 2>/dev/null || \
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGas","params":[],"id":1}' $RPC
echo ""

echo "--- EVM Profile ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' $RPC | python3 -m json.tool 2>/dev/null || \
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' $RPC
echo ""

echo "--- Optimizer ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_optimizer","params":[],"id":1}' $RPC
echo ""

echo "--- Call Cache ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_callCache","params":[],"id":1}' $RPC
echo ""

echo "--- Parallel Stats ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_parallelStats","params":[],"id":1}' $RPC
echo ""

echo "--- Auto Tuner ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_autoTuner","params":[],"id":1}' $RPC
echo ""

echo "--- Precompiles ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_precompiles","params":[],"id":1}' $RPC
echo ""

echo "--- Node Health ---"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_nodeHealth","params":[],"id":1}' $RPC
echo ""

echo ""
echo "============================================"
echo "  Benchmark Complete"
echo "============================================"
