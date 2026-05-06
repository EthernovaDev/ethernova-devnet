# Ethernova Phase 5 — Brutal Test Suite

Comprehensive **real-network** test suite for NIP-0004 Phase 5 (State Lifecycle Tiers). Tests run against your live devnet (not local Hardhat sandbox), verifying tier transitions, warming-fee gas math, witness restoration round-trip, EIP-214 STATICCALL safety, longevity (Active never demoted), brutal stress, regression on existing EVM, and full multi-node consensus.

## Quick Start (Windows)

```cmd
:: 1. First run akan auto-copy .env.example -> .env dan buka notepad
::    Edit .env: set PRIVATE_KEY=0x... (key dari faucet.ethnova.net)

:: 2. One command run all tests:
run.cmd
```

`run.cmd` otomatis:
1. Cek Node.js
2. Copy `.env.example` -> `.env` kalau belum ada (lalu buka notepad)
3. `npm install` (sekali)
4. `npx hardhat compile` (sekali)
5. Jalanin 10 suites berurutan
6. Print final summary

## Quick Start (Linux/Mac)

```bash
cp .env.example .env
$EDITOR .env  # set PRIVATE_KEY=0x...
npm install
npx hardhat compile
node scripts/run-all.js
```

## Struktur

```
p5test/
├── .env.example                    # Config template
├── package.json                    # npm deps
├── hardhat.config.js
├── run.cmd                         # Windows runner
├── README.md
├── contracts/
│   ├── LifecycleHarness.sol        # Storage exerciser (5 slots)
│   ├── WitnessProbe.sol             # STATICCALL test wrapper
│   └── RegressionToken.sol          # Vanilla ERC20
└── scripts/
    ├── shared.js                   # Helpers: RPC, consensus, logging
    ├── run-all.js                  # Orchestrator
    ├── print-report.js             # Pretty-print report.json
    ├── 00-preflight.js             # Env check
    ├── 01-deploy.js                # Deploy 3 contracts
    ├── 02-tier-transitions.js      # Active->Warm->Cold->Archived
    ├── 03-warming-fee.js           # Gas surcharge formula
    ├── 04-witness-restore.js       # Full witness round-trip
    ├── 05-eip214-staticcall.js     # EIP-214 enforcement
    ├── 06-active-immune.js         # Continuous touch never demoted
    ├── 07-multinode-consensus.js   # Full consensus health check
    ├── 08-brutal-stress.js         # 50 contracts, mixed lifecycle
    └── 09-regression.js            # Existing ERC20 still works
```

## Yang ditest

| # | Suite | Verifikasi |
|---|-------|-----------|
| 00 | Preflight | 2 node alive, chainId match, fork active, threshold sync, precompile 0x2F responds, signer funded |
| 01 | Deploy | LifecycleHarness, WitnessProbe, RegressionToken via Hardhat |
| 02 | **Tier transitions** | Active->Warm->Cold->Archived di blok thresholds + multi-node consensus tiap transition |
| 03 | Warming fee | Gas surcharge = `tier_gap × 32 × fee_per_byte`, multi-node estimateGas consensus |
| 04 | **Witness restore** | Full round-trip: archive → witness gen → restore via 0x2F → tier flips Active. Plus rejection paths. |
| 05 | **EIP-214** | Selector 0x02 (write) revert via STATICCALL, selectors 0x01/0x03 (read) success |
| 06 | Active immune | Contract continuously touched stays Active across full Cold-tier window |
| 07 | **Multi-node consensus** | Block hash, state root, WarmStateRoot, lifecycle config, tier of probes, storage roots all agree |
| 08 | **Brutal stress** | 50 contracts × 5 slots, concurrent burst, mixed lifecycle (Active/Warm/Cold/Archived), sweep cap stress, multi-node consensus |
| 09 | Regression | Vanilla ERC20 transfer/approve/transferFrom works, gas in normal range |

## Configuration (.env)

Wajib:
```ini
PRIVATE_KEY=0x...           # signer funded dari faucet.ethnova.net
```

Default 2-node setup:
```ini
PRIMARY_RPC=http://127.0.0.1:8545
CONSENSUS_NODES=Local=http://127.0.0.1:8545,Devrpc=https://devrpc.ethnova.net
```

