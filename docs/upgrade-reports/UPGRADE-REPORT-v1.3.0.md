# UPGRADE-REPORT v1.3.0

Generated: 2026-02-05 17:22:44

## Summary

| Host | Pre Version | Post Version | Pre fork next (from peers) | Post fork next (from peers) | PeerCount pre/post | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| RPC/Explorer (207.180.230.125) | Ethernova/v1.2.9/linux-amd64/go1.21.13 | Ethernova/v1.3.0/linux-amd64/go1.21.13 | 110500 | 118200 | 0x4 / 0x1 | pre mismatch != 110500: 0, post mismatch != 118200: 0 |
| NovaPool (207.180.211.179) | Ethernova/v1.2.9/linux-amd64/go1.21.13 | Ethernova/v1.3.0/linux-amd64/go1.21.13 | n/a (admin_* disabled) | n/a (admin_* disabled) | 0x4 / 0x2 | admin_nodeInfo/admin_peers not available on this RPC |

## POST2 Recheck (Admin RPC Enabled on NovaPool)

| Host | Version | Fork next (from peers) | PeerCount | Notes |
| --- | --- | --- | --- | --- |
| RPC/Explorer (207.180.230.125) | Ethernova/v1.3.0/linux-amd64/go1.21.13 | 118200 | 0x1 | from *rpc-POST2-admin_peers.json |
| NovaPool (207.180.211.179) | Ethernova/v1.3.0/linux-amd64/go1.21.13 | 118200 | 0x2 | admin RPC enabled via systemd override |

## Peer Client Versions (pre/post)

RPC pre: Ethernova/v1.2.9/linux-amd64/go1.21.13
RPC post: Ethernova/v1.3.0/linux-amd64/go1.21.13
Pool pre: n/a (admin_peers disabled)
Pool post: n/a (admin_peers disabled)

## ForkID Drop Logs

See: *-POST-journal-forkid.txt

