#!/bin/bash
# Stop all devnet nodes
echo "Stopping all Ethernova devnet nodes..."
pkill -f "geth.*121526" 2>/dev/null || true
sleep 2
if pgrep -f "geth.*121526" > /dev/null; then
    echo "Force killing..."
    pkill -9 -f "geth.*121526" 2>/dev/null || true
fi
echo "All devnet nodes stopped."
