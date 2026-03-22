# Ethernova v1.2.8 Fork Enforcement & EVM Fork Verification Report

Date: 2026-02-02
Repo: C:\dev\Ethernova\core-geth-src-clean-v1.2.7

## Summary

- Log value correctness (121525 should report enforcement 105000): **PASS**
- Fork logic correctness (SHL/CHAINID/SELFBALANCE pre/post 105000): **PASS** (Go tests + RPC)
- Genesis unchanged (hash + file fingerprint): **PASS**
- RPC verification (Windows): **PASS**
- RPC verification (Linux via WSL): **FAIL** (WSL nested virtualization not supported on this host)

Note: chainId 121525 hex is **0x1dab5** (not 0x1daa5).

---

## 1) Log line source + legacy constant search

### Log line source
```
eth/backend.go:265-273
  enforcement := ethernova.ForkEnforcementDecision(chainID, genesisHash)
  enforcementBlock := ethernova.FormatBlockWithCommas(enforcement.Block)
  log.Info("Ethernova fork enforcement block=%s", enforcementBlock, ...)
```

Evidence (Select-String):
```
eth/backend.go:265: enforcement := ethernova.ForkEnforcementDecision(chainID, genesisHash)
eth/backend.go:267: log.Info(fmt.Sprintf("Ethernova fork enforcement block=%s", enforcementBlock), ...)
```

### Legacy value 138396 only exists in enforcement logic + docs
```
params/ethernova/enforcement.go:13: LegacyForkEnforcementBlock uint64 = 138396
docs/RELEASE-v1.2.8.md:26: legacy chainId 77777 (or legacy genesis) => enforcement 138396
```

Command run:
```
Get-ChildItem -Recurse -File -Include *.go,*.md,*.ps1,*.sh,*.json,*.txt |
  Where-Object { $_.FullName -notmatch "\\dist\\" -and $_.FullName -notmatch "\\logs\\" -and $_.FullName -notmatch "\\.git\\" } |
  Select-String -Pattern "138396"
```

---

## 2) Dataflow / impact analysis

### Where the enforcement value comes from
```
params/ethernova/enforcement.go
- ForkEnforcementDecision(chainID, genesis) chooses:
  * chainId 121525 (+ expected genesis) => 105000
  * legacy 77777 or legacy genesis => 138396
  * unknown chain => 0 (with warning)
```

### Usage of ForkEnforcementDecision
Search results show it is **only used for logging**:
```
eth/backend.go:265: enforcement := ethernova.ForkEnforcementDecision(...)
params/ethernova/enforcement.go:28: func ForkEnforcementDecision(...)
params/ethernova/enforcement_test.go:10: TestForkEnforcementDecisionMainnet
...
```

No other references appear in consensus, fork rules, forkid, or chain config migration logic.
Therefore, the **enforcement value is cosmetic/logging only** and does not affect fork activation or peer compatibility.

---

## 3) Fork activation logic (chain config)

Genesis fields show the fork at 105000:
```
genesis-mainnet.json:425: "constantinopleBlock": 105000
genesis-mainnet.json:426: "petersburgBlock": 105000
genesis-mainnet.json:427: "istanbulBlock": 105000
```

---

## 4) Runtime opcode activation tests (Go)

### EVM opcode pre/post fork checks
Test file: `core/vm/runtime/ethernova_fork_test.go`
- SHL, CHAINID, SELFBALANCE invalid at 104999
- SHL, CHAINID, SELFBALANCE valid at 105000

Command + output:
```
C:\Go\bin\go.exe test ./core/vm/runtime -run Ethernova
ok   github.com/ethereum/go-ethereum/core/vm/runtime 0.065s
```

### Legacy enforcement value tests
Test file: `params/ethernova/enforcement_test.go`
- chainId 121525 + expected genesis => 105000
- legacy chainId 77777 => 138396
- unknown chain => not 138396

