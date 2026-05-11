"use strict";

const http = require("http");
const https = require("https");
const crypto = require("crypto");

const CHAIN_ID = 121526;

const DOMAIN_PREFIXES = {
  0: "",
  1: "ef01",
  2: "ef02",
  legacy: "",
  nova: "ef01",
  channel: "ef02",
};

const PRECOMPILES = Object.freeze({
  protocolObjectRegistry: "0x0000000000000000000000000000000000000029",
  deferredQueue: "0x000000000000000000000000000000000000002A",
  contentRegistry: "0x000000000000000000000000000000000000002B",
  mailboxManager: "0x000000000000000000000000000000000000002C",
  sessionArbiter: "0x000000000000000000000000000000000000002D",
  stateWitness: "0x000000000000000000000000000000000000002F",
  mailboxOps: "0x0000000000000000000000000000000000000035",
});

const CHAT_PROFILE_CONTENT_TYPE = "application/ethernova.chat-profile+json";
const CHAT_MESSAGE_CONTENT_TYPE = "application/ethernova.chat-message+json";

class NovaProvider {
  constructor(rpcUrl, options = {}) {
    if (!rpcUrl) {
      throw new Error("NovaProvider requires an RPC URL");
    }
    this.rpcUrl = rpcUrl;
    this.fallbackNamespace = options.fallbackNamespace !== false;
  }

  async rpc(method, params = []) {
    const payload = {
      jsonrpc: "2.0",
      method,
      params,
      id: Date.now(),
    };
    const body = await postJson(this.rpcUrl, payload);
    if (body.error) {
      const err = new Error(body.error.message || "JSON-RPC error");
      err.code = body.error.code;
      err.data = body.error.data;
      throw err;
    }
    return body.result;
  }

  async nova(methodSuffix, params = []) {
    try {
      return await this.rpc(`nova_${methodSuffix}`, params);
    } catch (err) {
      if (!this.fallbackNamespace || err.code !== -32601) {
        throw err;
      }
      return this.rpc(`ethernova_${methodSuffix}`, params);
    }
  }

  chainId() {
    return this.rpc("eth_chainId", []);
  }

  getProtocolObject(id) {
    return this.nova("getProtocolObject", [id]);
  }

  getProtocolObjectTier(id) {
    return this.nova("getProtocolObjectTier", [id]);
  }

  getMailbox(id) {
    return this.nova("getMailbox", [id]);
  }

  getMessages(mailboxId, fromIndex = 0, limit = 50) {
    return this.nova("getMessages", [mailboxId, fromIndex, limit]);
  }

  getContentRef(id) {
    return this.nova("getContentRef", [id]);
  }

  getSession(id) {
    return this.nova("getSession", [id]);
  }

  getStateTier(address, slot = ZERO_HASH) {
    return this.nova("getStateTier", [address, slot]);
  }

  getStateWitness(address, slot = ZERO_HASH) {
    return this.nova("getStateWitness", [address, slot]);
  }

  getPendingEffects(offset = 0, limit = 50) {
    return this.nova("getPendingEffects", [offset, limit]);
  }

  getCapabilities(address) {
    return this.nova("getCapabilities", [address]);
  }

  getDomain(address) {
    return this.nova("getDomain", [address]);
  }

  sessionConfig() {
    return this.nova("sessionConfig", []);
  }

  developerTooling() {
    return this.nova("developerTooling", []);
  }

  chatConfig() {
    return this.nova("chatConfig", []);
  }

  getChatMailbox(owner, offset = 0, limit = 25) {
    return this.nova("getChatMailbox", [owner, offset, limit]);
  }

  resourceConfig() {
    return this.nova("resourceConfig", []);
  }

  resourcePrices() {
    return this.nova("resourcePrices", []);
  }

  estimateResourceLimits(gasLimit) {
    const encoded = typeof gasLimit === "string" ? gasLimit : `0x${BigInt(gasLimit).toString(16)}`;
    return this.nova("estimateResourceLimits", [encoded]);
  }

  quoteResourceFee(vector) {
    return this.nova("quoteResourceFee", [vector]);
  }

  resourceCongestion() {
    return this.nova("resourceCongestion", []);
  }

  getResourceVector(txHash) {
    return this.nova("getResourceVector", [txHash]);
  }
}

const ZERO_HASH = "0x0000000000000000000000000000000000000000000000000000000000000000";

function normalizeHex(value) {
  if (typeof value !== "string") {
    throw new Error("hex value must be a string");
  }
  return value.startsWith("0x") ? value.slice(2) : value;
}

function strip0x(value) {
  return normalizeHex(value).toLowerCase();
}

