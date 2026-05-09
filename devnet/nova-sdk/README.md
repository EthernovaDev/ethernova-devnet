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

## Phase 9 Chat Helpers

```js
const {
  buildChatProfile,
  buildChatMessageEnvelope,
  encryptChatPayload,
  decryptChatPayload,
  generateChatIdentity,
} = require("./devnet/nova-sdk");

const alice = generateChatIdentity();
const bob = generateChatIdentity();

const encrypted = encryptChatPayload("hello", alice.privateKey, bob.publicKey);
const decrypted = decryptChatPayload(encrypted, bob.privateKey, alice.publicKey);

const profile = buildChatProfile({
  owner: "0x1111111111111111111111111111111111111111",
  mailboxId: "0x" + "bb".repeat(32),
  identity: alice,
  createdAtBlock: 1,
});

const envelope = buildChatMessageEnvelope({
  from: "0x1111111111111111111111111111111111111111",
  to: "0x2222222222222222222222222222222222222222",
  toMailboxId: profile.profile.mailboxId,
  payload: encrypted,
});
```

The chat helpers implement the Phase 9 proving-ground conventions only. They do not change consensus rules and they do not embed new fields into `MailboxConfig`.
