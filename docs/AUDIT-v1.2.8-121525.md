# AUDIT v1.2.8 - Chain 121525 (Ethernova)

Date: 2026-02-02 14:33:37
Repo: C:\dev\Ethernova\core-geth-src-clean-v1.2.7
Expected chainId/networkId: 121525 (hex 0x1dab5)
Expected genesis hash: 0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9

## Repo state (audit only)
`
## main
 M core/vm/runtime/ethernova_fork_test.go
 M scripts/verify-fork-linux.sh
 M scripts/verify-fork-windows.ps1
?? docs/REPORT-v1.2.8-fork-verification.md
`

## PASS/FAIL summary
- Log value correctness (no legacy enforcement on chain 121525): PASS
- Fork activation logic at 105000 (Constantinople/Petersburg/Istanbul): PASS
- Genesis hash unchanged: PASS
- RPC verification (Windows local): PASS
- RPC verification (Linux local): NOT RUN (WSL not available in this environment)

## 1) String searches (evidence)

### Pattern: "Ethernova fork enforcement"
`

  docs\RELEASE-v1.2.8.md:30:
  docs\RELEASE-v1.2.8.md:31:- Windows: `scripts/verify-enforcement-windows.ps1`
  docs\RELEASE-v1.2.8.md:32:- Linux: `scripts/verify-enforcement-linux.sh`
  docs\RELEASE-v1.2.8.md:33:
  docs\RELEASE-v1.2.8.md:34:These scripts also check for the startup log line:
> docs\RELEASE-v1.2.8.md:35:`Ethernova fork enforcement block=105,000`
  docs\RELEASE-v1.2.8.md:36:
  docs\RELEASE-v1.2.8.md:37:## Mandatory Upgrade
  docs\RELEASE-v1.2.8.md:38:
  docs\RELEASE-v1.2.8.md:39:Upgrade **before block 105000**.
  docs\RELEASE-v1.2.8.md:40:
  docs\REPORT-v1.2.8-fork-verification.md:20:### Log line source
  docs\REPORT-v1.2.8-fork-verification.md:21:```
  docs\REPORT-v1.2.8-fork-verification.md:22:eth/backend.go:265-273
  docs\REPORT-v1.2.8-fork-verification.md:23:  enforcement := ethernova.ForkEnforcementDecision(chainID, genesisHash)
  docs\REPORT-v1.2.8-fork-verification.md:24:  enforcementBlock := ethernova.FormatBlockWithCommas(enforcement.Block)
> docs\REPORT-v1.2.8-fork-verification.md:25:  log.Info("Ethernova fork enforcement block=%s", enforcementBlock, ...)
  docs\REPORT-v1.2.8-fork-verification.md:26:```
  docs\REPORT-v1.2.8-fork-verification.md:27:
  docs\REPORT-v1.2.8-fork-verification.md:28:Evidence (Select-String):
  docs\REPORT-v1.2.8-fork-verification.md:29:```
  docs\REPORT-v1.2.8-fork-verification.md:30:eth/backend.go:265: enforcement := 
