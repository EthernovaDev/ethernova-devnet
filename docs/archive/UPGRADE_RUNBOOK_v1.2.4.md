# UPGRADE_RUNBOOK_v1.2.4

## Summary
Ethernova v1.2.4 schedules a hard fork at block **70000** to activate the missing Byzantium base package.
This fixes contract-to-contract view calls by enabling STATICCALL (EIP-214) and related base EVM rules.

## Who must upgrade
- RPC nodes
- Miners/validators
- Seed/boot nodes

## Pre-fork checklist (before block 70000)
1) Update binaries to v1.2.4.
   - Windows: `update.bat`
   - Linux: `./update.sh`
2) Restart the node/service.
3) Verify version:
   - `ethernova version` shows **1.2.4**
4) Ensure config update applied (no chain reset):
   - `ethernova --datadir <your-datadir> init genesis-upgrade-70000.json`

## Post-fork checklist (block >= 70000)
1) Deploy a BalanceChecker (or similar) contract.
2) Call `balanceOf` via a contract-to-contract view call.
3) Re-test DEX flows (addLiquidity/mint) that previously failed.

## Notes
- Do NOT replace the genesis file in your datadir.
- The run-mainnet-node scripts apply `genesis-upgrade-70000.json` automatically if present (idempotent).
