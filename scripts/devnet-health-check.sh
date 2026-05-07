#!/usr/bin/env bash
set -euo pipefail

# Ethernova devnet health check. No secrets required.
# Override ENDPOINTS with newline-separated "name url" pairs if needed.
ENDPOINTS=${ENDPOINTS:-$'node1 http://192.168.1.15:8551\nnode2 http://192.168.1.34:8551\nnode3 http://192.168.1.134:8551\nnode4 http://192.168.1.16:8551\ndevrpc https://devrpc.ethnova.net'}
export ENDPOINTS
EXPLORER_STATUS_URL=${EXPLORER_STATUS_URL:-https://devexplorer.ethnova.net/api/v2/main-page/indexing-status}

python3 - <<'PY'
import json
import os
import time
import urllib.request

endpoints = []
for line in os.environ['ENDPOINTS'].splitlines():
    line = line.strip()
    if not line:
        continue
    name, url = line.split(maxsplit=1)
    endpoints.append((name, url))

def rpc(url, method, params=None, timeout=8):
    payload = json.dumps({'jsonrpc': '2.0', 'id': 1, 'method': method, 'params': params or []}).encode()
    req = urllib.request.Request(url, data=payload, headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(req, timeout=timeout) as response:
        result = json.loads(response.read().decode())
    if 'error' in result:
        raise RuntimeError(result['error'])
    return result['result']

print('Devnet health check', time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()))
rows = []
for name, url in endpoints:
    chain_id = int(rpc(url, 'eth_chainId'), 16)
    net_version = rpc(url, 'net_version')
    head = int(rpc(url, 'eth_blockNumber'), 16)
    block_hash = rpc(url, 'eth_getBlockByNumber', [hex(head), False])['hash']
    peers = int(rpc(url, 'net_peerCount'), 16)
    rows.append((name, url, chain_id, net_version, head, block_hash, peers))
    print(f'{name:6} chainId={chain_id} net={net_version} head={head} hash={block_hash} peers={peers}')

common = min(row[4] for row in rows)
print(f'common_check_block={common}')
hashes = []
for name, url, *_ in rows:
    block_hash = rpc(url, 'eth_getBlockByNumber', [hex(common), False])['hash']
    hashes.append(block_hash)
    print(f'commonHash {name:6} {block_hash}')
print('common_hash_match=' + ('PASS' if len(set(hashes)) == 1 else 'FAIL'))
PY

printf '\nExplorer indexing status:\n'
curl -fsS "$EXPLORER_STATUS_URL"
printf '\n'
