"use strict";

const {
  CHAT_MESSAGE_CONTENT_TYPE,
  CHAT_PROFILE_CONTENT_TYPE,
  NovaProvider,
  ZERO_HASH,
  buildChatMessageEnvelope,
  buildChatProfile,
  buildContentRefInput,
  buildCreateMailboxInput,
  buildMailboxSendInput,
  buildOpenChatSessionInput,
  decryptChatPayload,
  encryptChatPayload,
  generateChatIdentity,
  hashHex,
} = require("../nova-sdk");

const RPC = process.argv[2] || process.env.RPC_URL || "https://devrpc.ethnova.net";
const ZERO_ADDR = "0x0000000000000000000000000000000000000000";
const ALICE = "0x1111111111111111111111111111111111111111";
const BOB = "0x2222222222222222222222222222222222222222";
const BOB_MAILBOX = "0x" + "bb".repeat(32);

let pass = 0;
let fail = 0;
let warn = 0;

function log(level, name, detail) {
  console.log(`[${level}] ${name}${detail === undefined ? "" : ` - ${detail}`}`);
}

async function check(name, fn) {
  try {
    const detail = await fn();
    pass += 1;
    log("PASS", name, detail);
  } catch (err) {
    fail += 1;
    log("FAIL", name, err && err.message ? err.message : String(err));
  }
}

async function optional(name, fn) {
  try {
    const detail = await fn();
    pass += 1;
    log("PASS", name, detail);
  } catch (err) {
    warn += 1;
    log("WARN", name, err && err.message ? err.message : String(err));
  }
}

