# Phase 9 - NIP-0003 Chat Rebase Proving Ground

Phase 9 maps the old NIP-0003 chat idea onto the NIP-0004 primitives that are already live on devnet.

No `MailboxConfig` RLP fields were changed. Chat identity metadata is anchored with a ContentRef convention so old mailbox objects stay valid.

## Primitive Mapping

- Chat registry: `nova_getChatMailbox(owner)` resolves mailboxes owned by an address.
- X25519 pubkey metadata: canonical chat profile JSON, hash-anchored by ContentRef.
- Direct messages: Phase 7 Session type `Chat` plus encrypted P2P payloads.
- Mailbox notifications: `novaMailboxOps` `sendMessage(mailboxId, payloadHash, postage)`.
- Message body anchors: ContentRef with content type `application/ethernova.chat-message+json`.
- Group chat: Domain 1 ChatRoom contracts can fan out mailbox notification hashes through Deferred Processing.

## Run Validation

```bash
node devnet/phase9/phase9-chat-proving-ground-test.js https://devrpc.ethnova.net
```

With explicit consensus nodes:

```bash
CONSENSUS_NODES=node1=http://192.168.1.15:8551,node2=http://192.168.1.34:8551,node3=http://192.168.1.134:8551,node4=http://192.168.1.16:8551,devrpc=https://devrpc.ethnova.net \
node devnet/phase9/phase9-chat-proving-ground-test.js https://devrpc.ethnova.net
```

## Chat Profile Shape

Content type:

```text
application/ethernova.chat-profile+json
```

Canonical JSON fields:

```json
{
  "version": 1,
  "owner": "0x...",
  "mailboxId": "0x...",
  "x25519PublicKey": "base64-spki-der",
  "x25519PublicKeyHash": "0x...",
  "createdAtBlock": 123,
  "profileNonce": "optional"
}
```

The ContentRef `contentHash` is `sha256(canonicalProfileJson)`.

## Message Envelope Shape

Content type:

```text
application/ethernova.chat-message+json
```

Canonical JSON fields:

```json
{
  "version": 1,
  "from": "0x...",
  "to": "0x...",
  "toMailboxId": "0x...",
  "sessionId": "0x...",
  "contentRefId": "0x...",
  "payload": {
    "version": 1,
    "algorithm": "X25519+AES-256-GCM",
    "nonce": "...",
    "ciphertext": "...",
    "tag": "..."
  },
  "timestamp": 123
}
```

The mailbox notification payload hash is `sha256(canonicalMessageEnvelopeJson)`.
