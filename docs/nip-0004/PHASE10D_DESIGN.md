# NIP-0004 Phase 10D — Multi-Dimensional Resource Metering (consensus enforcement)

**Status**: implemented (this changeset)
**Supersedes**: Phase 10A, 10B, 10C (quote-only telemetry layers)
**Activation**: `params/ethernova.ResourceMeteringForkBlock` (current devnet = 196714)

## Goal

Promote the Phase 10A/B/C 5-dimension resource vector from a per-node
telemetry surface into a CONSENSUS object — every full node must agree
on the per-dimension usage of every block and on the per-dimension base
price for every block.

## Dimensions

The canonical NIP-0004 §6.1 5-tuple, in fixed order:

1. `compute` — `gasUsed - intrinsicGas`
2. `state_read` — `SLOAD_count * 2100 + EXTCODE*_count * 700`
3. `state_write` — `SSTORE_count * 20000 + (CREATE|CREATE2)_count * 32000 + SELFDESTRUCT_count * 5000`
4. `protocol_ops` — sum of precompile gas costs for Nova precompiles
   `0x29, 0x2A, 0x2B, 0x2C, 0x30..0x36` (excluding state-witness selectors)
5. `proof_verify` — `0x2F` (state witness) plus session commit/close/dispute
   selectors on `0x2D`

## Wire shape

### Header extension

Two new fields appended to `core/types.Header`, both `rlp:"optional"` so
pre-fork headers remain decodable:

```go
ResourceUsed       *ResourceLimits  // 5-uint64 list, sum of per-tx vectors
ResourceBasePrice  *ResourceLimits  // 5-uint64 list, basis points (10000 = 1.00x)
```

After the fork both fields MUST be present and consensus-correct. RLP
encoding for the optional cascade is hand-rolled in
`core/types/gen_header_rlp.go`.

### ResourceTx (envelope byte 0x05)

Optional new typed-tx that carries an explicit per-dimension limit
vector:

```go
type ResourceTx struct {
    ChainID            *big.Int
    Nonce              uint64
    GasTipCap          *big.Int
    GasFeeCap          *big.Int
    Gas                uint64
    To                 *common.Address `rlp:"nil"`
    Value              *big.Int
    Data               []byte
    AccessList         AccessList
    ResourceLimits     ResourceLimits   // <- the new field
    V, R, S            *big.Int
}
```

Signing follows the EIP-2718 typed-tx pattern: `0x05 || rlp(...)` with
the signing payload identical to the wire payload minus the signature
triple. Signer wraps the Cancun signer chain, see
`core/types/transaction_signing_resource.go`.

Legacy / AccessList / DynamicFee / Blob / Tempo tx are UNCHANGED.

## Consensus rules added

### CR-1 (header sum match)

For every block N ≥ ResourceMeteringForkBlock:

```
block.Header.ResourceUsed == sum_over_txs( vector_of(tx) )
```

Mismatch rejects the block. Implemented in `core/state_processor.go`
`Process()`.

### CR-2 (deterministic price adjustment)

For every block N ≥ ResourceMeteringForkBlock:

```
block.Header.ResourceBasePrice == CalcNextResourcePrice(
    parent.ResourceBasePrice,
    parent.ResourceUsed,
    parent.GasLimit,
)
```

`CalcNextResourcePrice` lives in `consensus/misc/resource_fee.go` and is
a pure, total function — no global state, no float math. EIP-1559-style
per-dimension adjustment with the same constants as the Phase 10C quote
layer (`MaxAdjustment = 12.5%`, `MaxPriceMultiplier = 16x`, target =
gasLimit/2).

### CR-3 (per-dim out-of-resource error)

During state transition, after `evm.Call` / `evm.Create` returns, the
state-transition enforcer computes the per-dim usage and compares
against:

  - For legacy / EIP-1559 / Blob tx: `vm.DeriveLegacyLimits(gasLimit)`
    which sets EVERY dimension equal to gasLimit. This is the
    BACKWARD-COMPAT GUARANTEE: no tx that succeeded pre-fork can fail
    post-fork due to the per-dim cap alone.
  - For ResourceTx (type 0x05): the explicit `tx.ResourceLimits` field.

On overflow the enforcer overwrites `vmerr` with the matching sentinel:
`ErrOutOfResourceCompute`, `..StateRead`, `..StateWrite`,
`..ProtocolOps`, `..ProofVerify`.

A `ResourceMeteringTransitionGracePostFork` window (default 0) lets a
freshly upgraded fleet record the vectors into the header WITHOUT yet
raising the OOR sentinel — useful for observing real workload before
flipping the kill switch.

## Backward compatibility surface

- **Block hash**: CHANGES at the fork block (header has new optional
  fields). This is why the fork block exists.
- **Pre-fork blocks**: untouched. RLP decode of pre-fork headers
  produces `nil` resource fields, which `state_processor` accepts.
- **Legacy transaction signing/hash**: UNCHANGED. ethers.js, MetaMask,
  Hardhat, and existing dApps continue to work without modification.
- **eth_gasPrice / eth_feeHistory / eth_estimateGas**: UNCHANGED. Gas
  charging still uses `gasUsed * effectiveGasPrice` for legacy tx; per-
  dim metering is an additional consensus check, not a fee replacement.
- **RPC schemas**: every existing `nova_*` and `eth_*` method retains
  its prior shape; new fields are additive.

## New RPC surface

- `nova_resourceConfig` — now reports `enforcement="active"`,
  `consensusGasChanged=true`, `extendedTxFormat=true`, `forkBlock`,
  `resourceTxType=0x05`, `headerFields=[resourceUsed, resourceBasePrice]`.
- `nova_getResourcePrices` — alias of `nova_resourcePrices`. Returns
  the head header's `ResourceBasePrice` as the canonical source.
- `nova_getResourceUsage` — returns the head header's `ResourceUsed`
  plus historical pricer sample.
- `nova_calcResourcePriceFor(parentPrice, parentUsage, parentGasLimit)`
  — exposes the deterministic adjustment formula so SDKs can predict
  the next block's price table without reading the chain.

## Files changed / added

See `PHASE10D_CHANGELOG.md`.

## Activation policy

- **Devnet** (chain id 121526): `ResourceMeteringForkBlock = 196714` —
  Phase 10D activates after the pre-existing devnet history, because
  blocks 0..196713 were mined before the resource header fields existed.
- **Mainnet** (chain id 121525): set explicitly in a separate PR. Until
  set, mainnet binaries refuse to enable Phase 10D enforcement.

## Risks

1. **Block hash divergence at activation** — mitigated by the
   `rlp:"optional"` cascade. Pre-fork headers serialise to identical
   bytes; post-fork headers add two list elements at the tail.
2. **Pricer skew between miner and validator** — eliminated. Pricing is
   no longer global mutable state; it is a pure function of the canonical
   chain head.
3. **Legacy tx breaking** — mitigated by `DeriveLegacyLimits` setting
   every dim equal to gasLimit (no tightening).
4. **Signature wire change** — only for ResourceTx (0x05), an OPT-IN
   new tx type. Existing tx types' signatures are unchanged.
