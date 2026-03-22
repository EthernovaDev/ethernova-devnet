#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"

printf '%s\n' "== Running full verification pack =="

ok=1
"$script_dir/verify_p2p_gate.sh" || ok=0
"$script_dir/verify-fork-linux.sh" || ok=0

if [[ "$ok" -eq 1 ]]; then
  printf '%s\n' "VERIFY_ALL: PASS"
  exit 0
fi

printf '%s\n' "VERIFY_ALL: FAIL"
exit 1
