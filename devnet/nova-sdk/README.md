# Nova SDK

Small dependency-free helper for the devnet `nova_*` RPC namespace introduced in NIP-0004 Phase 8.

```js
const { NovaProvider } = require("./devnet/nova-sdk");

const nova = new NovaProvider("https://devrpc.ethnova.net");

const domain = await nova.getDomain("0x0000000000000000000000000000000000000000");
const caps = await nova.getCapabilities("0x0000000000000000000000000000000000000000");
const session = await nova.getSession("0x0000000000000000000000000000000000000000000000000000000000000000");
```

The SDK tries `nova_*` first and falls back to the legacy `ethernova_*` namespace only when a node has not been upgraded yet.

## Domain Helpers

```js
const { buildDomainInitcode, domainRuntimeBytecode } = require("./devnet/nova-sdk");

const domain1Runtime = domainRuntimeBytecode(1, "0x60006000f3");
const deployableInitcode = buildDomainInitcode(2, "0x60006000f3");
```

- Domain `0`: legacy bytecode, no prefix.
- Domain `1`: Nova contracts, `0xef01` runtime prefix.
- Domain `2`: Channel/session contracts, `0xef02` runtime prefix.

`buildDomainInitcode` is intentionally simple and constructor-free. For Solidity contracts with constructor arguments, use the Hardhat helper in `devnet/nova-hardhat-plugin` or build a custom deploy wrapper that returns prefixed runtime bytecode.
