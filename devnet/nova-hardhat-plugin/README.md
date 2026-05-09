# Nova Hardhat Plugin

Minimal Phase 8 helper for deploying Domain 1/2 runtime bytecode on devnet.

Add this to `hardhat.config.js`:

```js
require("./devnet/nova-hardhat-plugin");
```

Deploy a constructor-free compiled artifact as a Domain 1 contract:

```js
const deployed = await hre.nova.deployArtifactDomain("MyContract", 1);
console.log(deployed.address);
```

Deploy raw runtime bytecode as a Domain 2 channel contract:

```js
const deployed = await hre.nova.deployDomainRuntime("0x60006000f3", 2);
```

Notes:

- Domain 0 stays fully backward-compatible and needs no plugin.
- Domain 1 uses runtime prefix `0xef01`.
- Domain 2 uses runtime prefix `0xef02`.
- This helper uses the artifact `deployedBytecode`, so it is best for constructor-free test contracts. Constructor-heavy production deployment should use a purpose-built wrapper that returns prefixed runtime bytecode.
