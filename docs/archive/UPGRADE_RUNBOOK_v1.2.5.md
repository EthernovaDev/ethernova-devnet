# UPGRADE_RUNBOOK_v1.2.5

## Summary
Ethernova v1.2.5 switches chainId from legacy-chainid to 121525 at block 138392. The switch is
runtime-only, so no genesis update or datadir wipe is required.

## Who must upgrade
- RPC nodes
- Miners/validators
- Seed/boot nodes

## Pre-fork checklist (before block 138392)
1) Update binaries to v1.2.5.
   - Windows: `update-1.2.5.bat` or `update.bat`
   - Linux: `./update-1.2.5.sh` or `./update.sh`
2) Restart the node/service.
3) Verify version:
   - `ethernova version` shows **1.2.5**
4) Verify RPC chainId:
   - `eth_chainId` returns **0x1dab5** (121525)
5) Ensure networkid is updated to 121525 (scripts and service files updated).

## Post-fork checklist (block >= 138392)
1) Confirm new txs signed with chainId 121525 are accepted.
2) Confirm txs signed with chainId legacy-chainid are rejected.
3) Check peer count recovers on the 121525 network.

## Notes
- Do NOT replace the genesis file in your datadir.
- If you have not applied the older upgrade genesis (70000), run:
  `ethernova --datadir <your-datadir> init genesis-upgrade-70000.json`
