# MEGA-FORK-INVENTORY (ChainId 121525)

This inventory is derived from the CoreGeth chain config struct and getters:
- `params/types/coregeth/chain_config.go`
- `params/types/coregeth/chain_config_configurator.go`

Current values are taken from `genesis-mainnet.json` (repo root) and
`params/ethernova/genesis-121525-alloc.json`. These two configs are currently identical.

Legend:
- **Current value**: `not set` means the JSON field is absent.
- `0` means explicitly set to genesis activation.

## Supported Transition Fields (CoreGeth)

| JSON field | Getter | Human name / upstream fork | Current value (121525) |
| --- | --- | --- | --- |
| `eip2FBlock` | `GetEIP2Transition` | Homestead (EIP-2) | 118200 |
| `eip7FBlock` | `GetEIP7Transition` | Homestead (EIP-7 / DELEGATECALL) | 118200 |
| `daoForkBlock` | `GetEthashEIP779Transition` | DAO Fork (EIP-779) | not set |
| `eip150Block` | `GetEIP150Transition` | Tangerine Whistle (EIP-150) | 118200 |
| `eip155Block` | `GetEIP155Transition` | Spurious Dragon (EIP-155) | 0 |
| `eip160Block` | `GetEIP160Transition` | Spurious Dragon (EIP-160) | 118200 |
| `eip161FBlock` | `GetEIP161abcTransition` / `GetEIP161dTransition` | Spurious Dragon (EIP-161) | 118200 |
| `eip170FBlock` | `GetEIP170Transition` | Spurious Dragon (EIP-170) | 118200 |
| `eip100FBlock` | `GetEthashEIP100BTransition` | Byzantium (EIP-100) | 118200 |
| `eip140FBlock` | `GetEIP140Transition` | Byzantium (EIP-140) | 118200 |
| `eip198FBlock` | `GetEIP198Transition` | Byzantium (EIP-198) | 118200 |
| `eip211FBlock` | `GetEIP211Transition` | Byzantium (EIP-211) | 118200 |
| `eip212FBlock` | `GetEIP212Transition` | Byzantium (EIP-212) | 118200 |
| `eip213FBlock` | `GetEIP213Transition` | Byzantium (EIP-213) | 118200 |
| `eip214FBlock` | `GetEIP214Transition` | Byzantium (EIP-214) | 118200 |
| `eip658FBlock` | `GetEIP658Transition` | Byzantium (EIP-658 receipt status) | 110500 |
| `constantinopleBlock` | N/A (implicit; default for `GetEIP145Transition` / `GetEIP1014Transition` / `GetEIP1052Transition` / `GetEIP1283Transition`) | Constantinople HF | 105000 |
| `eip145FBlock` | `GetEIP145Transition` | Constantinople (EIP-145) | 105000 |
| `eip1014FBlock` | `GetEIP1014Transition` | Constantinople (EIP-1014 / CREATE2) | 105000 |
| `eip1052FBlock` | `GetEIP1052Transition` | Constantinople (EIP-1052 / EXTCODEHASH) | 105000 |
| `eip1283FBlock` | `GetEIP1283Transition` | Constantinople (EIP-1283) | 105000 |
| `petersburgBlock` | `GetEIP1283DisableTransition` | Petersburg HF (disables EIP-1283) | 105000 |
| `eip1706FBlock` | `GetEIP1706Transition` | Petersburg fix (EIP-1706) | 118200 |
| `istanbulBlock` | N/A (implicit; default for `GetEIP152Transition` / `GetEIP1108Transition` / `GetEIP1344Transition` / `GetEIP1884Transition` / `GetEIP2028Transition` / `GetEIP2200Transition`) | Istanbul HF | 105000 |
| `eip152FBlock` | `GetEIP152Transition` | Istanbul (EIP-152 / BLAKE2) | 105000 |
| `eip1108FBlock` | `GetEIP1108Transition` | Istanbul (EIP-1108) | 105000 |
| `eip1344FBlock` | `GetEIP1344Transition` | Istanbul (EIP-1344 / CHAINID) | 105000 |
| `eip1884FBlock` | `GetEIP1884Transition` | Istanbul (EIP-1884) | 105000 |
| `eip2028FBlock` | `GetEIP2028Transition` | Istanbul (EIP-2028) | 105000 |
| `eip2200FBlock` | `GetEIP2200Transition` | Istanbul (EIP-2200) | 105000 |
| `eip2200DisableFBlock` | `GetEIP2200DisableTransition` | Istanbul (EIP-2200 disable) | not set |
| `eip2384FBlock` | `GetEthashEIP2384Transition` | Muir Glacier (EIP-2384, bomb delay) | not set |
| `eip3554FBlock` | `GetEthashEIP3554Transition` | London-era bomb delay (EIP-3554) | not set |
| `eip4345FBlock` | `GetEthashEIP4345Transition` | Arrow Glacier (EIP-4345) | not set |
| `eip5133FBlock` | `GetEthashEIP5133Transition` | Gray Glacier (EIP-5133) | not set |
| `eip2565FBlock` | `GetEIP2565Transition` | Berlin (EIP-2565) | 0 |
| `eip2718FBlock` | `GetEIP2718Transition` | Berlin (EIP-2718 typed tx) | 0 |
| `eip2929FBlock` | `GetEIP2929Transition` | Berlin (EIP-2929) | 0 |
| `eip2930FBlock` | `GetEIP2930Transition` | Berlin (EIP-2930 access lists) | 0 |
| `eip1559FBlock` | `GetEIP1559Transition` | London (EIP-1559 fee market) | 0 |
| `eip3198FBlock` | `GetEIP3198Transition` | London (EIP-3198 BASEFEE) | 0 |
| `eip3529FBlock` | `GetEIP3529Transition` | London (EIP-3529 refund reduction) | 0 |
| `eip3541FBlock` | `GetEIP3541Transition` | London (EIP-3541 EOF) | 0 |
| `eip4399FBlock` | `GetEIP4399Transition` | Paris (EIP-4399 RANDOM) | not set |
| `mergeNetsplitVBlock` | `GetMergeVirtualTransition` | Merge netsplit virtual fork | not set |
| `eip3651FTime` | `GetEIP3651TransitionTime` | Shanghai (EIP-3651 Warm COINBASE) | not set |
| `eip3651FBlock` | `GetEIP3651Transition` | Shanghai (block-based) | not set |
| `eip3855FTime` | `GetEIP3855TransitionTime` | Shanghai (EIP-3855 PUSH0) | not set |
| `eip3855FBlock` | `GetEIP3855Transition` | Shanghai (block-based) | not set |
| `eip3860FTime` | `GetEIP3860TransitionTime` | Shanghai (EIP-3860 initcode limit) | not set |
| `eip3860FBlock` | `GetEIP3860Transition` | Shanghai (block-based) | not set |
| `eip4895FTime` | `GetEIP4895TransitionTime` | Shanghai (EIP-4895 withdrawals) | not set |
| `eip4895FBlock` | `GetEIP4895Transition` | Shanghai (block-based) | not set |
| `eip6049FTime` | `GetEIP6049TransitionTime` | Shanghai (EIP-6049 SELFDESTRUCT deprecation) | not set |
| `eip6049FBlock` | `GetEIP6049Transition` | Shanghai (block-based) | not set |
| `eip4844FTime` | `GetEIP4844TransitionTime` | Cancun (EIP-4844 blobs) | not set |
| `eip4844FBlock` | `GetEIP4844Transition` | Cancun (block-based) | not set |
| `eip7516FTime` | `GetEIP7516TransitionTime` | Cancun (EIP-7516 blob base fee opcode) | not set |
| `eip7516FBlock` | `GetEIP7516Transition` | Cancun (block-based) | not set |
| `eip1153FTime` | `GetEIP1153TransitionTime` | Cancun (EIP-1153 transient storage) | not set |
| `eip1153FBlock` | `GetEIP1153Transition` | Cancun (block-based) | not set |
| `eip5656FTime` | `GetEIP5656TransitionTime` | Cancun (EIP-5656 MCOPY) | not set |
| `eip5656FBlock` | `GetEIP5656Transition` | Cancun (block-based) | not set |
| `eip6780FTime` | `GetEIP6780TransitionTime` | Cancun (EIP-6780 SELFDESTRUCT changes) | not set |
| `eip6780FBlock` | `GetEIP6780Transition` | Cancun (block-based) | not set |
| `eip4788FTime` | `GetEIP4788TransitionTime` | Cancun (EIP-4788 beacon root) | not set |
| `eip4788FBlock` | `GetEIP4788Transition` | Cancun (block-based) | not set |
| `verkleFTime` | `GetVerkleTransitionTime` | Verkle (time-based) | not set |
| `verkleFBlock` | `GetVerkleTransition` | Verkle (block-based) | not set |
| `ecip1010PauseBlock` | `GetEthashECIP1010PauseTransition` | ECIP-1010 (difficulty bomb pause) | not set |
| `ecip1010Length` | `GetEthashECIP1010ContinueTransition` (derived) | ECIP-1010 (pause length) | not set |
| `ecip1017FBlock` | `GetEthashECIP1017Transition` | ECIP-1017 (reward era) | not set |
| `ecip1017EraRounds` | `GetEthashECIP1017EraRounds` | ECIP-1017 (era rounds) | not set |
| `ecip1080FBlock` | `GetECIP1080Transition` | ECIP-1080 | not set |
| `ecip1099FBlock` | `GetEthashECIP1099Transition` | ECIP-1099 (etchash) | not set |
| `ecbp1100FBlock` | `GetECBP1100Transition` | ECBP-1100 (MESS finality) | not set |
| `ecbp1100DeactivateFBlockFBlock` | `GetECBP1100DeactivateTransition` | ECBP-1100 deactivate | not set |
| `disposalBlock` | `GetEthashECIP1041Transition` | ECIP-1041 (difficulty bomb disposal) | not set |
| `eip2315FBlock` | `GetEIP2315Transition` | EIP-2315 (subroutines; not deployed on mainnet) | not set |
| `eip2537FBlock` | `GetEIP2537Transition` | EIP-2537 (BLS12-381 precompiles) | not set |
