# Ethernova v1.2.6 (MANDATORY)

MANDATORY UPDATE: v1.2.6 enforces the chainId split fix at block **138,396** and disconnects peers running older binaries.

- Fork enforcement block: **138,396** (>= 138,396).
- chainId/networkId: **121525** (0x1dab5).
- Older clients are rejected at P2P handshake and cannot mine valid blocks after the fork.

Upgrade steps:
- Windows: use `dist/update-windows.ps1` to stop service, back up the old binary, replace it, and restart.
- Linux: use `dist/update-linux.sh` to stop systemd, back up the old binary, replace it, daemon-reload if needed, and restart.

Verification commands:
- `eth_chainId` should return `0x1dab5`.
- `net_version` should return `121525`.
- `eth_getBlockByNumber(0).hash` should return `0xc67bd6160c1439360ab14abf7414e8f07186f3bed095121df3f3b66fdc6c2183`.

# Ethernova v1.2.5

- Hardfork at block 138392 switches chainId to 121525 (0x1dab5).
- Pre-fork dual-accept of legacy-chainid/121525; post-fork only 121525.
- RPC eth_chainId returns 121525; scripts default networkid 121525; no genesis re-init required.

# Ethernova v1.2.4

- Hardfork 1.2.4 at block 70000 enables Byzantium base (EIP-214 STATICCALL) to fix contract-to-contract view calls.
- New upgrade genesis: `genesis-upgrade-70000.json` (config-only, no chain reset).
- One-click update scripts for Windows + Linux bundles.

# Ethernova v1.2.3-nova

- Fork60000 upgrade files (mainnet config update + devnet fork-20).
- Modern EVM opcode subset for 2025 contracts (Shanghai/Cancun: PUSH0, MCOPY, TSTORE/TLOAD, SELFDESTRUCT changes, initcode limits, warm coinbase).
- Expanded `evmcheck` with on-chain opcode verification and PASS/FAIL output.
- Operator runbook updates and fork-specific release notes.
- One-click devnet test/run scripts for Windows and Linux bundles.

Expected `evmcheck` output (pre-fork vs post-fork):

```text
Current block: 10
Fork block: 60000
Pre-fork: true
CHAINID opcode: FAIL (expected pre-fork failure: invalid opcode (CHAINID/0x46))
CREATE2 opcode: FAIL (expected pre-fork failure: invalid opcode (CREATE2/0xF5))
PUSH0 opcode: FAIL (expected pre-fork failure: invalid opcode (PUSH0))
MCOPY opcode: FAIL (expected pre-fork failure: invalid opcode (MCOPY))
TSTORE/TLOAD opcodes: FAIL (expected pre-fork failure: invalid opcode (TSTORE/TLOAD))
SELFDESTRUCT (EIP-6780): FAIL (expected pre-fork behavior: code deleted)
EVM upgrade check: FAIL
```

```text
Current block: 60002
Fork block: 60000
Pre-fork: false
CHAINID opcode: PASS
CREATE2 opcode: PASS
PUSH0 opcode: PASS
MCOPY opcode: PASS
TSTORE/TLOAD opcodes: PASS
SELFDESTRUCT (EIP-6780): PASS
EVM upgrade check: PASS
```

# Ethernova v1.0.0-nova

- Ethash PoW chain with EIP-1559 baseFee redirected to treasury vault `0x3a38560b66205bb6a31decbcb245450b2f15d4fd`; tips remain with miners.
- Block reward schedule: starts at 10 NOVA, halves every ~2,102,400 blocks (~1 year @15s) with a 1 NOVA floor (encoded in genesis).
- Forks active from block 0: Berlin/London (type-2 tx, baseFee active).
- Genesis files: `genesis-mainnet.json` (chainId legacy-chainid, difficulty 0x400000, extraData "NOVA MAINNET") and `genesis-dev.json` (chainId 77778, difficulty 0x1).
- Windows scripts: build, init, smoke test, bootnode, second node, peering check, genesis fingerprint, release packaging.
