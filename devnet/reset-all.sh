#!/bin/bash
# Reset all devnet data (wipes chain, keeps genesis)
#
# REQUIRED after pulling the v2.0.0 baseline: this release changes the
# genesis block, so any existing chaindata from the v1.x line is
# incompatible and must be wiped before start-all.sh.
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
