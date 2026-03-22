#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

go_bin="$(command -v go || true)"
if [[ -z "$go_bin" && -x "$repo_root/.tools/go/bin/go" ]]; then
  export GOROOT="$repo_root/.tools/go"
  export PATH="$GOROOT/bin:$PATH"
  go_bin="$GOROOT/bin/go"
fi

if [[ -z "$go_bin" ]]; then
  printf '%s\n' "FAIL: go not found. Install Go 1.21+ or place it at .tools/go."
  exit 1
fi

printf '%s\n' "== P2P version gate verification =="
output="$("$go_bin" test ./eth -run TestVerifyPeerVersionGate -v 2>&1)"
printf '%s\n' "$output"

reject_line="$(printf '%s\n' "$output" | grep -F 'VERIFY_P2P_GATE: name="CoreGeth/v1.2.6' || true)"
accept_line="$(printf '%s\n' "$output" | grep -F 'VERIFY_P2P_GATE: name="CoreGeth/v1.2.7' || true)"

if [[ -n "$reject_line" ]] && printf '%s\n' "$reject_line" | grep -Fq 'rejected' \
  && [[ -n "$accept_line" ]] && printf '%s\n' "$accept_line" | grep -Fq 'accepted'; then
  printf '%s\n' "PASS: P2P version gate"
  exit 0
fi

printf '%s\n' "FAIL: P2P version gate"
exit 1
