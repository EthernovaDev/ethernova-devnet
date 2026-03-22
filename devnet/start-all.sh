#!/bin/bash
# Start all 4 devnet nodes
# Nodes 1-2 mine, Nodes 3-4 are RPC/observer only

set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Starting Ethernova Devnet (4 nodes)..."

# Start miners in background
$SCRIPT_DIR/start-node.sh 1 --mine &
PID1=$!
sleep 2

$SCRIPT_DIR/start-node.sh 2 --mine &
PID2=$!
sleep 2

# Start observer nodes
$SCRIPT_DIR/start-node.sh 3 &
PID3=$!
sleep 2

$SCRIPT_DIR/start-node.sh 4 &
PID4=$!

echo ""
echo "All nodes started:"
echo "  Node 1 (miner):    PID=$PID1  RPC=http://localhost:8551"
echo "  Node 2 (miner):    PID=$PID2  RPC=http://localhost:8552"
echo "  Node 3 (observer): PID=$PID3  RPC=http://localhost:8553"
echo "  Node 4 (observer): PID=$PID4  RPC=http://localhost:8554"
echo ""
echo "To stop all: kill $PID1 $PID2 $PID3 $PID4"
echo "Or: pkill -f 'geth.*121526'"

wait
