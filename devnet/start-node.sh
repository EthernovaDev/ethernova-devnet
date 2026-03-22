#!/bin/bash
# Ethernova Devnet Node Startup Script
# Usage: ./start-node.sh <node-number> [--mine]
#
# Node 1: port 30301, rpc 8551, ws 8561
# Node 2: port 30302, rpc 8552, ws 8562
# Node 3: port 30303, rpc 8553, ws 8563
# Node 4: port 30304, rpc 8554, ws 8564

set -e

NODE_NUM=${1:-1}
MINE_FLAG=""
if [ "$2" == "--mine" ]; then
    MINE_FLAG="--mine --miner.threads=1"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GENESIS="$SCRIPT_DIR/genesis-devnet.json"
GETH="$SCRIPT_DIR/../build/bin/geth"
DATADIR="$SCRIPT_DIR/data/node$NODE_NUM"
P2P_PORT=$((30300 + NODE_NUM))
RPC_PORT=$((8550 + NODE_NUM))
WS_PORT=$((8560 + NODE_NUM))
NETWORK_ID=121526

# Miner addresses (one per node)
MINERS=(
    "0x1111111111111111111111111111111111111111"
    "0x2222222222222222222222222222222222222222"
    "0x3333333333333333333333333333333333333333"
    "0x4444444444444444444444444444444444444444"
)
ETHERBASE=${MINERS[$((NODE_NUM - 1))]}

echo "============================================"
echo "  ETHERNOVA DEVNET - Node $NODE_NUM"
echo "  P2P:  $P2P_PORT"
echo "  RPC:  http://localhost:$RPC_PORT"
echo "  WS:   ws://localhost:$WS_PORT"
echo "  Data: $DATADIR"
echo "  Mine: $([ -n "$MINE_FLAG" ] && echo "YES ($ETHERBASE)" || echo "NO")"
echo "============================================"

# Init genesis if needed
if [ ! -d "$DATADIR/geth/chaindata" ]; then
    echo "Initializing genesis..."
    $GETH init --datadir "$DATADIR" "$GENESIS"
fi

# Static nodes file
STATIC_NODES="$DATADIR/geth/static-nodes.json"
if [ ! -f "$STATIC_NODES" ]; then
    mkdir -p "$DATADIR/geth"
    cp "$SCRIPT_DIR/static-nodes.json" "$STATIC_NODES" 2>/dev/null || echo "[]" > "$STATIC_NODES"
fi

# Start node
exec $GETH \
    --networkid $NETWORK_ID \
    --datadir "$DATADIR" \
    --port $P2P_PORT \
    --http --http.addr 0.0.0.0 --http.port $RPC_PORT \
    --http.api eth,net,web3,debug,txpool,admin,ethernova \
    --http.corsdomain "*" \
    --ws --ws.addr 0.0.0.0 --ws.port $WS_PORT \
    --ws.api eth,net,web3,debug,txpool,admin,ethernova \
    --ws.origins "*" \
    --miner.etherbase "$ETHERBASE" \
    --allow-insecure-unlock \
    --nodiscover \
    --verbosity 3 \
    $MINE_FLAG \
    --log.file "$DATADIR/node.log"
