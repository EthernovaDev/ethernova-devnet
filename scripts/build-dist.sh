#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

VERSION="${1:-}"
if [[ -z "$VERSION" && -f "$ROOT_DIR/VERSION" ]]; then
  VERSION="$(tr -d '\r' < "$ROOT_DIR/VERSION")"
fi
if [[ -z "$VERSION" ]]; then
  echo "Version not set. Provide as arg or set VERSION file." >&2
  exit 1
fi

echo "Packaging release $VERSION (linux/amd64)"

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

mkdir -p "$ROOT_DIR/bin"
go build -o "$ROOT_DIR/bin/ethernova" "$ROOT_DIR/cmd/geth"
go build -o "$ROOT_DIR/bin/evmcheck" "$ROOT_DIR/cmd/evmcheck"

DIST_DIR="$ROOT_DIR/dist"
STAGE_LINUX="$DIST_DIR/stage-linux"

rm -rf "$DIST_DIR"
mkdir -p "$STAGE_LINUX/bin" "$STAGE_LINUX/genesis" "$STAGE_LINUX/network" "$STAGE_LINUX/scripts" "$STAGE_LINUX/docs" "$STAGE_LINUX/systemd"

cp "$ROOT_DIR/bin/ethernova" "$STAGE_LINUX/bin/ethernova"
cp "$ROOT_DIR/bin/evmcheck" "$STAGE_LINUX/bin/evmcheck"

for name in genesis-mainnet.json genesis-dev.json; do
  cp "$ROOT_DIR/$name" "$STAGE_LINUX/genesis/$name"
done

cp "$ROOT_DIR/docs/runbooks/OPERATOR_RUNBOOK.md" "$STAGE_LINUX/OPERATOR_RUNBOOK.md"
cp "$ROOT_DIR/docs/README_QUICKSTART.md" "$STAGE_LINUX/README_QUICKSTART.md"
cp "$ROOT_DIR/docs/releases/RELEASE-NOTES.md" "$STAGE_LINUX/RELEASE-NOTES.md"
cp "$ROOT_DIR/docs/releases/RELEASE_NOTES_v1.2.7.md" "$STAGE_LINUX/RELEASE_NOTES_v1.2.7.md"
cp "$ROOT_DIR/docs/README-WINDOWS.txt" "$STAGE_LINUX/README-WINDOWS.txt"
cp "$ROOT_DIR/docs/README-LINUX.txt" "$STAGE_LINUX/README-LINUX.txt"
cp "$ROOT_DIR/docs/releases/RELEASE_v1.2.7.md" "$STAGE_LINUX/RELEASE_v1.2.7.md"


if [[ -d "$ROOT_DIR/network" ]]; then
  cp -a "$ROOT_DIR/network/." "$STAGE_LINUX/network/"
fi

cp "$ROOT_DIR/scripts/"*.ps1 "$STAGE_LINUX/scripts/" 2>/dev/null || true
cp "$ROOT_DIR/scripts/"*.bat "$STAGE_LINUX/scripts/" 2>/dev/null || true
cp "$ROOT_DIR/scripts/"*.sh "$STAGE_LINUX/scripts/" 2>/dev/null || true

cp "$ROOT_DIR/scripts/update.sh" "$STAGE_LINUX/update.sh"
cp "$ROOT_DIR/scripts/update-1.2.7.sh" "$STAGE_LINUX/update-1.2.7.sh"
cp "$ROOT_DIR/scripts/install.sh" "$STAGE_LINUX/install.sh"
cp "$ROOT_DIR/docs/README-LINUX.txt" "$STAGE_LINUX/README-LINUX.txt"

if [[ -f "$ROOT_DIR/systemd/ethernova.service" ]]; then
  cp "$ROOT_DIR/systemd/ethernova.service" "$STAGE_LINUX/systemd/ethernova.service"
fi

chmod +x "$STAGE_LINUX/bin/ethernova" "$STAGE_LINUX/bin/evmcheck" || true
chmod +x "$STAGE_LINUX/scripts/"*.sh "$STAGE_LINUX/update.sh" "$STAGE_LINUX/update-1.2.7.sh" "$STAGE_LINUX/install.sh" 2>/dev/null || true

tar_name="ethernova-linux-amd64-$VERSION.tar.gz"
tar_path="$DIST_DIR/$tar_name"
tar -czf "$tar_path" -C "$STAGE_LINUX" .

sha256sum "$tar_path" > "$DIST_DIR/checksums-sha256.txt"

echo "Artifacts:"
ls -1 "$DIST_DIR"