ethernova.ForkEnforcementDecision(chainID, genesisHash)
> docs\REPORT-v1.2.8-fork-verification.md:31:eth/backend.go:267: log.Info(fmt.Sprintf("Ethernova fork enforcement 
block=%s", enforcementBlock), ...)
  docs\REPORT-v1.2.8-fork-verification.md:32:```
  docs\REPORT-v1.2.8-fork-verification.md:33:
  docs\REPORT-v1.2.8-fork-verification.md:34:### Legacy value 138396 only exists in enforcement logic + docs
  docs\REPORT-v1.2.8-fork-verification.md:35:```
  docs\REPORT-v1.2.8-fork-verification.md:36:params/ethernova/enforcement.go:13: LegacyForkEnforcementBlock uint64 = 
138396
  docs\REPORT-v1.2.8-fork-verification.md:178:
  docs\REPORT-v1.2.8-fork-verification.md:179:## 7) Enforcement log correctness (chain-aware)
  docs\REPORT-v1.2.8-fork-verification.md:180:
  docs\REPORT-v1.2.8-fork-verification.md:181:Sample log line from local run:
  docs\REPORT-v1.2.8-fork-verification.md:182:```
> docs\REPORT-v1.2.8-fork-verification.md:183:"Ethernova fork enforcement block=105,000" chain_id=121,525 
genesis=c3812e..c453d9 reason="chain=121525"
  docs\REPORT-v1.2.8-fork-verification.md:184:```
  docs\REPORT-v1.2.8-fork-verification.md:185:
  docs\REPORT-v1.2.8-fork-verification.md:186:This confirms 121525 logs 105000, **not** the legacy 138396 value.
  docs\REPORT-v1.2.8-fork-verification.md:187:
  docs\REPORT-v1.2.8-fork-verification.md:188:---
  docs\UPGRADE-v1.2.8.md:48:
  docs\UPGRADE-v1.2.8.md:49:Confirm:
  docs\UPGRADE-v1.2.8.md:50:
  docs\UPGRADE-v1.2.8.md:51:- `web3_clientVersion` shows `Ethernova/v1.2.8/...`
  docs\UPGRADE-v1.2.8.md:52:- Fork schedule log shows `fork scheduled at 105000 (Constantinople)`
> docs\UPGRADE-v1.2.8.md:53:- Enforcement log shows `Ethernova fork enforcement block=105,000`
  docs\UPGRADE-v1.2.8.md:54:
  docs\UPGRADE-v1.2.8.md:55:Run the verification script:
  docs\UPGRADE-v1.2.8.md:56:
  docs\UPGRADE-v1.2.8.md:57:**Windows:**
  docs\UPGRADE-v1.2.8.md:58:```
  eth\backend.go:262:	}
  eth\backend.go:263:	log.Info("Chain identity", "chain_id", chainID, "network_id", networkID, "genesis", genesisHash)
  eth\backend.go:264:	if isEthernova {
  eth\backend.go:265:		enforcement := ethernova.ForkEnforcementDecision(chainID, genesisHash)
  eth\backend.go:266:		enforcementBlock := ethernova.FormatBlockWithCommas(enforcement.Block)
> eth\backend.go:267:		log.Info(fmt.Sprintf("Ethernova fork enforcement block=%s", enforcementBlock),
  eth\backend.go:268:			"chain_id", chainID, "genesis", genesisHash, "reason", enforcement.Reason)
  eth\backend.go:269:		if enforcement.Warning != "" {
> eth\backend.go:270:			log.Warn("Ethernova fork enforcement warning", "warning", enforcement.Warning, "chain_id", 
chainID, "genesis", genesisHash)
  eth\backend.go:271:		}
  eth\backend.go:272:		log.Info(fmt.Sprintf("Ethernova EVM fork block=%s enforcement block=%s",
  eth\backend.go:273:			ethernova.FormatBlockWithCommas(ethernova.EVMCompatibilityForkBlock), enforcementBlock),
  eth\backend.go:274:			"chain_id", chainID, "genesis", genesisHash)
  eth\backend.go:275:		if chainID == nil || chainID.Cmp(ethernova.NewChainIDBig) != 0 {
  scripts\verify-enforcement-linux.sh:1:#!/usr/bin/env bash
  scripts\verify-enforcement-linux.sh:2:set -euo pipefail
  scripts\verify-enforcement-linux.sh:3:
  scripts\verify-enforcement-linux.sh:4:ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  scripts\verify-enforcement-linux.sh:5:EXPECTED_GENESIS="0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c
453d9"
> scripts\verify-enforcement-linux.sh:6:EXPECTED_LOG="Ethernova fork enforcement block=105,000"
  scripts\verify-enforcement-linux.sh:7:
  scripts\verify-enforcement-linux.sh:8:BIN="${ETHE_BIN:-$ROOT_DIR/dist/ethernova-v1.2.8-linux-amd64}"
  scripts\verify-enforcement-linux.sh:9:GENESIS="${GENESIS_PATH:-$ROOT_DIR/genesis-mainnet.json}"
  scripts\verify-enforcement-linux.sh:10:PORT="${HTTP_PORT:-8545}"
  scripts\verify-enforcement-linux.sh:11:
  scripts\verify-enforcement-windows.ps1:155:  $genesisHash = $block0.hash
  scripts\verify-enforcement-windows.ps1:156:  if ((Normalize-Hex $genesisHash) -ne (Normalize-Hex 
$ExpectedGenesisHash)) {
  scripts\verify-enforcement-windows.ps1:157:    throw ("unexpected genesis hash: {0}" -f $genesisHash)
  scripts\verify-enforcement-windows.ps1:158:  }
  scripts\verify-enforcement-windows.ps1:159:
> scripts\verify-enforcement-windows.ps1:160:  $expectedLine = "Ethernova fork enforcement block=105,000"
  scripts\verify-enforcement-windows.ps1:161:  $found = $false
  scripts\verify-enforcement-windows.ps1:162:  $logDeadline = (Get-Date).AddSeconds(10)
  scripts\verify-enforcement-windows.ps1:163:  while ((Get-Date) -lt $logDeadline) {
  scripts\verify-enforcement-windows.ps1:164:    if (Test-Path $LogPath) {
  scripts\verify-enforcement-windows.ps1:165:      $match = Select-String -Path @($LogPath, $ErrPath) -Pattern 
$expectedLine -SimpleMatch -ErrorAction SilentlyContinue
`

### Pattern: "138396"
`

  docs\RELEASE-v1.2.8.md:21:## Enforcement block (chain-aware)
  docs\RELEASE-v1.2.8.md:22:
  docs\RELEASE-v1.2.8.md:23:The client derives the fork enforcement block from the active chain identity:
  docs\RELEASE-v1.2.8.md:24:
  docs\RELEASE-v1.2.8.md:25:- chainId **121525** + genesis **0xc3812e...c453d9** => enforcement **105000**
> docs\RELEASE-v1.2.8.md:26:- legacy chainId **77777** (or legacy genesis) => enforcement **138396**
  docs\RELEASE-v1.2.8.md:27:- unknown chains **do not** default to the legacy value
  docs\RELEASE-v1.2.8.md:28:
  docs\RELEASE-v1.2.8.md:29:Verify with:
  docs\RELEASE-v1.2.8.md:30:
  docs\RELEASE-v1.2.8.md:31:- Windows: `scripts/verify-enforcement-windows.ps1`
  docs\REPORT-v1.2.8-fork-verification.md:29:```
  docs\REPORT-v1.2.8-fork-verification.md:30:eth/backend.go:265: enforcement := 
ethernova.ForkEnforcementDecision(chainID, genesisHash)
  docs\REPORT-v1.2.8-fork-verification.md:31:eth/backend.go:267: log.Info(fmt.Sprintf("Ethernova fork enforcement 
block=%s", enforcementBlock), ...)
  docs\REPORT-v1.2.8-fork-verification.md:32:```
  docs\REPORT-v1.2.8-fork-verification.md:33:
> docs\REPORT-v1.2.8-fork-verification.md:34:### Legacy value 138396 only exists in enforcement logic + docs
  docs\REPORT-v1.2.8-fork-verification.md:35:```
> docs\REPORT-v1.2.8-fork-verification.md:36:params/ethernova/enforcement.go:13: LegacyForkEnforcementBlock uint64 = 
138396
> docs\REPORT-v1.2.8-fork-verification.md:37:docs/RELEASE-v1.2.8.md:26: legacy chainId 77777 (or legacy genesis) => 
enforcement 138396
  docs\REPORT-v1.2.8-fork-verification.md:38:```
  docs\REPORT-v1.2.8-fork-verification.md:39:
  docs\REPORT-v1.2.8-fork-verification.md:40:Command run:
  docs\REPORT-v1.2.8-fork-verification.md:41:```
  docs\REPORT-v1.2.8-fork-verification.md:42:Get-ChildItem -Recurse -File -Include *.go,*.md,*.ps1,*.sh,*.json,*.txt |
  docs\REPORT-v1.2.8-fork-verification.md:43:  Where-Object { $_.FullName -notmatch "\\dist\\" -and $_.FullName 
-notmatch "\\logs\\" -and $_.FullName -notmatch "\\.git\\" } |
> docs\REPORT-v1.2.8-fork-verification.md:44:  Select-String -Pattern "138396"
  docs\REPORT-v1.2.8-fork-verification.md:45:```
  docs\REPORT-v1.2.8-fork-verification.md:46:
  docs\REPORT-v1.2.8-fork-verification.md:47:---
  docs\REPORT-v1.2.8-fork-verification.md:48:
  docs\REPORT-v1.2.8-fork-verification.md:49:## 2) Dataflow / impact analysis
  docs\REPORT-v1.2.8-fork-verification.md:51:### Where the enforcement value comes from
  docs\REPORT-v1.2.8-fork-verification.md:52:```
  docs\REPORT-v1.2.8-fork-verification.md:53:params/ethernova/enforcement.go
  docs\REPORT-v1.2.8-fork-verification.md:54:- ForkEnforcementDecision(chainID, genesis) chooses:
  docs\REPORT-v1.2.8-fork-verification.md:55:  * chainId 121525 (+ expected genesis) => 105000
> docs\REPORT-v1.2.8-fork-verification.md:56:  * legacy 77777 or legacy genesis => 138396
  docs\REPORT-v1.2.8-fork-verification.md:57:  * unknown chain => 0 (with warning)
  docs\REPORT-v1.2.8-fork-verification.md:58:```
  docs\REPORT-v1.2.8-fork-verification.md:59:
  docs\REPORT-v1.2.8-fork-verification.md:60:### Usage of ForkEnforcementDecision
  docs\REPORT-v1.2.8-fork-verification.md:61:Search results show it is **only used for logging**:
  docs\REPORT-v1.2.8-fork-verification.md:96:```
  docs\REPORT-v1.2.8-fork-verification.md:97:
  docs\REPORT-v1.2.8-fork-verification.md:98:### Legacy enforcement value tests
  docs\REPORT-v1.2.8-fork-verification.md:99:Test file: `params/ethernova/enforcement_test.go`
  docs\REPORT-v1.2.8-fork-verification.md:100:- chainId 121525 + expected genesis => 105000
> docs\REPORT-v1.2.8-fork-verification.md:101:- legacy chainId 77777 => 138396
> docs\REPORT-v1.2.8-fork-verification.md:102:- unknown chain => not 138396
  docs\REPORT-v1.2.8-fork-verification.md:103:
  docs\REPORT-v1.2.8-fork-verification.md:104:Command + output:
  docs\REPORT-v1.2.8-fork-verification.md:105:```
  docs\REPORT-v1.2.8-fork-verification.md:106:C:\Go\bin\go.exe test ./params/ethernova -run ForkEnforcementDecision
  docs\REPORT-v1.2.8-fork-verification.md:107:ok   github.com/ethereum/go-ethereum/params/ethernova (cached)
  docs\REPORT-v1.2.8-fork-verification.md:181:Sample log line from local run:
  docs\REPORT-v1.2.8-fork-verification.md:182:```
  docs\REPORT-v1.2.8-fork-verification.md:183:"Ethernova fork enforcement block=105,000" chain_id=121,525 
genesis=c3812e..c453d9 reason="chain=121525"
  docs\REPORT-v1.2.8-fork-verification.md:184:```
  docs\REPORT-v1.2.8-fork-verification.md:185:
> docs\REPORT-v1.2.8-fork-verification.md:186:This confirms 121525 logs 105000, **not** the legacy 138396 value.
  docs\REPORT-v1.2.8-fork-verification.md:187:
  docs\REPORT-v1.2.8-fork-verification.md:188:---
  docs\REPORT-v1.2.8-fork-verification.md:189:
  docs\REPORT-v1.2.8-fork-verification.md:190:## 8) If legacy log ever appears for 121525 (fix plan)
  docs\REPORT-v1.2.8-fork-verification.md:191:
  docs\REPORT-v1.2.8-fork-verification.md:193:- `params/ethernova/ForkEnforcementDecision` is chain-aware.
  docs\REPORT-v1.2.8-fork-verification.md:194:- `TestForkEnforcementDecisionMainnet` asserts 121525 => 105000.
  docs\REPORT-v1.2.8-fork-verification.md:195:
  docs\REPORT-v1.2.8-fork-verification.md:196:Minimal fix (if regression occurs):
  docs\REPORT-v1.2.8-fork-verification.md:197:1) Ensure `ForkEnforcementDecision` uses (chainId==121525 && 
genesis==ExpectedGenesisHash) => 105000.
> docs\REPORT-v1.2.8-fork-verification.md:198:2) Keep legacy 138396 only for chainId 77777 or legacy genesis.
  docs\REPORT-v1.2.8-fork-verification.md:199:3) Keep `TestForkEnforcementDecisionMainnet` to prevent regression.
  docs\REPORT-v1.2.8-fork-verification.md:200:
  docs\REPORT-v1.2.8-fork-verification.md:201:---
  docs\REPORT-v1.2.8-fork-verification.md:202:
  docs\REPORT-v1.2.8-fork-verification.md:203:## Commands Run (raw)
  docs\REPORT-v1.2.8-fork-verification.md:204:
> docs\REPORT-v1.2.8-fork-verification.md:205:- `Select-String` searches for log source and 138396 (see sections above)
  docs\REPORT-v1.2.8-fork-verification.md:206:- `C:\Go\bin\go.exe test ./core/vm/runtime -run Ethernova`
  docs\REPORT-v1.2.8-fork-verification.md:207:- `C:\Go\bin\go.exe test ./params/ethernova -run ForkEnforcementDecision`
  docs\REPORT-v1.2.8-fork-verification.md:208:- `.\scripts\verify-fork-windows.ps1`
  docs\REPORT-v1.2.8-fork-verification.md:209:- `wsl -d Ubuntu-22.04 -- bash -lc "... ./scripts/verify-fork-linux.sh"`
  docs\REPORT-v1.2.8-fork-verification.md:210:- `.\dist\ethernova-v1.2.8-windows-amd64.exe print-genesis`
  params\ethernova\enforcement.go:8:	"github.com/ethereum/go-ethereum/common"
  params\ethernova\enforcement.go:9:)
  params\ethernova\enforcement.go:10:
  params\ethernova\enforcement.go:11:const (
  params\ethernova\enforcement.go:12:	LegacyChainID              uint64 = 77777
> params\ethernova\enforcement.go:13:	LegacyForkEnforcementBlock uint64 = 138396
  params\ethernova\enforcement.go:14:)
  params\ethernova\enforcement.go:15:
  params\ethernova\enforcement.go:16:const LegacyGenesisHashHex = 
"0xc67bd6160c1439360ab14abf7414e8f07186f3bed095121df3f3b66fdc6c2183"
  params\ethernova\enforcement.go:17:
  params\ethernova\enforcement.go:18:var LegacyGenesisHash = common.HexToHash(LegacyGenesisHashHex)
`

### Pattern: "77777"
`
--- cmd\\devp2p\\internal\\ethtest\\testdata\\headstate.json ---

  cmd\devp2p\internal\ethtest\testdata\headstate.json:2645:      "key": 
"0x720f25b62fc39426f70eb219c9dd481c1621821c8c0fa5367a1df6e59e3edf59"
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2646:    },
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2647:    "0x7fd02a3bb5d5926d4981efbf63b66de2a7b1aa63": {
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2648:      "balance": "0",
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2649:      "nonce": 1,
> cmd\devp2p\internal\ethtest\testdata\headstate.json:2650:      "root": 
"0x7bf542bdaff5bfe3d33c26a88777773b5e525461093c36acb0dab591a319e509",
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2651:      "codeHash": 
"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2652:      "storage": {
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2653:        
"0x0000000000000000000000000000000000000000000000000000000000000032": "32",
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2654:        
"0x0000000000000000000000000000000000000000000000000000000000000033": "33",
  cmd\devp2p\internal\ethtest\testdata\headstate.json:2655:        
"0x0000000000000000000000000000000000000000000000000000000000000034": "34"

--- consensus\\ethash\\algorithm.go ---

  consensus\ethash\algorithm.go:1137:	263716672, 263847872, 263978944, 264108608, 264241088, 264371648,
  consensus\ethash\algorithm.go:1138:	264501184, 264632768, 264764096, 264895936, 265024576, 265158464,
  consensus\ethash\algorithm.go:1139:	265287488, 265418432, 265550528, 265681216, 265813312, 265943488,
  consensus\ethash\algorithm.go:1140:	266075968, 266206144, 266337728, 266468032, 266600384, 266731072,
  consensus\ethash\algorithm.go:1141:	266862272, 266993344, 267124288, 267255616, 267386432, 267516992,
> consensus\ethash\algorithm.go:1142:	267648704, 267777728, 267910592, 268040512, 268172096, 268302784,
  consensus\ethash\algorithm.go:1143:	268435264, 268566208, 268696256, 268828096, 268959296, 269090368,
  consensus\ethash\algorithm.go:1144:	269221312, 269352256, 269482688, 269614784, 269745856, 269876416,
  consensus\ethash\algorithm.go:1145:	270007616, 270139328, 270270272, 270401216, 270531904, 270663616,
  consensus\ethash\algorithm.go:1146:	270791744, 270924736, 271056832, 271186112, 271317184, 271449536,
  consensus\ethash\algorithm.go:1147:	271580992, 271711936, 271843136, 271973056, 272105408, 272236352,

--- core\\vm\\testdata\\precompiles\\blsG1MultiExp.json ---

  core\vm\testdata\precompiles\blsG1MultiExp.json:229:    "Name": "matter_g1_multiexp_29",
  core\vm\testdata\precompiles\blsG1MultiExp.json:230:    "Gas": 64128,
  core\vm\testdata\precompiles\blsG1MultiExp.json:231:    "NoBenchmark": false
  core\vm\testdata\precompiles\blsG1MultiExp.json:232:  },
  core\vm\testdata\precompiles\blsG1MultiExp.json:233:  {
> core\vm\testdata\precompiles\blsG1MultiExp.json:234:    "Input": "0000000000000000000000000000000004663e332c105837eeb
fb9ecaf524a8f7f4d651f3eeae6909824eaaa6250c9f7fc212f98c6b3d4c08c5198477f240a8300000000000000000000000000000000057144a857
8437c9a10a7801fb179e417e9bbe1b85e9dd8e2208943978cdd77a8345d682ba83950e174c6cd39c9eb936a57b2c351a7946a20cbae1fd789ecc5f7
7376b09e911749831e9b5680185b1530000000000000000000000000000000017c44ab586ecd185de616da02f99ee799487b32baf2470871865baa2
b2e3ca20f61e6c82d741853b71c5578199d46afb000000000000000000000000000000000c77154ab5f0ba817b30672367bf1e19f9e53a95d7fcc45
65f82f604a07d5eedba2182cf1bcca2371af4d1bd09146cb98fbff9f8ac4ad10718d46a857ba28f182263bf2d13c8b6a00902af737dea5616000000
0000000000000000000000000002df334ee40a5aa144d3727ec6c19d8dac476c01935e7ddbfc164112e35cca9180ffdae5e56f1fb31741c327b5733
d6b0000000000000000000000000000000006c1721530a765ce427eacc4e5679c42591d5d1443f0a1bca8a87dd19d6a33b731db6561c50a35511735
324c5f402858b061de16f4f609c6947733b58c6444fa9549721fd9a2459652e8e4b8c69b5d6100000000000000000000000000000000016682e225b
46618ff794f2da02a82e40193289c9df4ed6985b4daca3e9ce9ac6e8ce84a3fd6776119ae1a2e84f62e73000000000000000000000000000000000e
383f55e44fa8528e80fdf391f2804f7b7f3367e0db07b78647e9ceeba5fb151a5b867bafb2d9c07a6a572ee71c2714355ed5b57b28451ad98fbacd5
ae87551b7304e4ef5cf7b7dc443a66432406f9a00000000000000000000000000000000176de8a3ee21e803ec6fd42f7f297daeaf1541c08c5c359e
286ba65b78d7c31a0a630a2c73d2e886cfcb289783f30cf20000000000000000000000000000000010645db8d7d42e004c4f76bb2fe8b99a3177624
ce0c1f465e67f3767bb57ca80ebadb12fba65bd021106e17adcd8553430b6eeb01874ff4b0fb07dc9f23d8e45455c1480eba7fb3033942214e85a77
200000000000000000000000000000000006c151767d1066f9567ed86f7759a6f425a9a130a4530a2dec0913e4efe2485dd4b0105f453e90bf27cbe
ee5d0482af40000000000000000000000000000000019a081fb1fe2893f1919628cb8a3b332ef072971fe6ea7fbaf79d327440274a589045db5d3f0
6d6dc32d6bc7038c528b89a697a0e8d2cf512edd2a3c3df354eb30a3eaf697779dd9270234b367c2b5ff000000000000000000000000000000000d1
9d55d1fa04f886078bba50e09ece3a394f3413745785c16d17c5936941345e42e4ac50cba055d79f2d813c69e0b2000000000000000000000000000
0000000ba513864132f44be3056d3d3d1fe8d10b8be954e785e3d07f816875a3454fb6d44c1a6da8c9644648b46dc7d8a0b67120b72463d54ac1d8f
1b3f56f0f98861768b05d5174cf1883dd8eb0410420d5620000000000000000000000000000000019cb4ac7844effff88b242db9908bd8773d91cbd
8e076127493c548350bb9f8230d57a3e9c4e4b212e5686bee925d80a00000000000000000000000000000000021e94fbe9881b2f5ce2e8d777a3333
6fa21c24818cc1b6b699f0bf5cf1f22d7b9fe85be05d09509b88391f78eadf14e3de7997113708f9d092836c2b0b59abf710d8401baea6de73ee068
9436f035fe000000000000000000000000000000000c6429ad7548acf43bd9e7fd9ccbb09b5b9b4474937bcca985a2d00c62cc8b72e07e725a5d447
e2a92a6bb9fff0c50c100000000000000000000000000000000135ae562ac2225bdfcbed36817c8deadf892da1f8982f4bf53271320bb4e70202212
8dfbf9e48fc6623648878020c1a67fc3d0560432dbb721f8a0610f0db31dfdfea8cd5ebe8da3fe3b8ac5358dd440000000000000000000000000000
0000004a813c60a1988f7983f6ac644a66369153319e3bceda90fcef6fdf3e53ceb04b2c5d240cc65aaeb2530e8931f1a962b000000000000000000
00000000000000141411938210cef5576dacba6d521bc46b13ce9c1f2a9aa41a0e9b56639995b69b6198f2a406ca5e471cb0a48233985ff0b271f02
031a126f8632e30d8b17cc5b57de7b8b873e0971ff392d4246a40f400000000000000000000000000000000041855bc5957b8649451b7d91ef58fe8
e0770b113ea3009815e60cb36c9b7ab797b4448d3747fa9b64b7fb50af906b6d00000000000000000000000000000000048f78b763a88fb7122e117
ea4946a631be83b5ae456f0c77a16f3f2b546802bea7117eb27e23a5db65d616966bf2630f8b5c136aa5e2d670edcfb5bee9ff6095d85a332ad5576
3fe1e5e8babd145c070000000000000000000000000000000003ca70d52cbfe2c097c17bd300f4baba1d03951c6dae613bfbbd53f68598a71d80a28
5af1a16365b5b82991599ae8fd0000000000000000000000000000000000ff454d717d8518415f23ced167ad7ad1ec76c437e29fef81b5604e8bc62
8b320fa39c192f32aa6201c2b5b4035cfddc285193e7c10646a4601787edfad3d76e19d5b013a0a954873d92bd5293d325820000000000000000000
0000000000000098363ac967c6800b28c28afe92c1379574ec11e0585a0319273aaa6b92322563ad56144437569f3b9cd70ba9e7f9e030000000000
000000000000000000000006e4aa226ef031c07150bb231046f36b8ced6b795b3e3f25f707435abc214f14e0c420c699f9c880e8d647ba85d467ef3
5bb2175fff61894ccbb69d90375df627e925f1ac430a349e75580dd39546e440000000000000000000000000000000001ced5366374fd923b3196d8
f6e35900b80d01eeaa6ac41bf7d05d1fb7d47810eb8cd2d1ab793126edbe863be4c1224200000000000000000000000000000000010b27a94ae8413
494e0560a10ac71554ff502be7e86cd9760b0d4ea7d1df926cf7ff1661b7902fb93ebcfd1542619caa25856e5fb9547c48d41783bf2cd13493a1fd7
1e56b9c7e62af84a1f6cdae1c800000000000000000000000000000000120ffc413256888669dce253043ace9a8c924f2996d73ef3a64d76d88dab4
15c870071a22b97da222361dc02d91cb25e000000000000000000000000000000000940f2259f4fadc3bfbed20ed2b80bdd86f30a846d6167661339
e15548f6e57030fcd0be99496fa406a2d025077a4a4e1155c0b9c4185025310e8020eb52abb6f2f1780da15e4ba81f3c9a88ed1b4a6400000000000
00000000000000000000003ea26434b5bc703c242cc5e84e17be5c7777758f0b232feccef6d200db9a03f10df46cf0eead48064f8dbbccccc336900
0000000000000000000000000000000649df5d665a64565079201123e954e78f07177739d082c2bd0aabddcc13f9fec6ef082a1348a369e446b8218
1e52aadc5610b2707ce84ce67e82d5c0e5f5cd2c90925aefc1e39468ca86475012df045",
  core\vm\testdata\precompiles\blsG1MultiExp.json:235:    "Expected": "00000000000000000000000000000000110fac33d46271da
f3924995a4798b3f62c79562d3b44f736b91add9f2af779a614d4b12a9c0d7c60bcb1f104b35474c000000000000000000000000000000001592121
fbb147085613d1b647cb0e4a7b895bfd4e5391b45bcb287975bbf0e5218078d3e88f8383a506550ae07c9d167",
  core\vm\testdata\precompiles\blsG1MultiExp.json:236:    "Name": "matter_g1_multiexp_30",
  core\vm\testdata\precompiles\blsG1MultiExp.json:237:    "Gas": 64128,
  core\vm\testdata\precompiles\blsG1MultiExp.json:238:    "NoBenchmark": false
  core\vm\testdata\precompiles\blsG1MultiExp.json:239:  },

--- core\\vm\\testdata\\precompiles\\blsG2Mul.json ---

  core\vm\testdata\precompiles\blsG2Mul.json:594:    "Gas": 55000,
  core\vm\testdata\precompiles\blsG2Mul.json:595:    "NoBenchmark": false
  core\vm\testdata\precompiles\blsG2Mul.json:596:  },
  core\vm\testdata\precompiles\blsG2Mul.json:597:  {
  core\vm\testdata\precompiles\blsG2Mul.json:598:    "Input": "000000000000000000000000000000000598e111dcfeaaae66d1522b
e2a21131350577253a3f33bdd74a04b0bfba2940e73b62fefa8f0c34c4aa91b633f6bdfd0000000000000000000000000000000017fefff7d94afbe
ceb33714e9b5480c3a2f3eabf9d7f6e8507ae54cb65f69b21cd7d04d23f24e3a272c589f572b91864000000000000000000000000000000001652e3
f5a99ba8dfbcd1f90de955ef527947642054be603c1b84b24bebb579b78e2a0be426ec21d32783a0e55f0178dc00000000000000000000000000000
0000a6c9ec91e8bc86ab198416cbc76239f0ac0b903f40310ee1f2066b01b08191538ca913c2736f53f23ef37fea13d52756e0512ecbc5a1b02ab19
bc9bee4d3d9c721278e07b7a6e389c4d6443232a4035",
> core\vm\testdata\precompiles\blsG2Mul.json:599:    "Expected": "0000000000000000000000000000000002a0214be95f020c70221
fb4fb6856af7ce3845a4b607340f85127b52f8a204efcd94a152835860a4ddeef84946671b1000000000000000000000000000000001767777740a9
922a91c39a36e2cdfcd544df902b31812ffc88418dab7321f73406ab142055b5bb264c187f2d4f2d6f9d00000000000000000000000000000000026
e6941364c74997506df0f9fbe6b2769839e8b7c7293f4e63d13bd7bee90ff779cf82adc2f23c569d1e13826cdb0e400000000000000000000000000
0000001618ab2ffd4b823b9c9776baf849641240109b7a4c4e9269f3df69a06f85a777cb4463b456023b7001adac93243c26f5",
  core\vm\testdata\precompiles\blsG2Mul.json:600:    "Name": "matter_g2_mul_81",
  core\vm\testdata\precompiles\blsG2Mul.json:601:    "Gas": 55000,
  core\vm\testdata\precompiles\blsG2Mul.json:602:    "NoBenchmark": false
  core\vm\testdata\precompiles\blsG2Mul.json:603:  },
  core\vm\testdata\precompiles\blsG2Mul.json:604:  {

--- crypto\\blake2b\\blake2b_test.go ---

  crypto\blake2b\blake2b_test.go:844:	"ecaa6e999ef355a0768730edb835db411829a3764f79d764bb5682af6d00f51b313e017b83fffe2e
332cd4a3de0a81d6a52084d5748346a1f81eb9b183ff6d93d05edc00e938d001c90872dfe234e8dd085f639af168af4a07e18f1c56ca6c7c1addffc
4a70eb4660666dda0321636c3f83479ad3b64e23d749620413a2ecdcc52ad4e6e63f2b817ce99c15b5d2da3792721d7158297cce65e0c04fe810d7e
2434b969e4c7892b3840623e153576356e9a696fd9e7a801c25de621a7849da3f99158d3d09bf039f43c510c8ffb00fa3e9a3c12d2c8062dd25b8da
be53d8581e30427e81c3dfc2d455352487e1255",
  crypto\blake2b\blake2b_test.go:845:	"23a3fe80e3636313fdf922a1359514d9f31775e1adf24285e8001c04dbce866df055edf25b506e18
953492a173ba5aa0c1ec758123406a97025ba9b6b7a97eb14734424d1a7841ec0eaeba0051d6e9734263bea1af9895a3b8c83d8c854da2ae7832bdd
7c285b73f8113c3821cced38b3656b4e6369a9f8327cd368f04128f1d78b6b4260f55995277feffa15e34532cd0306c1f47354667c17018ee012a79
1af2dbbc7afc92c388008c601740cccbbe66f1eb06ea657e9d478066c2bd2093ab62cd94abadc002722f50968e8acf361658fc64f50685a5b1b0048
88b3b4f64a4ddb67bec7e4ac64c9ee8deeda896b9",
  crypto\blake2b\blake2b_test.go:846:	"758f3567cd992228386a1c01930f7c52a9dcce28fdc1aaa54b0fed97d9a54f1df805f31bac12d559
e90a2063cd7df8311a148f6904f78c5440f75e49877c0c0855d59c7f7ee52837e6ef3e54a568a7b38a0d5b896e298c8e46a56d24d8cabda8aeff85a
622a3e7c87483ba921f34156defd185f608e2241224286e38121a162c2ba7604f68484717196f6628861a948180e8f06c6cc1ec66d032cf8d16da03
9cd74277cde31e535bc1692a44046e16881c954af3cd91dc49b443a3680e4bc42a954a46ebd1368b1398edd7580f935514b15c7fbfa9b40048a3512
2283af731f5e460aa85b66e65f49a9d158699bd2870",
  crypto\blake2b\blake2b_test.go:847:	"fe511e86971cea2b6af91b2afa898d9b067fa71780790bb409189f5debe719f405e16acf7c4306a6
e6ac5cd535290efe088943b9e6c5d25bfc508023c1b105d20d57252fee8cdbddb4d34a6ec2f72e8d55be55afcafd2e922ab8c31888bec4e816d04f0
b2cd23df6e04720969c5152b3563c6da37e4608554cc7b8715bc10aba6a2e3b6fbcd35408df0dd73a9076bfad32b741fcdb0edfb563b3f753508b9b
26f0a91673255f9bcda2b9a120f6bfa0632b6551ca517d846a747b66ebda1b2170891ece94c19ce8bf682cc94afdf0053fba4e4f0530935c07cdd6f
879c999a8c4328ef6d3e0a37974a230ada83910604337",
  crypto\blake2b\blake2b_test.go:848:	"a6024f5b959698c0de45f4f29e1803f99dc8112989c536e5a1337e281bc856ff721e986de183d7b0
ea9eb61166830ae5d6d6bc857dc833ff189b52889b8e2bd3f35b4937624d9b36dc5f19db44f0772508029784c7dac9568d28609058bc437e2f79f95
b12307d8a8fb042d7fd6ee910a9e8df609ede3283f958ba918a9925a0b1d0f9f9f232062315f28a52cbd60e71c09d83e0f6600f508f0ae8ad7642c0
80ffc618fcd2314e26f67f1529342569f6df37017f7e3b2dac32ad88d56d175ab22205ee7e3ee94720d76933a21132e110fefbb0689a3adbaa4c685
f43652136d09b3a359b5c671e38f11915cb5612db2ae294",
> crypto\blake2b\blake2b_test.go:849:	"af6de0e227bd78494acb559ddf34d8a7d55a03912384831be21c38376f39cda8a864aff7a48aed75
8f6bdf777779a669068a75ce82a06f6b3325c855ed83daf5513a078a61f7dc6c1622a633367e5f3a33e765c8ec5d8d54f48494006fdbf8922063e53
40013e312871b7f8f8e5ea439c0d4cb78e2f19dd11f010729b692c65dd0d347f0ce53de9d849224666ea2f6487f1c6f953e8f9dbfd3d6de291c3e9d
045e633cfd83c89d2f2327d0b2f31f72ac1604a3db1febc5f22cad08153278047210cc2894582c251a014c652e3951593e70e52a5d7451be8924b64
f85c8247dab6268d24710b39fc1c07b4ac829fbda34ed79b5",
  crypto\blake2b\blake2b_test.go:850:	"d7314e8b1ff82100b8f5870da62b61c31ab37ace9e6a7b6f7d294571523783c1fdedcbc00dd487dd
6f848c34aab493507d07071b5eb59d1a2346068c7f356755fbde3d2cab67514f8c3a12d6ff9f96a977a9ac9263491bd33122a904da5386b943d35a6
ba383932df07f259b6b45f69e9b27b4ca124fb3ae143d709853eed86690bc2754d5f8865c355a44b5279d8eb31cdc00f7407fb5f5b34edc57fc7ace
943565da2222dc80632ccf42f2f125ceb19714ea964c2e50603c9f8960c3f27c2ed0e18a559931c4352bd7422109a28c5e145003f55c9b7c664fdc9
85168868950396eaf6fefc7b73d815c1aca721d7c67da632925",
  crypto\blake2b\blake2b_test.go:851:	"2928b55c0e4d0f5cb4b60af59e9a702e3d616a8cf427c8bb03981fb8c29026d8f7d89161f36c1165
4f9a5e8ccb703595a58d671ecdc22c6a784abe363158682be4643002a7da5c9d268a30ea9a8d4cc24f562ab59f55c2b43af7dbcecc7e5ebe7494e82
d74145a1e7d442125eb0431c5ea0939b27afa47f8ca97849f341f707660c7fbe49b7a0712fbcb6f7562ae2961425f27c7779c7534ecdeb8047ff3cb
89a25159f3e1cefe42f9ef16426241f2c4d62c11d7ac43c4500dfcd184436bb4ef33260366f875230f26d81613c334dbda4736ba9d1d2966502914e
c01bbe72d885606ec11da7a2cb01b29d35eebedbb0ecc73ed6c35",
  crypto\blake2b\blake2b_test.go:852:	"fd993f50e8a68c7b2c7f87511ce65b93c0aa94dcbdf2c9cca93816f0f3b2ab34c62c586fc507b490
0a34cf9d0517e0fe10a89d154c5419c1f5e38de00e8834fe3dc1032abdeb10729a81655a69a12856a78ca6e12110580de879b086fd6608726541cfa
9616326bdd36064bc0d1e5f9c93b41278bff6a13b2494b81e238c0c45aea1b07d855e8f3fe1478e373bd9d3957cf8a5e5b9003386793d994c7c575c
ff2322e2428cbbaa4f47560316ae3354a7478842ff7cc5dcbacb6e871e72b36f06d63a9aaeb9044cfb7974afdc238a5816f537dcf33ee40b4e1a5eb
3cff2402b46d548264e133008d284f11b7e4e450bc3c5ff9f79b9c4",
  crypto\blake2b\blake2b_test.go:853:	"8df21892f5fc303b0de4adef1970186db6fe71bb3ea3094922e13afcfabf1d0be009f36d6f6310c5
f9fda51f1a946507a055b645c296370440e5e83d8e906a2fb51f2b42de8856a81a4f28a73a8825c68ea08e5e366730bce8047011cb7d6d9be8c6f42
11308fad21856284d5bc47d199988e0abf5badf8693ceeed0a2d98e8ae94b7775a42925edb1f697ffbd8e806af23145054a85e071819cca4cd48875
290ca65e5ee72a9a54ff9f19c10ef4adaf8d04c9a9afcc73853fc128bbebc61f78702787c966ca6e1b1a0e4dab646acdfcd3c6bf3e5cfbec5ebe3e0
6c8abaa1de56e48421d87c46b5c78030afcafd91f27e7d7c85eb4872b",
  crypto\blake2b\blake2b_test.go:854:	"48ec6ec520f8e593d7b3f653eb15553de246723b81a6d0c3221aaa42a37420fba98a23796338dff5
f845dce6d5a449be5ecc1887356619270461087e08d05fb60433a83d7bd00c002b09ea210b428965124b9b27d9105a71c826c1a2491cfd60e4cfa86
c2da0c7100a8dc1c3f2f94b280d54e01e043acf0e966200d9fa8a41daf3b9382820786c75cadbb8841a1b2be5b6cbeb64878e4a231ae063a99b4e23
08960ef0c8e2a16bb3545cc43bdf171493fb89a84f47e7973dc60cf75aeeca71e0a7ebe17d161d4fb9fe009941cc438f16a5bae6c99fcad08cac486
eb2a48060b023d8730bf1d82fe60a2f036e6f52a5bff95f43bbe088933f",

--- crypto\\kzg4844\\trusted_setup.json ---

  crypto\kzg4844\trusted_setup.json:3281:    
"0xa386420b738aba2d7145eb4cba6d643d96bda3f2ca55bb11980b318d43b289d55a108f4bc23a9606fb0bccdeb3b3bb30",
  crypto\kzg4844\trusted_setup.json:3282:    
"0x847020e0a440d9c4109773ecca5d8268b44d523389993b1f5e60e541187f7c597d79ebd6e318871815e26c96b4a4dbb1",
  crypto\kzg4844\trusted_setup.json:3283:    
"0xa530aa7e5ca86fcd1bec4b072b55cc793781f38a666c2033b510a69e110eeabb54c7d8cbcb9c61fee531a6f635ffa972",
  crypto\kzg4844\trusted_setup.json:3284:    
"0x87364a5ea1d270632a44269d686b2402da737948dac27f51b7a97af80b66728b0256547a5103d2227005541ca4b7ed04",
  crypto\kzg4844\trusted_setup.json:3285:    
"0x8816fc6e16ea277de93a6d793d0eb5c15e9e93eb958c5ef30adaf8241805adeb4da8ce19c3c2167f971f61e0b361077d",
> crypto\kzg4844\trusted_setup.json:3286:    
"0x8836a72d301c42510367181bb091e4be377777aed57b73c29ef2ce1d475feedd7e0f31676284d9a94f6db01cc4de81a2",
  crypto\kzg4844\trusted_setup.json:3287:    
"0xb0d9d8b7116156d9dde138d28aa05a33e61f8a85839c1e9071ccd517b46a5b4b53acb32c2edd7150c15bc1b4bd8db9e3",
  crypto\kzg4844\trusted_setup.json:3288:    
"0xae931b6eaeda790ba7f1cd674e53dc87f6306ff44951fa0df88d506316a5da240df9794ccbd7215a6470e6b31c5ea193",
  crypto\kzg4844\trusted_setup.json:3289:    
"0x8c6d5bdf87bd7f645419d7c6444e244fe054d437ed1ba0c122fde7800603a5fadc061e5b836cb22a6cfb2b466f20f013",
  crypto\kzg4844\trusted_setup.json:3290:    
"0x90d530c6d0cb654999fa771b8d11d723f54b8a8233d1052dc1e839ea6e314fbed3697084601f3e9bbb71d2b4eaa596df",
  crypto\kzg4844\trusted_setup.json:3291:    
"0xb0d341a1422588c983f767b1ed36c18b141774f67ef6a43cff8e18b73a009da10fc12120938b8bba27f225bdfd3138f9",

--- docs\\RELEASE-v1.2.8.md ---

  docs\RELEASE-v1.2.8.md:21:## Enforcement block (chain-aware)
  docs\RELEASE-v1.2.8.md:22:
  docs\RELEASE-v1.2.8.md:23:The client derives the fork enforcement block from the active chain identity:
  docs\RELEASE-v1.2.8.md:24:
  docs\RELEASE-v1.2.8.md:25:- chainId **121525** + genesis **0xc3812e...c453d9** => enforcement **105000**
> docs\RELEASE-v1.2.8.md:26:- legacy chainId **77777** (or legacy genesis) => enforcement **138396**
  docs\RELEASE-v1.2.8.md:27:- unknown chains **do not** default to the legacy value
  docs\RELEASE-v1.2.8.md:28:
  docs\RELEASE-v1.2.8.md:29:Verify with:
  docs\RELEASE-v1.2.8.md:30:
  docs\RELEASE-v1.2.8.md:31:- Windows: `scripts/verify-enforcement-windows.ps1`

--- docs\\REPORT-v1.2.8-fork-verification.md ---

  docs\REPORT-v1.2.8-fork-verification.md:32:```
  docs\REPORT-v1.2.8-fork-verification.md:33:
  docs\REPORT-v1.2.8-fork-verification.md:34:### Legacy value 138396 only exists in enforcement logic + docs
  docs\REPORT-v1.2.8-fork-verification.md:35:```
  docs\REPORT-v1.2.8-fork-verification.md:36:params/ethernova/enforcement.go:13: LegacyForkEnforcementBlock uint64 = 
138396
> docs\REPORT-v1.2.8-fork-verification.md:37:docs/RELEASE-v1.2.8.md:26: legacy chainId 77777 (or legacy genesis) => 
enforcement 138396
  docs\REPORT-v1.2.8-fork-verification.md:38:```
  docs\REPORT-v1.2.8-fork-verification.md:39:
  docs\REPORT-v1.2.8-fork-verification.md:40:Command run:
  docs\REPORT-v1.2.8-fork-verification.md:41:```
  docs\REPORT-v1.2.8-fork-verification.md:42:Get-ChildItem -Recurse -File -Include *.go,*.md,*.ps1,*.sh,*.json,*.txt |
  docs\REPORT-v1.2.8-fork-verification.md:51:### Where the enforcement value comes from
  docs\REPORT-v1.2.8-fork-verification.md:52:```
  docs\REPORT-v1.2.8-fork-verification.md:53:params/ethernova/enforcement.go
  docs\REPORT-v1.2.8-fork-verification.md:54:- ForkEnforcementDecision(chainID, genesis) chooses:
  docs\REPORT-v1.2.8-fork-verification.md:55:  * chainId 121525 (+ expected genesis) => 105000
> docs\REPORT-v1.2.8-fork-verification.md:56:  * legacy 77777 or legacy genesis => 138396
  docs\REPORT-v1.2.8-fork-verification.md:57:  * unknown chain => 0 (with warning)
  docs\REPORT-v1.2.8-fork-verification.md:58:```
  docs\REPORT-v1.2.8-fork-verification.md:59:
  docs\REPORT-v1.2.8-fork-verification.md:60:### Usage of ForkEnforcementDecision
  docs\REPORT-v1.2.8-fork-verification.md:61:Search results show it is **only used for logging**:
  docs\REPORT-v1.2.8-fork-verification.md:96:```
  docs\REPORT-v1.2.8-fork-verification.md:97:
  docs\REPORT-v1.2.8-fork-verification.md:98:### Legacy enforcement value tests
  docs\REPORT-v1.2.8-fork-verification.md:99:Test file: `params/ethernova/enforcement_test.go`
  docs\REPORT-v1.2.8-fork-verification.md:100:- chainId 121525 + expected genesis => 105000
> docs\REPORT-v1.2.8-fork-verification.md:101:- legacy chainId 77777 => 138396
  docs\REPORT-v1.2.8-fork-verification.md:102:- unknown chain => not 138396
  docs\REPORT-v1.2.8-fork-verification.md:103:
  docs\REPORT-v1.2.8-fork-verification.md:104:Command + output:
  docs\REPORT-v1.2.8-fork-verification.md:105:```
  docs\REPORT-v1.2.8-fork-verification.md:106:C:\Go\bin\go.exe test ./params/ethernova -run ForkEnforcementDecision
  docs\REPORT-v1.2.8-fork-verification.md:193:- `params/ethernova/ForkEnforcementDecision` is chain-aware.
  docs\REPORT-v1.2.8-fork-verification.md:194:- `TestForkEnforcementDecisionMainnet` asserts 121525 => 105000.
  docs\REPORT-v1.2.8-fork-verification.md:195:
  docs\REPORT-v1.2.8-fork-verification.md:196:Minimal fix (if regression occurs):
  docs\REPORT-v1.2.8-fork-verification.md:197:1) Ensure `ForkEnforcementDecision` uses (chainId==121525 && 
genesis==ExpectedGenesisHash) => 105000.
> docs\REPORT-v1.2.8-fork-verification.md:198:2) Keep legacy 138396 only for chainId 77777 or legacy genesis.
  docs\REPORT-v1.2.8-fork-verification.md:199:3) Keep `TestForkEnforcementDecisionMainnet` to prevent regression.
  docs\REPORT-v1.2.8-fork-verification.md:200:
  docs\REPORT-v1.2.8-fork-verification.md:201:---
  docs\REPORT-v1.2.8-fork-verification.md:202:
  docs\REPORT-v1.2.8-fork-verification.md:203:## Commands Run (raw)

--- params\\ethernova\\enforcement.go ---

  params\ethernova\enforcement.go:7:
  params\ethernova\enforcement.go:8:	"github.com/ethereum/go-ethereum/common"
  params\ethernova\enforcement.go:9:)
  params\ethernova\enforcement.go:10:
  params\ethernova\enforcement.go:11:const (
> params\ethernova\enforcement.go:12:	LegacyChainID              uint64 = 77777
  params\ethernova\enforcement.go:13:	LegacyForkEnforcementBlock uint64 = 138396
  params\ethernova\enforcement.go:14:)
  params\ethernova\enforcement.go:15:
  params\ethernova\enforcement.go:16:const LegacyGenesisHashHex = 
"0xc67bd6160c1439360ab14abf7414e8f07186f3bed095121df3f3b66fdc6c2183"
  params\ethernova\enforcement.go:17:
  params\ethernova\enforcement.go:46:		return decision
  params\ethernova\enforcement.go:47:	}
  params\ethernova\enforcement.go:48:
  params\ethernova\enforcement.go:49:	if chainID != nil && chainIDValue == LegacyChainID {
  params\ethernova\enforcement.go:50:		decision.Block = LegacyForkEnforcementBlock
> params\ethernova\enforcement.go:51:		decision.Reason = "legacy chainId=77777"
  params\ethernova\enforcement.go:52:		return decision
  params\ethernova\enforcement.go:53:	}
  params\ethernova\enforcement.go:54:
  params\ethernova\enforcement.go:55:	if genesis == LegacyGenesisHash {
  params\ethernova\enforcement.go:56:		decision.Block = LegacyForkEnforcementBlock

--- params\\alloc_mintme.go ---
params\\alloc_mintme.go:1: package params
params\\alloc_mintme.go:2: 
params\\alloc_mintme.go:3: (truncated) ...c729533cd3b184d22c498429cd40aba0bfef4b74f69f238fad8f7fa\xb8@7777777777777777777777777777777777777777777777777777777700000000\xb8@5349664f7c729533cd3b184d22c4984...

--- signer\\fourbyte\\4byte.json ---
total_matches=1
signer\\fourbyte\\4byte.json:125276: "77762820": "settleGame()",
signer\\fourbyte\\4byte.json:125277: "7776466c": "preSaleToken()",
signer\\fourbyte\\4byte.json:125278: "777665b8": "transferTOKENtoProviders(address,address,uint256,address,uint256)",
signer\\fourbyte\\4byte.json:125279: "7776afa0": "_mint(address,uint256,uint256)",
signer\\fourbyte\\4byte.json:125280: "77773d90": "amountOfTokensPerEther()",
signer\\fourbyte\\4byte.json:125281: "7777789f": "_mint(address,uint256,uint256[])",
signer\\fourbyte\\4byte.json:125281: "7777789f": "_mint(address,uint256,uint256[])",
signer\\fourbyte\\4byte.json:125282: "7777d088": "lotteryTokensPercent()",
signer\\fourbyte\\4byte.json:125283: "777850f9": "payAfter(address,uint256)",
signer\\fourbyte\\4byte.json:125284: "777878c0": "flashloan(address,address[])",
signer\\fourbyte\\4byte.json:125285: "77790081": "updateMaritalStatus(string)",
`

Notes:
- Several matches in large test vectors or datasets (e.g., precompile JSON, trusted setup, fourbyte database) are incidental hex/function IDs and not chain configuration.
- params/alloc_mintme.go and signer/fourbyte/4byte.json are extremely large; context lines are truncated for readability (no functional impact).

## 2) Dataflow: log line -> enforcement decision (and impact)

### Log line site
`

  eth\backend.go:262:	}
  eth\backend.go:263:	log.Info("Chain identity", "chain_id", chainID, "network_id", networkID, "genesis", genesisHash)
  eth\backend.go:264:	if isEthernova {
  eth\backend.go:265:		enforcement := ethernova.ForkEnforcementDecision(chainID, genesisHash)
  eth\backend.go:266:		enforcementBlock := ethernova.FormatBlockWithCommas(enforcement.Block)
> eth\backend.go:267:		log.Info(fmt.Sprintf("Ethernova fork enforcement block=%s", enforcementBlock),
  eth\backend.go:268:			"chain_id", chainID, "genesis", genesisHash, "reason", enforcement.Reason)
  eth\backend.go:269:		if enforcement.Warning != "" {
> eth\backend.go:270:			log.Warn("Ethernova fork enforcement warning", "warning", enforcement.Warning, "chain_id", 
chainID, "genesis", genesisHash)
  eth\backend.go:271:		}
  eth\backend.go:272:		log.Info(fmt.Sprintf("Ethernova EVM fork block=%s enforcement block=%s",
  eth\backend.go:273:			ethernova.FormatBlockWithCommas(ethernova.EVMCompatibilityForkBlock), enforcementBlock),
  eth\backend.go:274:			"chain_id", chainID, "genesis", genesisHash)
  eth\backend.go:275:		if chainID == nil || chainID.Cmp(ethernova.NewChainIDBig) != 0 {
`

### Enforcement decision logic (chain-aware)
`

  params\ethernova\enforcement.go:30:	if chainID != nil {
  params\ethernova\enforcement.go:31:		chainIDValue = chainID.Uint64()
  params\ethernova\enforcement.go:32:	}
  params\ethernova\enforcement.go:33:	decision := EnforcementDecision{
  params\ethernova\enforcement.go:34:		ChainID: chainIDValue,
  params\ethernova\enforcement.go:35:		Genesis: genesis,
  params\ethernova\enforcement.go:36:	}
  params\ethernova\enforcement.go:37:
  params\ethernova\enforcement.go:38:	if chainID != nil && chainID.Cmp(NewChainIDBig) == 0 {
  params\ethernova\enforcement.go:39:		decision.Block = EVMCompatibilityForkBlock
> params\ethernova\enforcement.go:40:		decision.Reason = "chain=121525"
  params\ethernova\enforcement.go:41:		if genesis != (common.Hash{}) && genesis != ExpectedGenesisHash {
  params\ethernova\enforcement.go:42:			decision.Warning = fmt.Sprintf("chainId=121525 but genesis mismatch (got %s 
want %s); using enforcement %d",
  params\ethernova\enforcement.go:43:				genesis.Hex(), ExpectedGenesisHash.Hex(), decision.Block)
> params\ethernova\enforcement.go:44:			decision.Reason = "chain=121525 (genesis mismatch)"
  params\ethernova\enforcement.go:45:		}
  params\ethernova\enforcement.go:46:		return decision
  params\ethernova\enforcement.go:47:	}
  params\ethernova\enforcement.go:48:
  params\ethernova\enforcement.go:49:	if chainID != nil && chainIDValue == LegacyChainID {
  params\ethernova\enforcement.go:50:		decision.Block = LegacyForkEnforcementBlock
  params\ethernova\enforcement.go:51:		decision.Reason = "legacy chainId=77777"
  params\ethernova\enforcement.go:52:		return decision
  params\ethernova\enforcement.go:53:	}
  params\ethernova\enforcement.go:54:
  params\ethernova\enforcement.go:55:	if genesis == LegacyGenesisHash {
  params\ethernova\enforcement.go:56:		decision.Block = LegacyForkEnforcementBlock
  params\ethernova\enforcement.go:57:		decision.Reason = "legacy genesis"
  params\ethernova\enforcement.go:58:		return decision
  params\ethernova\enforcement.go:59:	}
  params\ethernova\enforcement.go:60:
  params\ethernova\enforcement.go:61:	decision.Block = 0
  params\ethernova\enforcement.go:62:	decision.Reason = "unknown chain"
  params\ethernova\enforcement.go:63:	decision.Warning = "unknown chain; enforcement disabled (no legacy fallback)"
  params\ethernova\enforcement.go:64:	return decision
`

### Usage of ForkEnforcementDecision (search)
`

  eth\backend.go:263:	log.Info("Chain identity", "chain_id", chainID, "network_id", networkID, "genesis", genesisHash)
  eth\backend.go:264:	if isEthernova {
> eth\backend.go:265:		enforcement := ethernova.ForkEnforcementDecision(chainID, genesisHash)
  eth\backend.go:266:		enforcementBlock := ethernova.FormatBlockWithCommas(enforcement.Block)
  eth\backend.go:267:		log.Info(fmt.Sprintf("Ethernova fork enforcement block=%s", enforcementBlock),
  params\ethernova\enforcement.go:26:}
  params\ethernova\enforcement.go:27:
> params\ethernova\enforcement.go:28:func ForkEnforcementDecision(chainID *big.Int, genesis common.Hash) 
EnforcementDecision {
  params\ethernova\enforcement.go:29:	var chainIDValue uint64
  params\ethernova\enforcement.go:30:	if chainID != nil {
  params\ethernova\enforcement_test.go:8:)
  params\ethernova\enforcement_test.go:9:
> params\ethernova\enforcement_test.go:10:func TestForkEnforcementDecisionMainnet(t *testing.T) {
> params\ethernova\enforcement_test.go:11:	decision := ForkEnforcementDecision(new(big.Int).Set(NewChainIDBig), 
ExpectedGenesisHash)
  params\ethernova\enforcement_test.go:12:	if decision.Block != EVMCompatibilityForkBlock {
  params\ethernova\enforcement_test.go:13:		t.Fatalf("expected enforcement %d, got %d", EVMCompatibilityForkBlock, 
decision.Block)
  params\ethernova\enforcement_test.go:18:}
  params\ethernova\enforcement_test.go:19:
> params\ethernova\enforcement_test.go:20:func TestForkEnforcementDecisionMainnetGenesisMismatchWarns(t *testing.T) {
  params\ethernova\enforcement_test.go:21:	badGenesis := common.HexToHash("0x1234")
> params\ethernova\enforcement_test.go:22:	decision := ForkEnforcementDecision(new(big.Int).Set(NewChainIDBig), 
badGenesis)
  params\ethernova\enforcement_test.go:23:	if decision.Block != EVMCompatibilityForkBlock {
  params\ethernova\enforcement_test.go:24:		t.Fatalf("expected enforcement %d, got %d", EVMCompatibilityForkBlock, 
decision.Block)
  params\ethernova\enforcement_test.go:29:}
  params\ethernova\enforcement_test.go:30:
> params\ethernova\enforcement_test.go:31:func TestForkEnforcementDecisionLegacyChain(t *testing.T) {
> params\ethernova\enforcement_test.go:32:	decision := ForkEnforcementDecision(new(big.Int).SetUint64(LegacyChainID), 
LegacyGenesisHash)
  params\ethernova\enforcement_test.go:33:	if decision.Block != LegacyForkEnforcementBlock {
  params\ethernova\enforcement_test.go:34:		t.Fatalf("expected enforcement %d, got %d", LegacyForkEnforcementBlock, 
decision.Block)
  params\ethernova\enforcement_test.go:36:}
  params\ethernova\enforcement_test.go:37:
> params\ethernova\enforcement_test.go:38:func TestForkEnforcementDecisionUnknownChain(t *testing.T) {
> params\ethernova\enforcement_test.go:39:	decision := ForkEnforcementDecision(big.NewInt(424242), 
common.HexToHash("0xdeadbeef"))
  params\ethernova\enforcement_test.go:40:	if decision.Block == LegacyForkEnforcementBlock {
  params\ethernova\enforcement_test.go:41:		t.Fatalf("unknown chain should not default to legacy enforcement %d", 
LegacyForkEnforcementBlock)
`

Conclusion:
- ForkEnforcementDecision is only referenced in eth/backend.go (logging) and in tests. No references appear in consensus, VM opcode activation, or fork-id computation.
- Therefore, the enforcement value affects logging/telemetry only, not fork activation.

## 3) Fork activation at block 105000 (Constantinople/Petersburg/Istanbul)

### Fork block constant
`

  params\ethernova\forks.go:2:
  params\ethernova\forks.go:3:const (
> params\ethernova\forks.go:4:	// EVMCompatibilityForkBlock enables Constantinople + Petersburg + Istanbul.
> params\ethernova\forks.go:5:	EVMCompatibilityForkBlock uint64 = 105000
  params\ethernova\forks.go:6:)
`

### Genesis config (mainnet and embedded alloc)
`

  genesis-mainnet.json:423:    },
  genesis-mainnet.json:424:    "chainId": 121525,
> genesis-mainnet.json:425:    "constantinopleBlock": 105000,
> genesis-mainnet.json:426:    "petersburgBlock": 105000,
> genesis-mainnet.json:427:    "istanbulBlock": 105000,
  genesis-mainnet.json:428:    "eip1559FBlock": 0,
  genesis-mainnet.json:429:    "eip155Block": 0,


  params\ethernova\genesis-121525-alloc.json:423:    },
  params\ethernova\genesis-121525-alloc.json:424:    "chainId": 121525,
> params\ethernova\genesis-121525-alloc.json:425:    "constantinopleBlock": 105000,
> params\ethernova\genesis-121525-alloc.json:426:    "petersburgBlock": 105000,
> params\ethernova\genesis-121525-alloc.json:427:    "istanbulBlock": 105000,
  params\ethernova\genesis-121525-alloc.json:428:    "eip1559FBlock": 0,
  params\ethernova\genesis-121525-alloc.json:429:    "eip155Block": 0,
`

### Chain-config migration guard (uses fork=105000)
`

  core\ethernova_fork_migration.go:18:	chainID := cfg.GetChainID()
  core\ethernova_fork_migration.go:19:	if chainID == nil || chainID.Uint64() != ethernova.NewChainID {
  core\ethernova_fork_migration.go:20:		return false, nil
  core\ethernova_fork_migration.go:21:	}
  core\ethernova_fork_migration.go:22:
> core\ethernova_fork_migration.go:23:	forkBlock := ethernova.EVMCompatibilityForkBlock
> core\ethernova_fork_migration.go:24:	missing, mismatched, err := ethernovaForkStatus(cfg, forkBlock)
  core\ethernova_fork_migration.go:25:	if err != nil {
  core\ethernova_fork_migration.go:26:		return false, err
  core\ethernova_fork_migration.go:27:	}
  core\ethernova_fork_migration.go:28:	if len(mismatched) > 0 {
> core\ethernova_fork_migration.go:29:		return false, fmt.Errorf("ethernova chain config has unexpected fork block 
values (%s); expected %d", strings.Join(mismatched, ", "), forkBlock)
  core\ethernova_fork_migration.go:30:	}
  core\ethernova_fork_migration.go:31:	if !missing {
  core\ethernova_fork_migration.go:32:		return false, nil
  core\ethernova_fork_migration.go:33:	}
> core\ethernova_fork_migration.go:34:	if head >= forkBlock {
> core\ethernova_fork_migration.go:35:		return false, fmt.Errorf("UPGRADE REQUIRED: ethernova chain config missing 
Constantinople/Petersburg/Istanbul fork blocks; head=%d fork=%d. Refusing to start; upgrade before block %d", head, 
forkBlock, forkBlock)
  core\ethernova_fork_migration.go:36:	}
> core\ethernova_fork_migration.go:37:	updated, err := ethernovaApplyForks(cfg, forkBlock)
  core\ethernova_fork_migration.go:38:	if err != nil {
  core\ethernova_fork_migration.go:39:		return false, err
  core\ethernova_fork_migration.go:40:	}
  core\ethernova_fork_migration.go:41:	if updated {
> core\ethernova_fork_migration.go:42:		log.Warn("Ethernova chain config upgraded in-place", "fork_block", forkBlock, 
"head", head)
  core\ethernova_fork_migration.go:43:	}
  core\ethernova_fork_migration.go:44:	return updated, nil
  core\ethernova_fork_migration.go:45:}
  core\ethernova_fork_migration.go:46:
> core\ethernova_fork_migration.go:47:func ethernovaForkStatus(cfg ctypes.ChainConfigurator, forkBlock uint64) 
(missing bool, mismatched []string, err error) {
  core\ethernova_fork_migration.go:48:	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
  core\ethernova_fork_migration.go:49:	if !ok {
  core\ethernova_fork_migration.go:50:		return false, nil, fmt.Errorf("unsupported chain config type for ethernova: 
%T", cfg)
  core\ethernova_fork_migration.go:51:	}
  core\ethernova_fork_migration.go:52:	checkBig := func(name string, val *big.Int) {
  core\ethernova_fork_migration.go:53:		if val == nil {
  core\ethernova_fork_migration.go:54:			missing = true
  core\ethernova_fork_migration.go:55:			return
  core\ethernova_fork_migration.go:56:		}
> core\ethernova_fork_migration.go:57:		if val.Uint64() != forkBlock {
  core\ethernova_fork_migration.go:58:			mismatched = append(mismatched, fmt.Sprintf("%s=%d", name, val.Uint64()))
  core\ethernova_fork_migration.go:59:		}
  core\ethernova_fork_migration.go:60:	}
  core\ethernova_fork_migration.go:61:	checkBig("constantinopleBlock", cg.ConstantinopleBlock)
  core\ethernova_fork_migration.go:62:	checkBig("petersburgBlock", cg.PetersburgBlock)
  core\ethernova_fork_migration.go:64:
  core\ethernova_fork_migration.go:65:	check := func(name string, val *uint64) {
  core\ethernova_fork_migration.go:66:		if val == nil {
  core\ethernova_fork_migration.go:67:			return
  core\ethernova_fork_migration.go:68:		}
> core\ethernova_fork_migration.go:69:		if *val != forkBlock {
  core\ethernova_fork_migration.go:70:			mismatched = append(mismatched, fmt.Sprintf("%s=%d", name, *val))
  core\ethernova_fork_migration.go:71:		}
  core\ethernova_fork_migration.go:72:	}
  core\ethernova_fork_migration.go:73:	check("eip145", cfg.GetEIP145Transition())
  core\ethernova_fork_migration.go:74:	check("eip1014", cfg.GetEIP1014Transition())
  core\ethernova_fork_migration.go:82:	check("eip2028", cfg.GetEIP2028Transition())
  core\ethernova_fork_migration.go:83:	check("eip2200", cfg.GetEIP2200Transition())
  core\ethernova_fork_migration.go:84:	return missing, mismatched, nil
  core\ethernova_fork_migration.go:85:}
  core\ethernova_fork_migration.go:86:
> core\ethernova_fork_migration.go:87:func ethernovaApplyForks(cfg ctypes.ChainConfigurator, forkBlock uint64) (bool, 
error) {
  core\ethernova_fork_migration.go:88:	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
  core\ethernova_fork_migration.go:89:	if !ok {
  core\ethernova_fork_migration.go:90:		return false, fmt.Errorf("unsupported chain config type for ethernova: %T", 
cfg)
  core\ethernova_fork_migration.go:91:	}
  core\ethernova_fork_migration.go:92:	updated := false
  core\ethernova_fork_migration.go:93:	if cg.ConstantinopleBlock == nil {
> core\ethernova_fork_migration.go:94:		cg.ConstantinopleBlock = new(big.Int).SetUint64(forkBlock)
  core\ethernova_fork_migration.go:95:		updated = true
  core\ethernova_fork_migration.go:96:	}
  core\ethernova_fork_migration.go:97:	if cg.PetersburgBlock == nil {
> core\ethernova_fork_migration.go:98:		cg.PetersburgBlock = new(big.Int).SetUint64(forkBlock)
  core\ethernova_fork_migration.go:99:		updated = true
  core\ethernova_fork_migration.go:100:	}
  core\ethernova_fork_migration.go:101:	if cg.IstanbulBlock == nil {
> core\ethernova_fork_migration.go:102:		cg.IstanbulBlock = new(big.Int).SetUint64(forkBlock)
  core\ethernova_fork_migration.go:103:		updated = true
  core\ethernova_fork_migration.go:104:	}
  core\ethernova_fork_migration.go:105:	return updated, nil
  core\ethernova_fork_migration.go:106:}
`

### Runtime EVM opcode test (SHL/CHAINID/SELFBALANCE pre/post fork)
`

  core\vm\runtime\ethernova_fork_test.go:32:	if cfg.IsEnabled(cfg.GetEIP145Transition, pre) {
  core\vm\runtime\ethernova_fork_test.go:33:		t.Fatalf("eip145 should be disabled before fork %d", fork)
  core\vm\runtime\ethernova_fork_test.go:34:	}
  core\vm\runtime\ethernova_fork_test.go:35:	if !cfg.IsEnabled(cfg.GetEIP145Transition, post) {
  core\vm\runtime\ethernova_fork_test.go:36:		t.Fatalf("eip145 should be enabled at fork %d", fork)
  core\vm\runtime\ethernova_fork_test.go:37:	}
  core\vm\runtime\ethernova_fork_test.go:38:	if cfg.IsEnabled(cfg.GetEIP1283DisableTransition, pre) {
  core\vm\runtime\ethernova_fork_test.go:39:		t.Fatalf("petersburg should be disabled before fork %d", fork)
  core\vm\runtime\ethernova_fork_test.go:40:	}
  core\vm\runtime\ethernova_fork_test.go:41:	if !cfg.IsEnabled(cfg.GetEIP1283DisableTransition, post) {
  core\vm\runtime\ethernova_fork_test.go:42:		t.Fatalf("petersburg should be enabled at fork %d", fork)
  core\vm\runtime\ethernova_fork_test.go:43:	}
  core\vm\runtime\ethernova_fork_test.go:44:	if cfg.IsEnabled(cfg.GetEIP1344Transition, pre) {
  core\vm\runtime\ethernova_fork_test.go:45:		t.Fatalf("eip1344 should be disabled before fork %d", fork)
  core\vm\runtime\ethernova_fork_test.go:46:	}
  core\vm\runtime\ethernova_fork_test.go:47:	if !cfg.IsEnabled(cfg.GetEIP1344Transition, post) {
  core\vm\runtime\ethernova_fork_test.go:48:		t.Fatalf("eip1344 should be enabled at fork %d", fork)
  core\vm\runtime\ethernova_fork_test.go:49:	}
  core\vm\runtime\ethernova_fork_test.go:50:}
  core\vm\runtime\ethernova_fork_test.go:51:
> core\vm\runtime\ethernova_fork_test.go:52:func TestEthernovaEVMOpcodesPrePostFork(t *testing.T) {
  core\vm\runtime\ethernova_fork_test.go:53:	fork := ethernova.EVMCompatibilityForkBlock
  core\vm\runtime\ethernova_fork_test.go:54:	cfg := ethernovaForkConfig(t, fork)
  core\vm\runtime\ethernova_fork_test.go:55:
  core\vm\runtime\ethernova_fork_test.go:56:	shlCode := []byte{0x60, 0x01, 0x60, 0x02, 0x1b, 0x00} // PUSH1 1, PUSH1 
2, SHL, STOP
  core\vm\runtime\ethernova_fork_test.go:57:	preCfg := &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork - 
1))}
  core\vm\runtime\ethernova_fork_test.go:58:	if _, _, err := Execute(shlCode, nil, preCfg); err == nil {
  core\vm\runtime\ethernova_fork_test.go:59:		t.Fatal("expected SHL to be invalid before fork")
  core\vm\runtime\ethernova_fork_test.go:60:	} else {
  core\vm\runtime\ethernova_fork_test.go:61:		var invalid *vm.ErrInvalidOpCode
  core\vm\runtime\ethernova_fork_test.go:62:		if !errors.As(err, &invalid) {
  core\vm\runtime\ethernova_fork_test.go:63:			t.Fatalf("expected invalid opcode before fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:64:		}
  core\vm\runtime\ethernova_fork_test.go:65:	}
  core\vm\runtime\ethernova_fork_test.go:66:
  core\vm\runtime\ethernova_fork_test.go:67:	postCfg := &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork))}
  core\vm\runtime\ethernova_fork_test.go:68:	if _, _, err := Execute(shlCode, nil, postCfg); err != nil {
  core\vm\runtime\ethernova_fork_test.go:69:		t.Fatalf("expected SHL to succeed after fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:70:	}
  core\vm\runtime\ethernova_fork_test.go:71:
  core\vm\runtime\ethernova_fork_test.go:72:	chainIDCode := []byte{0x46, 0x00} // CHAINID, STOP


  core\vm\runtime\ethernova_fork_test.go:8:	"github.com/ethereum/go-ethereum/core/vm"
  core\vm\runtime\ethernova_fork_test.go:9:	"github.com/ethereum/go-ethereum/params/ethernova"
  core\vm\runtime\ethernova_fork_test.go:10:	coregeth "github.com/ethereum/go-ethereum/params/types/coregeth"
  core\vm\runtime\ethernova_fork_test.go:11:)
  core\vm\runtime\ethernova_fork_test.go:12:
  core\vm\runtime\ethernova_fork_test.go:13:func ethernovaForkConfig(t *testing.T, forkBlock uint64) 
*coregeth.CoreGethChainConfig {
  core\vm\runtime\ethernova_fork_test.go:14:	t.Helper()
  core\vm\runtime\ethernova_fork_test.go:15:	cfg := &coregeth.CoreGethChainConfig{
> core\vm\runtime\ethernova_fork_test.go:16:		NetworkID: ethernova.NewChainID,
> core\vm\runtime\ethernova_fork_test.go:17:		ChainID:   new(big.Int).SetUint64(ethernova.NewChainID),
  core\vm\runtime\ethernova_fork_test.go:18:	}
  core\vm\runtime\ethernova_fork_test.go:19:	cfg.ConstantinopleBlock = new(big.Int).SetUint64(forkBlock)
  core\vm\runtime\ethernova_fork_test.go:20:	cfg.PetersburgBlock = new(big.Int).SetUint64(forkBlock)
  core\vm\runtime\ethernova_fork_test.go:21:	cfg.IstanbulBlock = new(big.Int).SetUint64(forkBlock)
  core\vm\runtime\ethernova_fork_test.go:22:	return cfg
  core\vm\runtime\ethernova_fork_test.go:23:}
  core\vm\runtime\ethernova_fork_test.go:24:
  core\vm\runtime\ethernova_fork_test.go:25:func TestEthernovaForkActivation105000(t *testing.T) {
  core\vm\runtime\ethernova_fork_test.go:64:		}
  core\vm\runtime\ethernova_fork_test.go:65:	}
  core\vm\runtime\ethernova_fork_test.go:66:
  core\vm\runtime\ethernova_fork_test.go:67:	postCfg := &Config{ChainConfig: cfg, BlockNumber: big.NewInt(int64(fork))}
  core\vm\runtime\ethernova_fork_test.go:68:	if _, _, err := Execute(shlCode, nil, postCfg); err != nil {
  core\vm\runtime\ethernova_fork_test.go:69:		t.Fatalf("expected SHL to succeed after fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:70:	}
  core\vm\runtime\ethernova_fork_test.go:71:
> core\vm\runtime\ethernova_fork_test.go:72:	chainIDCode := []byte{0x46, 0x00} // CHAINID, STOP
> core\vm\runtime\ethernova_fork_test.go:73:	if _, _, err := Execute(chainIDCode, nil, preCfg); err == nil {
> core\vm\runtime\ethernova_fork_test.go:74:		t.Fatal("expected CHAINID to be invalid before fork")
  core\vm\runtime\ethernova_fork_test.go:75:	} else {
  core\vm\runtime\ethernova_fork_test.go:76:		var invalid *vm.ErrInvalidOpCode
  core\vm\runtime\ethernova_fork_test.go:77:		if !errors.As(err, &invalid) {
  core\vm\runtime\ethernova_fork_test.go:78:			t.Fatalf("expected invalid opcode before fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:79:		}
  core\vm\runtime\ethernova_fork_test.go:80:	}
> core\vm\runtime\ethernova_fork_test.go:81:	if _, _, err := Execute(chainIDCode, nil, &Config{ChainConfig: cfg, 
BlockNumber: big.NewInt(int64(fork))}); err != nil {
> core\vm\runtime\ethernova_fork_test.go:82:		t.Fatalf("expected CHAINID to succeed after fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:83:	}
  core\vm\runtime\ethernova_fork_test.go:84:
> core\vm\runtime\ethernova_fork_test.go:85:	selfBalanceCode := []byte{0x47, 0x00} // SELFBALANCE, STOP
> core\vm\runtime\ethernova_fork_test.go:86:	if _, _, err := Execute(selfBalanceCode, nil, preCfg); err == nil {
> core\vm\runtime\ethernova_fork_test.go:87:		t.Fatal("expected SELFBALANCE to be invalid before fork")
  core\vm\runtime\ethernova_fork_test.go:88:	} else {
  core\vm\runtime\ethernova_fork_test.go:89:		var invalid *vm.ErrInvalidOpCode
  core\vm\runtime\ethernova_fork_test.go:90:		if !errors.As(err, &invalid) {
  core\vm\runtime\ethernova_fork_test.go:91:			t.Fatalf("expected invalid opcode before fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:92:		}
  core\vm\runtime\ethernova_fork_test.go:93:	}
> core\vm\runtime\ethernova_fork_test.go:94:	if _, _, err := Execute(selfBalanceCode, nil, &Config{ChainConfig: cfg, 
BlockNumber: big.NewInt(int64(fork))}); err != nil {
> core\vm\runtime\ethernova_fork_test.go:95:		t.Fatalf("expected SELFBALANCE to succeed after fork, got %v", err)
  core\vm\runtime\ethernova_fork_test.go:96:	}
  core\vm\runtime\ethernova_fork_test.go:97:}
`

### Go test output
`
ok  	github.com/ethereum/go-ethereum/core/vm/runtime	(cached)
`

## 4) Fork-id change proof
`

  core\forkid\ethernova_forkid_test.go:25:
  core\forkid\ethernova_forkid_test.go:26:	idNoFork := NewID(baseCfg, genesisBlock, 0, 0)
  core\forkid\ethernova_forkid_test.go:27:
  core\forkid\ethernova_forkid_test.go:28:	forkCfg := &coregeth.CoreGethChainConfig{
  core\forkid\ethernova_forkid_test.go:29:		NetworkID:           ethernova.NewChainID,
  core\forkid\ethernova_forkid_test.go:30:		ChainID:             new(big.Int).SetUint64(ethernova.NewChainID),
> core\forkid\ethernova_forkid_test.go:31:		ConstantinopleBlock: 
new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
> core\forkid\ethernova_forkid_test.go:32:		PetersburgBlock:     
new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
> core\forkid\ethernova_forkid_test.go:33:		IstanbulBlock:       
new(big.Int).SetUint64(ethernova.EVMCompatibilityForkBlock),
  core\forkid\ethernova_forkid_test.go:34:	}
  core\forkid\ethernova_forkid_test.go:35:	idFork := NewID(forkCfg, genesisBlock, 0, 0)
  core\forkid\ethernova_forkid_test.go:36:
  core\forkid\ethernova_forkid_test.go:37:	if idFork == idNoFork {
  core\forkid\ethernova_forkid_test.go:38:		t.Fatalf("forkid should change when fork block is configured")
  core\forkid\ethernova_forkid_test.go:39:	}
> core\forkid\ethernova_forkid_test.go:40:	if idFork.Next != ethernova.EVMCompatibilityForkBlock {
> core\forkid\ethernova_forkid_test.go:41:		t.Fatalf("unexpected forkid next: have %d want %d", idFork.Next, 
ethernova.EVMCompatibilityForkBlock)
  core\forkid\ethernova_forkid_test.go:42:	}
  core\forkid\ethernova_forkid_test.go:43:}
`

Go test output:
`
ok  	github.com/ethereum/go-ethereum/core/forkid	(cached)
`

## 5) Enforcement log correctness for chain 121525
`

  logs\ethernova.log:114:INFO [02-02|13:28:33.924] Chain identity                           chain_id=121,525 
network_id=121,525 genesis=c3812e..c453d9
  logs\ethernova.log:115:INFO [02-02|13:28:33.924] Resuming state snapshot generation       root=4d320b..66c200 
accounts=0 slots=0 storage=0.00B dangling=0 elapsed=4.041ms
> logs\ethernova.log:116:INFO [02-02|13:28:33.926] "Ethernova fork enforcement block=105,000" chain_id=121,525 
genesis=c3812e..c453d9 reason="chain=121525"
  logs\ethernova.log:117:INFO [02-02|13:28:33.929] "Ethernova EVM fork block=105,000 enforcement block=105,000" 
chain_id=121,525 genesis=c3812e..c453d9
  logs\ethernova.log:118:INFO [02-02|13:28:33.939] Generated state snapshot                 accounts=136 slots=0 
storage=6.26KiB dangling=0 elapsed=19.803ms
  logs\ethernova.log:191:INFO [02-02|13:30:38.639] Chain identity                           chain_id=121,525 
network_id=121,525 genesis=c3812e..c453d9
  logs\ethernova.log:192:INFO [02-02|13:30:38.638] Resuming state snapshot generation       root=4d320b..66c200 
accounts=0 slots=0 storage=0.00B dangling=0 elapsed=4.474ms
> logs\ethernova.log:193:INFO [02-02|13:30:38.640] "Ethernova fork enforcement block=105,000" chain_id=121,525 
genesis=c3812e..c453d9 reason="chain=121525"
  logs\ethernova.log:194:INFO [02-02|13:30:38.645] "Ethernova EVM fork block=105,000 enforcement block=105,000" 
chain_id=121,525 genesis=c3812e..c453d9
  logs\ethernova.log:195:INFO [02-02|13:30:38.652] Generated state snapshot                 accounts=136 slots=0 
storage=6.26KiB dangling=0 elapsed=19.007ms
`

This shows the runtime log line:
- "Ethernova fork enforcement block=105,000" for chain_id=121,525 + genesis=c3812e..c453d9.
- No evidence of 138,396 being used for chain 121525.

## 6) Genesis unchanged proof

### SHA256 of genesis files
`

Algorithm Hash                                                             Path                                        
--------- ----                                                             ----                                        
SHA256    768F10BF9F77E20D5DA970436E283C5A6892C9169A7AF6D33C8E8EC506C9957D C:\dev\Ethernova\core-geth-src-clean-v1.2...
SHA256    768F10BF9F77E20D5DA970436E283C5A6892C9169A7AF6D33C8E8EC506C9957D C:\dev\Ethernova\core-geth-src-clean-v1.2...
`

### Embedded genesis in binary (print-genesis)
`
expected_genesis_hash=0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9
chain_id=121525
network_id=121525
embedded_genesis_sha256=768f10bf9f77e20d5da970436e283c5a6892c9169a7af6d33c8e8ec506c9957d
`

### Coinbase + baseFeeVault values
`

  genesis-mainnet.json:412:  "baseFeePerGas": "0x3b9aca00",
> genesis-mainnet.json:413:  "coinbase": "0x3a38560b66205bb6a31decbcb245450b2f15d4fd",
  genesis-mainnet.json:414:  "config": {
> genesis-mainnet.json:415:    "baseFeeVault": "0x3a38560b66205bb6a31decbcb245450b2f15d4fd",
> genesis-mainnet.json:416:    "baseFeeVaultFromBlock": 0,
  genesis-mainnet.json:417:    "blockReward": {
`

## 7) RPC evidence (Windows, no external deps)
`
== Ethernova Fork Verify ==
exe=C:\dev\Ethernova\core-geth-src-clean-v1.2.7\dist\ethernova-v1.2.8-windows-amd64.exe
datadir=C:\Users\Dev\AppData\Local\Temp\ethernova-verify-fork-235e4912128c4163a2a566d7c27bcd30
endpoint=http://127.0.0.1:8545
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
`

This shows:
- eth_chainId == 0x1dab5 (121525)
- genesis hash == 0xc3812e...c453d9
- SHL/CHAINID/SELFBALANCE invalid at block 104999 and valid at block 105000

## Conclusion
- The legacy enforcement value 138396 is not applied to chain 121525.
- Fork activation is scheduled at block 105000 and verified by tests and RPC trace calls.
- Genesis hash matches the expected value and the embedded genesis SHA256.

## Minimal Fix Plan (only if regression is observed)
- If a future run logs "Ethernova fork enforcement block=138,396" on chain 121525:
  1) Update params/ethernova/ForkEnforcementDecision to force 105000 when chainId==121525 and genesis==ExpectedGenesisHash.
  2) Keep 138396 only for legacy chainId 77777 or legacy genesis.
  3) Ensure TestForkEnforcementDecisionMainnet remains to prevent regression.