function must(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

async function consensusCheck() {
  const nodes = (process.env.CONSENSUS_NODES || [
    "node1=http://192.168.1.15:8551",
    "node2=http://192.168.1.34:8551",
    "node3=http://192.168.1.134:8551",
    "node4=http://192.168.1.16:8551",
    `devrpc=${RPC}`,
  ].join(","))
    .split(",")
    .map((item) => {
      const [name, url] = item.split("=");
      return { name, url };
    })
    .filter((item) => item.name && item.url);

  const rows = [];
  for (const node of nodes) {
    const provider = new NovaProvider(node.url, { fallbackNamespace: false });
    const blockHex = await provider.rpc("eth_blockNumber", []);
    const block = await provider.rpc("eth_getBlockByNumber", [blockHex, false]);
    rows.push({ name: node.name, block: blockHex, hash: block.hash });
  }
  const minBlock = Math.min(...rows.map((row) => Number.parseInt(row.block, 16)));
  const target = `0x${Math.max(0, minBlock - 1).toString(16)}`;
  const hashes = [];
  for (const node of nodes) {
    const provider = new NovaProvider(node.url, { fallbackNamespace: false });
    const block = await provider.rpc("eth_getBlockByNumber", [target, false]);
    hashes.push(`${node.name}:${block.hash}`);
  }
  const unique = new Set(hashes.map((line) => line.split(":").slice(1).join(":")));
  must(unique.size === 1, `hash mismatch at ${target}: ${hashes.join(", ")}`);
  return `target=${target} ${hashes.join(" ")}`;
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 9 - NIP-0003 Chat Rebase Proving Ground");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  const nova = new NovaProvider(RPC, { fallbackNamespace: false });
  const aliceIdentity = generateChatIdentity();
  const bobIdentity = generateChatIdentity();

  await check("eth_chainId is Ethernova devnet", async () => {
    const chainId = await nova.chainId();
    must(chainId === "0x1dab6", `expected 0x1dab6, got ${chainId}`);
    return chainId;
  });

  await check("nova_chatConfig exposes Phase 9 conventions", async () => {
    const cfg = await nova.chatConfig();
    must(cfg.phase === 9, `expected phase 9, got ${cfg.phase}`);
    must(cfg.chatProfile.contentType === CHAT_PROFILE_CONTENT_TYPE, "chat profile content type mismatch");
    must(cfg.directMessages.sessionTypeCode === 1, "chat session type must be 1");
    return cfg.description;
  });

  await check("nova_getChatMailbox owner lookup works", async () => {
    const mailbox = await nova.getChatMailbox(ZERO_ADDR, 0, 5);
    must(mailbox.profileContentType === CHAT_PROFILE_CONTENT_TYPE, "profile content type mismatch");
    must(Array.isArray(mailbox.mailboxes), "mailboxes must be an array");
    return `owner=${mailbox.owner} count=${mailbox.mailboxCount}`;
  });

  await check("X25519 chat identity generation", async () => {
    must(aliceIdentity.publicKey && bobIdentity.publicKey, "missing public keys");
    must(aliceIdentity.publicKeyHash.startsWith("0x"), "missing public key hash");
    return `${aliceIdentity.algorithm} alice=${aliceIdentity.publicKeyHash.slice(0, 18)}...`;
  });

  await check("Direct message encrypt/decrypt round trip", async () => {
    const aad = `${ALICE}:${BOB}`;
    const encrypted = encryptChatPayload("hello from phase9", aliceIdentity.privateKey, bobIdentity.publicKey, aad);
    const decrypted = decryptChatPayload(encrypted, bobIdentity.privateKey, aliceIdentity.publicKey, aad);
    must(decrypted === "hello from phase9", "decrypted message mismatch");
    return `${encrypted.algorithm} bytes=${encrypted.ciphertext.length}`;
  });

  await check("Chat profile ContentRef hash", async () => {
    const profile = buildChatProfile({
      owner: ALICE,
      mailboxId: BOB_MAILBOX,
      identity: aliceIdentity,
      createdAtBlock: 1,
      profileNonce: "phase9-test",
    });
    must(profile.contentType === CHAT_PROFILE_CONTENT_TYPE, "wrong profile content type");
    must(profile.contentHash === hashHex(profile.canonical), "profile hash mismatch");
    const input = buildContentRefInput({
      contentHash: profile.contentHash,
      size: profile.size,
      contentType: profile.contentType,
      availabilityProof: "ipfs://phase9-profile",
      rentPrepay: 1n,
      expiryBlock: 0n,
    });
    must(input.startsWith("0x01"), "ContentRef create selector missing");
    return `${profile.contentHash.slice(0, 18)}... inputBytes=${(input.length - 2) / 2}`;
  });

  await check("Message envelope hash and mailbox send input", async () => {
    const encrypted = encryptChatPayload("phase9 payload", aliceIdentity.privateKey, bobIdentity.publicKey);
    const envelope = buildChatMessageEnvelope({
      from: ALICE,
      to: BOB,
      toMailboxId: BOB_MAILBOX,
      sessionId: ZERO_HASH,
      contentRefId: ZERO_HASH,
      payload: encrypted,
      timestamp: 9,
    });
    must(envelope.contentType === CHAT_MESSAGE_CONTENT_TYPE, "wrong message content type");
    must(envelope.payloadHash === hashHex(envelope.canonical), "message hash mismatch");
    const sendInput = buildMailboxSendInput(BOB_MAILBOX, envelope.payloadHash, 0n);
    must(sendInput.length === 2 + 1 * 2 + 32 * 3 * 2, `unexpected send input length ${sendInput.length}`);
    return `${envelope.payloadHash.slice(0, 18)}...`;
  });

  await check("Chat mailbox create input shape", async () => {
    const input = buildCreateMailboxInput({
      capacityLimit: 256n,
      retentionPolicy: 0n,
      retentionBlocks: 0n,
      minPostageWei: 0n,
      aclMode: 0n,
      expiryBlock: 0n,
      rentPrepay: 0n,
      acl: [],
    });
    must(input.length === 2 + 1 * 2 + 8 * 32 * 2, `unexpected create input length ${input.length}`);
    return `bytes=${(input.length - 2) / 2}`;
  });

  await check("Chat session open input shape", async () => {
    const input = buildOpenChatSessionInput(BOB, 40n);
    must(input.startsWith("0x01"), "openSession selector missing");
    must(input.length === 2 + 1 * 2 + 5 * 32 * 2, `unexpected session input length ${input.length}`);
    return `bytes=${(input.length - 2) / 2}`;
  });

  await check("Group chat fanout envelopes are deterministic and unique", async () => {
    const recipients = [BOB, "0x3333333333333333333333333333333333333333", "0x4444444444444444444444444444444444444444"];
    const hashes = recipients.map((to, idx) =>
      buildChatMessageEnvelope({
        from: ALICE,
        to,
        toMailboxId: `0x${String(idx + 1).padStart(64, "0")}`,
        payload: { groupId: "phase9-room", body: `msg-${idx}` },
        timestamp: 9,
      }).payloadHash,
    );
    must(new Set(hashes).size === recipients.length, "fanout hashes must be unique");
    return hashes.map((h) => h.slice(0, 10)).join(",");
  });

  await check("Phase 3/4/7 primitive RPCs are present", async () => {
    const [content, mailbox, session, pending] = await Promise.all([
      nova.nova("contentRefConfig", []),
      nova.nova("mailboxConfig", []),
      nova.sessionConfig(),
      nova.getPendingEffects(0, 1),
    ]);
    must(content.active && mailbox.active && session.active, "one or more primitives inactive");
    return `pending=${pending.pending}`;
  });

  await optional("multi-node consensus still aligned", consensusCheck);

  console.log("------------------------------------------------------------------------");
  console.log(`RESULT: ${pass} pass, ${fail} fail, ${warn} warn`);
  if (fail > 0) {
    process.exit(1);
  }
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
