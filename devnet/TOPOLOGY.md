# Ethernova Devnet Topology

## Network: chainId 121526

## Nodes (4 VMs on ESXi)

| Node | Role     | P2P Port | RPC Port | WS Port | Miner Address                              |
|------|----------|----------|----------|---------|--------------------------------------------|
| 1    | Miner    | 30301    | 8551     | 8561    | 0x1111111111111111111111111111111111111111   |
| 2    | Miner    | 30302    | 8552     | 8562    | 0x2222222222222222222222222222222222222222   |
| 3    | Observer | 30303    | 8553     | 8563    | -                                          |
| 4    | Observer | 30304    | 8554     | 8564    | -                                          |

## Architecture

- **Nodes 1-2**: Mine blocks, produce profiling data
- **Nodes 3-4**: Validate blocks, serve RPC, observe consensus
- All nodes run `--nodiscover` and connect via static-nodes.json
- All nodes expose `ethernova` RPC namespace (including `evmProfile`)

## Consensus Validation

All 4 nodes must stay in sync. If any node diverges, the experimental
feature has broken determinism and must be fixed before proceeding.

## Quick Check

```bash
# Compare block numbers across all nodes
for port in 8551 8552 8553 8554; do
  echo -n "Node $port: "
  curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:$port | grep -o '"result":"[^"]*"'
done
```
