#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR"
if [[ "$(basename "$ROOT_DIR")" == "scripts" ]]; then
  ROOT_DIR="$(cd "$ROOT_DIR/.." && pwd)"
fi

export RELEASE_VERSION="${RELEASE_VERSION:-v1.2.7}"

if [[ -x "$ROOT_DIR/update.sh" ]]; then
  exec "$ROOT_DIR/update.sh" "$@"
fi
if [[ -x "$SCRIPT_DIR/update.sh" ]]; then
  exec "$SCRIPT_DIR/update.sh" "$@"
fi

echo "ERROR: update.sh not found." >&2
exit 1
