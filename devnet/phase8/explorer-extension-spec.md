# Phase 8 Explorer Extension Spec

This file documents the explorer-facing surface for NIP-0004 Protocol Objects, Mailboxes, Sessions, Domains, and Capabilities. The public explorer can consume these `nova_*` methods directly from `https://devrpc.ethnova.net`.

## Required RPC Namespace

Nodes must expose both namespaces during the transition:

- Canonical: `nova`
- Backward-compatible: `ethernova`

The startup API list should include:

```text
eth,net,web3,admin,debug,txpool,ethernova,nova
```

## Explorer Panels

### Contract Domain

Use `nova_getDomain(address)`.

Display:

- Domain code: `0`, `1`, or `2`
- Domain label: Legacy, Nova, or Channel
- Stored code size and runtime code size
- Domain prefix: `none`, `0xef01`, or `0xef02`
- Whether the address can directly call Nova precompiles

### Capability Model

Use `nova_getCapabilities(address)`.

Display:

- Effective capability mask
- Enabled capability names
- Precompile gate table for `0x29`, `0x2A`, `0x2B`, `0x2C`, `0x2D`, `0x2F`, and `0x35`
- Note that EOAs keep direct Nova precompile access while Domain 0 contracts do not

### Protocol Object

Use:

- `nova_getProtocolObject(id)`
- `nova_getProtocolObjectTier(id)`
- `nova_getProtocolObjectCount()`
- `nova_getProtocolObjectsByOwner(owner, offset, limit)`

Display:

- Object ID, owner, type tag/name
- Last touched block, expiry block, rent balance
- Lifecycle tier and source (`lifecycle-index` or `protocol-object-body`)

### Content Reference

Use:

- `nova_getContentRef(id)`
- `nova_listContentRefs(owner, offset, limit)`
- `nova_getContentRefCount()`

Display:

- Content hash, size, content type
- Availability proof length
- Rent balance stored/effective
- Expiry validity and reason

### Mailbox

Use:

- `nova_getMailbox(id)`
- `nova_getMailboxByOwner(owner, offset, limit)`
- `nova_getMessages(mailboxId, fromIndex, limit)`
- `nova_mailboxStats(id)`

Display:

- Capacity, retention policy, postage, ACL mode
- Queue head, tail, count, pending deliveries
- Message sender, payload hash, timestamp, sequence number

### Session / Channel

Use:

- `nova_getSession(id)`
- `nova_sessionConfig()`

Display:

- Initiator and counterparty
- Session type/status
- Sequence number and state hash
- Timeout block, dispute deadline, opened block, closed block

## Validation

Run:

```bash
node devnet/phase8/phase8-rpc-tooling-test.js https://devrpc.ethnova.net
```

Expected result:

```text
RESULT: 12 pass, 0 fail, 0 warn
```

Archive witness calls may produce a warning only if the target RPC is temporarily being rebuilt as an archive node. The method must be present and should pass on the public archive RPC once sync is complete.