function wordHex(value) {
  if (typeof value === "bigint" || typeof value === "number") {
    if (BigInt(value) < 0n) {
      throw new Error("wordHex cannot encode negative values");
    }
    return BigInt(value).toString(16).padStart(64, "0");
  }
  const hex = strip0x(value);
  if (hex.length > 64) {
    throw new Error(`word too large: ${value}`);
  }
  return hex.padStart(64, "0");
}

function addressWord(address) {
  const hex = strip0x(address);
  if (hex.length !== 40) {
    throw new Error(`invalid address: ${address}`);
  }
  return hex.padStart(64, "0");
}

function hashHex(data) {
  const buf = Buffer.isBuffer(data) ? data : Buffer.from(String(data));
  return `0x${crypto.createHash("sha256").update(buf).digest("hex")}`;
}

function stableStringify(value) {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(",")}]`;
  }
  if (value && typeof value === "object") {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(value);
}

function domainRuntimeBytecode(domain, runtimeBytecode) {
  const prefix = DOMAIN_PREFIXES[domain];
  if (prefix === undefined) {
    throw new Error(`unknown Nova execution domain: ${domain}`);
  }
  return `0x${prefix}${normalizeHex(runtimeBytecode)}`;
}

function buildDomainInitcode(domain, runtimeBytecode) {
  const runtime = normalizeHex(domainRuntimeBytecode(domain, runtimeBytecode));
  const runtimeBytes = runtime.length / 2;
  if (runtimeBytes > 0xffff) {
    throw new Error("runtime bytecode too large for helper initcode");
  }
  const len = runtimeBytes.toString(16).padStart(4, "0");
  const offset = "000f";
  return `0x61${len}61${offset}60003961${len}6000f3${runtime}`;
}

function generateChatIdentity() {
  try {
    const { publicKey, privateKey } = crypto.generateKeyPairSync("x25519");
    const publicDer = publicKey.export({ type: "spki", format: "der" });
    const privateDer = privateKey.export({ type: "pkcs8", format: "der" });
    return {
      algorithm: "X25519",
      publicKey: publicDer.toString("base64"),
      privateKey: privateDer.toString("base64"),
      publicKeyHash: hashHex(publicDer),
    };
  } catch (_) {
    const privateKey = crypto.randomBytes(32);
    const publicKey = crypto.createHash("sha256").update(privateKey).digest();
    return {
      algorithm: "X25519-dev-fallback",
      publicKey: publicKey.toString("base64"),
      privateKey: privateKey.toString("base64"),
      publicKeyHash: hashHex(publicKey),
    };
  }
}

function importX25519Private(privateKeyBase64) {
  return crypto.createPrivateKey({
    key: Buffer.from(privateKeyBase64, "base64"),
    type: "pkcs8",
    format: "der",
  });
}

function importX25519Public(publicKeyBase64) {
  return crypto.createPublicKey({
    key: Buffer.from(publicKeyBase64, "base64"),
    type: "spki",
    format: "der",
  });
}

function deriveChatKey(privateKeyBase64, peerPublicKeyBase64) {
  const secret = crypto.diffieHellman({
    privateKey: importX25519Private(privateKeyBase64),
    publicKey: importX25519Public(peerPublicKeyBase64),
  });
  return crypto.createHash("sha256").update("EthernovaChat:v1").update(secret).digest();
}

function encryptChatPayload(plaintext, privateKeyBase64, peerPublicKeyBase64, aad = "") {
  const key = deriveChatKey(privateKeyBase64, peerPublicKeyBase64);
  const nonce = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv("aes-256-gcm", key, nonce);
  if (aad) {
    cipher.setAAD(Buffer.from(aad));
  }
  const ciphertext = Buffer.concat([cipher.update(String(plaintext), "utf8"), cipher.final()]);
  return {
    version: 1,
    algorithm: "X25519+AES-256-GCM",
    nonce: nonce.toString("base64"),
    ciphertext: ciphertext.toString("base64"),
    tag: cipher.getAuthTag().toString("base64"),
  };
}

function decryptChatPayload(payload, privateKeyBase64, peerPublicKeyBase64, aad = "") {
  const key = deriveChatKey(privateKeyBase64, peerPublicKeyBase64);
  const decipher = crypto.createDecipheriv("aes-256-gcm", key, Buffer.from(payload.nonce, "base64"));
  if (aad) {
    decipher.setAAD(Buffer.from(aad));
  }
  decipher.setAuthTag(Buffer.from(payload.tag, "base64"));
  return Buffer.concat([
    decipher.update(Buffer.from(payload.ciphertext, "base64")),
    decipher.final(),
  ]).toString("utf8");
}

function buildChatProfile({ owner, mailboxId, identity, createdAtBlock = 0, profileNonce = "" }) {
  if (!identity || !identity.publicKey) {
    throw new Error("buildChatProfile requires a generated chat identity");
  }
  const profile = {
    version: 1,
    owner,
    mailboxId,
    x25519PublicKey: identity.publicKey,
    x25519PublicKeyHash: identity.publicKeyHash || hashHex(Buffer.from(identity.publicKey, "base64")),
    createdAtBlock,
    profileNonce,
  };
  const canonical = stableStringify(profile);
  return {
    profile,
    canonical,
    contentType: CHAT_PROFILE_CONTENT_TYPE,
    contentHash: hashHex(canonical),
    size: Buffer.byteLength(canonical),
  };
}

function buildChatMessageEnvelope({
  from,
  to,
  toMailboxId,
  sessionId = ZERO_HASH,
  contentRefId = ZERO_HASH,
  payload,
  timestamp = 0,
}) {
  const envelope = {
    version: 1,
    from,
    to,
    toMailboxId,
    sessionId,
    contentRefId,
    payload,
    timestamp,
  };
  const canonical = stableStringify(envelope);
  return {
    envelope,
    canonical,
    contentType: CHAT_MESSAGE_CONTENT_TYPE,
    payloadHash: hashHex(canonical),
    size: Buffer.byteLength(canonical),
  };
}

function buildContentRefInput({
  contentHash,
  size,
  contentType,
  availabilityProof = "",
  rentPrepay = 1n,
  expiryBlock = 0n,
}) {
  const typeBytes = Buffer.from(contentType || "", "utf8");
  const proofBytes = Buffer.from(availabilityProof || "", "utf8");
  return `0x01${wordHex(contentHash)}${wordHex(size)}${wordHex(typeBytes.length)}${wordHex(proofBytes.length)}${wordHex(rentPrepay)}${wordHex(expiryBlock)}${typeBytes.toString("hex")}${proofBytes.toString("hex")}`;
}

function buildCreateMailboxInput({
  capacityLimit = 256n,
  retentionPolicy = 0n,
  retentionBlocks = 0n,
  minPostageWei = 0n,
  aclMode = 0n,
  expiryBlock = 0n,
  rentPrepay = 0n,
  acl = [],
} = {}) {
  const aclTail = acl.map(addressWord).join("");
  return `0x01${wordHex(capacityLimit)}${wordHex(retentionPolicy)}${wordHex(retentionBlocks)}${wordHex(minPostageWei)}${wordHex(aclMode)}${wordHex(expiryBlock)}${wordHex(rentPrepay)}${wordHex(acl.length)}${aclTail}`;
}

function buildMailboxSendInput(mailboxId, payloadHash, postage = 0n) {
  return `0x01${wordHex(mailboxId)}${wordHex(payloadHash)}${wordHex(postage)}`;
}

function buildOpenChatSessionInput(counterparty, timeoutBlocks, disputeRules = ZERO_HASH, rentPrepay = 0n) {
  return `0x01${addressWord(counterparty)}${wordHex(1n)}${wordHex(timeoutBlocks)}${wordHex(disputeRules)}${wordHex(rentPrepay)}`;
}

async function postJson(url, payload) {
  if (typeof fetch === "function") {
    const res = await fetch(url, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      throw new Error(`HTTP ${res.status} from ${url}`);
    }
    return res.json();
  }
  return new Promise((resolve, reject) => {
    const target = new URL(url);
    const lib = target.protocol === "https:" ? https : http;
    const req = lib.request(
      target,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
      },
      (res) => {
        let data = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          data += chunk;
        });
        res.on("end", () => {
          try {
            resolve(JSON.parse(data));
          } catch (err) {
            reject(err);
          }
        });
      },
    );
    req.on("error", reject);
    req.write(JSON.stringify(payload));
    req.end();
  });
}

module.exports = {
  CHAIN_ID,
  DOMAIN_PREFIXES,
  PRECOMPILES,
  CHAT_MESSAGE_CONTENT_TYPE,
  CHAT_PROFILE_CONTENT_TYPE,
  ZERO_HASH,
  NovaProvider,
  buildDomainInitcode,
  buildChatMessageEnvelope,
  buildChatProfile,
  buildContentRefInput,
  buildCreateMailboxInput,
  domainRuntimeBytecode,
  buildMailboxSendInput,
  buildOpenChatSessionInput,
  decryptChatPayload,
  deriveChatKey,
  encryptChatPayload,
  generateChatIdentity,
  hashHex,
  stableStringify,
};
