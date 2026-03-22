#!/bin/bash
# Reset all devnet data (wipes chain, keeps genesis)
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "WARNING: This will delete all devnet chain data!"
read -p "Continue? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 0
fi

$SCRIPT_DIR/stop-all.sh

echo "Removing data directories..."
rm -rf "$SCRIPT_DIR/data/node1"
rm -rf "$SCRIPT_DIR/data/node2"
rm -rf "$SCRIPT_DIR/data/node3"
rm -rf "$SCRIPT_DIR/data/node4"

echo "Devnet reset complete. Run start-all.sh to reinitialize."
