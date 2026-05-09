"use strict";

const http = require("http");
const https = require("https");

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
}

const ZERO_HASH = "0x0000000000000000000000000000000000000000000000000000000000000000";

function normalizeHex(value) {
  if (typeof value !== "string") {
    throw new Error("hex value must be a string");
  }
  return value.startsWith("0x") ? value.slice(2) : value;
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
  ZERO_HASH,
  NovaProvider,
  buildDomainInitcode,
  domainRuntimeBytecode,
};
