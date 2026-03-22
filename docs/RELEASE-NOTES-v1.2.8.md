# Ethernova v1.2.8 (Chain 121525)

## Why this update matters
- Enforces the EVM compatibility fork at **block 105000** (Constantinople + Petersburg + Istanbul), fixing modern Solidity opcodes.
- Ensures fork enforcement logging is **chain-aware**: chain **121525** reports **105,000** (legacy **138,396** only applies to legacy chainId 77777 or legacy genesis).
- Adds repeatable verification (Go tests + RPC scripts) for opcode activation and chain identity.

## Mandatory upgrade
- **Upgrade before block 105000** (or immediately if already past the fork).

## Verification highlights
- chainId/networkId: **121525** (hex **0x1dab5**)
- genesis hash: **0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9**
- genesis SHA256 fingerprint: **768f10bf9f77e20d5da970436e283c5a6892c9169a7af6d33c8e8ec506c9957d**
- SHL/CHAINID/SELFBALANCE invalid at **104999**, valid at **105000** (Go tests + RPC verify scripts)
- ForkID “Next” changes at **105000**

## Downloads
- `ethernova-v1.2.8-windows-amd64.exe`
- `ethernova-v1.2.8-linux-amd64`
- `SHA256SUMS-v1.2.8.txt`

## In-repo audit & verification
- `docs/AUDIT-v1.2.8-121525.md`
- `docs/REPORT-v1.2.8-fork-verification.md`
- `docs/RELEASE-v1.2.8.md`
- `docs/UPGRADE-v1.2.8.md`
