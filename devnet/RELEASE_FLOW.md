# Ethernova Devnet Release Flow

This is the default flow for every NIP phase once Noven says the phase is
ready to build/audit.

## Scope

- Devnet chain/network: `121526`
- Devnet node fleet: 4 ESXi VMs from `devnet/TOPOLOGY.md`
- Explorer VPS: update together with the node fleet because it serves the live
  devnet explorer/RPC-facing view.

Do not paste credentials, private keys, or SSH secrets into this file.

## Required Steps

1. Sync source.

```bash
git fetch --all --prune
git pull --ff-only
git status -sb
git log --oneline -5
```

2. Audit the phase implementation.

- Confirm fork blocks and precompile addresses in `params/ethernova/forks.go`.
- Check stateful precompile write paths for `readOnly` / `STATICCALL`.
- Check deterministic storage keys, monotonic counters, and no map iteration.
- Check cross-block deferred processing paths before release.

3. Run focused tests.

```bash
go test ./core/types ./core/vm ./core -run 'Mailbox|Deferred|ProtocolObject|ContentRef' -count=1
go test ./core/vm -run 'Mailbox' -count=1 -v
```

Full upstream suite can have fork-local failures; document those separately and
do not confuse them with phase-specific readiness.

4. Build Linux binary for VMs/VPS.

```bash
bash scripts/build-linux.sh
shasum -a 256 dist/ethernova-v2.0.0-devnet-linux-amd64
```

5. Deploy to all devnet nodes.

- Node 1 miner: RPC `8551`, WS `8561`, P2P `30301`
- Node 2 miner: RPC `8552`, WS `8562`, P2P `30302`
- Node 3 observer: RPC `8553`, WS `8563`, P2P `30303`
- Node 4 observer: RPC `8554`, WS `8564`, P2P `30304`

Recommended server-side pattern:

```bash
systemctl stop ethernova || true
install -m 0755 ethernova-v2.0.0-devnet-linux-amd64 /usr/local/bin/ethernova
systemctl start ethernova
systemctl status ethernova --no-pager
```

Adapt paths/service names to the actual VM setup.

6. Deploy to the explorer VPS.

- Update the node binary used by the explorer/RPC backend.
- Restart the explorer node/backend services.
- Confirm the explorer points to devnet `121526` and sees the upgraded node.

7. Post-deploy health checks.

```bash
for port in 8551 8552 8553 8554; do
  echo "RPC $port"
  curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    "http://localhost:$port"
  echo
done
```

Also check:

- `eth_chainId` returns `0x1dac6`.
- `net_version` returns `121526`.
- All nodes converge on the same block height/hash.
- `ethernova_mailboxConfig` reports Phase 4 active when testing Phase 4.
- `ethernova_deferredProcessingStats` does not show stuck queue growth.

8. Report results.

Include:

- Commit hash.
- Binary path and SHA256.
- Tests run.
- Nodes/VPS updated.
- Any known residual risks or upstream/fork-local test failures.
