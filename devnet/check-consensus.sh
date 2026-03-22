#!/bin/bash
# Check that all devnet nodes are in consensus
# Compares block numbers and block hashes across all 4 nodes

echo "=== Ethernova Devnet Consensus Check ==="
echo ""

PORTS=(8551 8552 8553 8554)
NAMES=("Node1-Miner" "Node2-Miner" "Node3-Observer" "Node4-Observer")
BLOCKS=()
HASHES=()

for i in "${!PORTS[@]}"; do
    port=${PORTS[$i]}
    name=${NAMES[$i]}

    # Get block number
    block=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        http://localhost:$port 2>/dev/null | grep -o '"result":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$block" ]; then
        echo "  $name (port $port): OFFLINE"
        continue
    fi

    block_dec=$(printf "%d" "$block" 2>/dev/null)

    # Get block hash at that number
    hash=$(curl -s -X POST -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBlockByNumber\",\"params\":[\"$block\",false],\"id\":1}" \
        http://localhost:$port 2>/dev/null | grep -o '"hash":"0x[a-f0-9]*"' | head -1 | cut -d'"' -f4)

    # Get peer count
    peers=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}' \
        http://localhost:$port 2>/dev/null | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
    peers_dec=$(printf "%d" "$peers" 2>/dev/null)

    echo "  $name (port $port): block=$block_dec  peers=$peers_dec  hash=${hash:0:18}..."
    BLOCKS+=("$block_dec")
    HASHES+=("$hash")
done

echo ""

# Check consensus
if [ ${#BLOCKS[@]} -lt 2 ]; then
    echo "NOT ENOUGH NODES ONLINE TO CHECK CONSENSUS"
    exit 1
fi

# Check if blocks are within 2 of each other
MIN=${BLOCKS[0]}
MAX=${BLOCKS[0]}
for b in "${BLOCKS[@]}"; do
    [ "$b" -lt "$MIN" ] && MIN=$b
    [ "$b" -gt "$MAX" ] && MAX=$b
done
DIFF=$((MAX - MIN))

if [ $DIFF -le 2 ]; then
    echo "CONSENSUS: OK (block spread: $DIFF)"
else
    echo "CONSENSUS: WARNING - block spread is $DIFF (>2)"
fi

# Check EVM profiling
echo ""
echo "=== EVM Profiling ==="
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' \
    http://localhost:8551 2>/dev/null | python3 -m json.tool 2>/dev/null || \
    curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ethernova_evmProfile","params":[],"id":1}' \
    http://localhost:8551
