#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_TAG="$(head -n 1 "$ROOT_DIR/VERSION" | tr -d '\r\n')"
VERSION_DATE="$(date +%Y%m%d)"
OUT_DIR="$ROOT_DIR/dist"
OUT_BIN="$OUT_DIR/ethernova-$VERSION_TAG-linux-amd64"

LDFLAGS="-X github.com/ethereum/go-ethereum/internal/version.gitCommit=$VERSION_TAG -X github.com/ethereum/go-ethereum/internal/version.gitDate=$VERSION_DATE"

mkdir -p "$OUT_DIR"

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

echo "Building Ethernova (linux amd64, CGO disabled)..."
go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$OUT_BIN" ./cmd/geth

echo "Built $OUT_BIN"