Command + output:
```
C:\Go\bin\go.exe test ./params/ethernova -run ForkEnforcementDecision
ok   github.com/ethereum/go-ethereum/params/ethernova (cached)
```

---

## 5) RPC verification (debug_traceCall)

### Windows (PASS)
Command:
```
.\scripts\verify-fork-windows.ps1
```
Output:
```
latest (head=0) SHL: OK
pre-fork 104999 SHL: OK
pre-fork 104999 CHAINID: OK
pre-fork 104999 SELFBALANCE: OK
post-fork 105000 SHL: OK
post-fork 105000 CHAINID: OK
post-fork 105000 SELFBALANCE: OK
clientVersion=Ethernova/v1.2.8/windows-amd64/go1.21.13
chainId=0x1dab5
genesis=0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9
OK: fork verification passed.
```

### Linux via WSL (FAIL - environment)
Command:
```
wsl -d Ubuntu-22.04 -- bash -lc "cd /mnt/c/dev/Ethernova/core-geth-src-clean-v1.2.7 && chmod +x scripts/verify-fork-linux.sh dist/ethernova-v1.2.8-linux-amd64 && ./scripts/verify-fork-linux.sh"
```
Output (failure due to host limitation):
```
wsl: Nested virtualization is not supported on this machine.
```

---

## 6) Genesis unchanged (hash + file fingerprint)

### Embedded genesis hash (from binary)
Command:
```
.\dist\ethernova-v1.2.8-windows-amd64.exe print-genesis
```
Output:
```
expected_genesis_hash=0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9
chain_id=121525
network_id=121525
embedded_genesis_sha256=768f10bf9f77e20d5da970436e283c5a6892c9169a7af6d33c8e8ec506c9957d
```

### File fingerprints match (genesis-mainnet.json == embedded genesis file)
Command:
```
Get-FileHash -Algorithm SHA256 genesis-mainnet.json
Get-FileHash -Algorithm SHA256 params\ethernova\genesis-121525-alloc.json
```
Output:
```
Hash : 768F10BF9F77E20D5DA970436E283C5A6892C9169A7AF6D33C8E8EC506C9957D (both files)
```

### BaseFeeVault (unchanged)
```
genesis-mainnet.json:415: "baseFeeVault": "0x3a38560b66205bb6a31decbcb245450b2f15d4fd"
```

---

## 7) Enforcement log correctness (chain-aware)

Sample log line from local run:
```
"Ethernova fork enforcement block=105,000" chain_id=121,525 genesis=c3812e..c453d9 reason="chain=121525"
```

This confirms 121525 logs 105000, **not** the legacy 138396 value.

---

## 8) If legacy log ever appears for 121525 (fix plan)

Already implemented and protected by tests:
- `params/ethernova/ForkEnforcementDecision` is chain-aware.
- `TestForkEnforcementDecisionMainnet` asserts 121525 => 105000.

Minimal fix (if regression occurs):
1) Ensure `ForkEnforcementDecision` uses (chainId==121525 && genesis==ExpectedGenesisHash) => 105000.
2) Keep legacy 138396 only for chainId 77777 or legacy genesis.
3) Keep `TestForkEnforcementDecisionMainnet` to prevent regression.

---

## Commands Run (raw)

- `Select-String` searches for log source and 138396 (see sections above)
- `C:\Go\bin\go.exe test ./core/vm/runtime -run Ethernova`
- `C:\Go\bin\go.exe test ./params/ethernova -run ForkEnforcementDecision`
- `.\scripts\verify-fork-windows.ps1`
- `wsl -d Ubuntu-22.04 -- bash -lc "... ./scripts/verify-fork-linux.sh"`
- `.\dist\ethernova-v1.2.8-windows-amd64.exe print-genesis`
- `Get-FileHash -Algorithm SHA256 genesis-mainnet.json`
- `Get-FileHash -Algorithm SHA256 params\ethernova\genesis-121525-alloc.json`