Threshold values (HARUS match `params/ethernova/forks.go`):
```ini
ACTIVE_TIER_BLOCKS=10
WARM_TIER_BLOCKS=25
COLD_TIER_BLOCKS=50
WARMING_FEE_PER_BYTE=5
```

Skip flags (untuk debug iterasi):
```ini
SKIP_BRUTAL=1        # skip suite 08 (saves ~30 min)
SKIP_REGRESSION=1    # skip suite 09
SKIP_WITNESS=1       # skip suite 04 (kalau no archive node)
```

## Time budget

Devnet block time ~10-11s. Phase 5 transitions:
- Active->Warm: 10 blocks ≈ 2 menit
- Warm->Cold: 25 blocks ≈ 5 menit
- Cold->Archived: 50 blocks ≈ 9 menit

**Full run estimate**: usually under 1 hour with the current fast devnet thresholds. Suite 02 and 06 are still the longest because they wait for lifecycle transitions.

Untuk debug iteration cepat, pake SKIP flags atau run individual suite langsung:
```bash
node scripts/00-preflight.js
node scripts/02-tier-transitions.js
node scripts/07-multinode-consensus.js
node scripts/run-all.js --start 03 --skip 01,02 --continue-on-fail
```

## Pass criteria

Final summary harus muncul:
```
[PASS] ALL SUITES PASSED - Phase 5 ready to promote.
```

Kalau **any suite fails**, terutama **02 (tier-transitions)**, **04 (witness-restore)**, atau **07 (multi-node consensus)**: **JANGAN PROMOTE** ke fase selanjutnya. Itu indikator consensus split atau determinism bug yang bakal catastrophic di mainnet.

## Output

Live: terminal dengan per-check pass/fail/skip + final summary.

Persisted: `./report.json` dengan all suite results (use `node scripts/print-report.js` untuk re-print).

## Multi-node setup

Default `CONSENSUS_NODES` = `Local=http://127.0.0.1:8545,Devrpc=https://devrpc.ethnova.net` (sesuai req lo: 2 node).

Untuk full topology 4-node + devrpc, edit `.env`:
```ini
CONSENSUS_NODES=M1=http://127.0.0.1:8551,M2=http://127.0.0.1:8552,O3=http://127.0.0.1:8553,O4=http://127.0.0.1:8554,Devrpc=https://devrpc.ethnova.net
```

Kalau cuma 1 node reachable, multi-node checks SKIPPED (not failed).

## Troubleshooting

**"PRIVATE_KEY missing"**  
Edit `.env`: `PRIVATE_KEY=0x...` (64 hex chars setelah 0x). Pake faucet account.

**"Threshold match: ... node returned ..., env has ..."**  
`.env` thresholds ga match dengan node binary. Pilih:
- Update `.env` cocok dengan `params/ethernova/forks.go`
- Atau rebuild binary dengan threshold yang lo mau

**"Tier == Warm — expected Warm, got Active"**  
Penyebab:
- `WAIT_BUFFER_BLOCKS` terlalu kecil — naikan
- `last_touched` index ga di-update Finalize hook — cek patch di `consensus/lyra2/consensus.go`

**"Submit witness restore tx — receipt status=0 (reverted)"**  
Biasanya:
- Witness generated tapi storage root sudah shift — pastikan node pake `--gcmode archive`
- Cold root capture failed — cek `runStateLifecycle` jalan setelah sweep

**"WarmStateRoot consensus — index divergence"**  
Critical bug. Dua node punya lifecycle index state berbeda. Likely cause: lifecycle hook missing di salah satu Finalize/FinalizeAndAssemble path. Cek BOTH consensus.go entry points panggil `runStateLifecycle`.

## Tuning

Untuk really brutal stress, edit `.env`:
```ini
STRESS_CONTRACTS=200
STRESS_SLOTS_PER_CONTRACT=10
STRESS_CONCURRENT_TX=500
```

That's `200 × 10 = 2000 storage slots` populated dalam satu suite. Pastikan wallet faucet punya ~50 NOVA.
