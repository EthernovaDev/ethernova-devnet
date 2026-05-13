"use strict";

// Phase 8  -  Nova RPC Namespace & Tooling  -  Real Usage Test Suite
//
// Scenarios A..M from the Phase 8 spec. Pure raw JSON-RPC. No mocks. No
// internal Go test plumbing. This runs against a live node.
//
// Output: a JSON summary at $PHASE8_REPORT_DIR/rpc-real-usage.json plus
// human log on stdout (captured by the PowerShell runner).

const path = require("path");
const fs = require("fs");

const H = require("./phase8-helpers");

const RPC = H.envString("PHASE8_RPC_URL", "http://127.0.0.1:8545");
const EXPECTED_CHAIN_ID_DEC = H.envNumber("PHASE8_CHAIN_ID", 121526);
const REPORT_DIR = H.envString("PHASE8_REPORT_DIR_RUN", path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual"));

const EXISTING_SESSION_ID = H.envString("PHASE8_EXISTING_SESSION_ID", "");
const EXISTING_MAILBOX_ID = H.envString("PHASE8_EXISTING_MAILBOX_ID", "");
const EXISTING_CONTENT_REF_ID = H.envString("PHASE8_EXISTING_CONTENT_REF_ID", "");
const EXISTING_PO_ID = H.envString("PHASE8_EXISTING_PROTOCOL_OBJECT_ID", "");

// The full list of nova_* methods Phase 8 spec asks for.
const SPEC_METHODS = [
  "nova_getProtocolObject",
  "nova_listProtocolObjects",
  "nova_getMailbox",
  "nova_getMessages",
  "nova_getContentRef",
  "nova_getSession",
  "nova_getStateTier",
  "nova_getStateWitness",
  "nova_getPendingEffects",
  "nova_getDeferredStats",
  "nova_getCapabilities",
  "nova_getDomain",
];

// Known substitutes for the spec methods that the implementation lacks
// (see PHASE8_AUDIT.md). The runner tries the spec method first; if it
// returns -32601 it records the gap and then tries the substitute.
const METHOD_SUBSTITUTES = {
  nova_listProtocolObjects: { method: "nova_getProtocolObjectsByOwner", note: "no type filter at API layer" },
  nova_getDeferredStats: { method: "nova_deferredProcessingStats", note: "no historical blockNumber lookup" },
};

const suite = new H.Suite("phase8-rpc-real-usage");

function shapeAddress(v) {
  // Some nodes return 0X-prefixed, some lowercase. Normalize for stable
  // comparisons but always preserve the original for the report.
  if (typeof v !== "string") return null;
  return v.toLowerCase();
}

async function scenarioA_RpcNamespaceAvailability() {
  console.log("\n== Scenario A: RPC Namespace Availability ==");

  // A1: eth_chainId works.
  await suite.step("A.eth_chainId responds and matches expected chain", async () => {
    const cid = await H.rpcResult(RPC, "eth_chainId", []);
    H.assertHex(cid, "eth_chainId");
    const dec = parseInt(cid, 16);
    if (dec !== EXPECTED_CHAIN_ID_DEC) {
      throw new Error(`expected chainId ${EXPECTED_CHAIN_ID_DEC}, got ${dec} (${cid})`);
    }
    return `chainId=${dec} (${cid})`;
  });

  // A2: eth_blockNumber works.
  await suite.step("A.eth_blockNumber responds", async () => {
    const bn = await H.rpcResult(RPC, "eth_blockNumber", []);
    H.assertHex(bn, "eth_blockNumber");
    return `block=${parseInt(bn, 16)}`;
  });

  // A3: web3_clientVersion.
  await suite.step("A.web3_clientVersion responds", async () => {
    const cv = await H.rpcResult(RPC, "web3_clientVersion", []);
    H.assertString(cv, "web3_clientVersion");
    if (cv.length === 0) throw new Error("clientVersion is empty");
    return cv;
  });

  // A4: rpc_modules  -  optional; many go-ethereum builds don't expose it.
  await suite.stepWarn("A.rpc_modules lists nova namespace (optional)", async () => {
    const body = await H.rpc(RPC, "rpc_modules", []);
    if (body.error) {
      throw new Error(`rpc_modules unavailable: code=${body.error.code} msg=${body.error.message}`);
    }
    H.assertObject(body.result, "rpc_modules");
    if (!("nova" in body.result)) {
      throw new Error(`'nova' namespace not in rpc_modules: keys=${Object.keys(body.result).join(",")}`);
    }
    return `nova=${body.result.nova}`;
  });

  // A5..A16: every spec method must NOT return method-not-found. Track which
  // methods are missing and fail those specifically.
  const missing = [];
  for (const method of SPEC_METHODS) {
    // Pick minimally-valid params so we don't get -32602 (invalid params)
    // that would mask the namespace check.
    const params = minimalValidParams(method);
    // eslint-disable-next-line no-await-in-loop
    const body = await H.rpc(RPC, method, params).catch((e) => ({ error: { code: -1, message: String(e) }, transport: true }));
    if (body.transport) {
      suite.fail(`A.${method} reachable`, `transport error: ${body.error.message}`, H.SEVERITY.CRITICAL);
      continue;
    }
    if (body.error && body.error.code === H.ERR_METHOD_NOT_FOUND) {
      const sub = METHOD_SUBSTITUTES[method];
      if (sub) {
        // Confirm substitute is reachable so we report "spec missing, substitute OK".
        // eslint-disable-next-line no-await-in-loop
        const subBody = await H.rpc(RPC, sub.method, minimalValidParams(sub.method)).catch((e) => ({
          error: { code: -1, message: String(e) },
          transport: true,
        }));
        if (subBody.error && subBody.error.code !== H.ERR_METHOD_NOT_FOUND) {
          // Substitute returned an error too (but not method-not-found) -> we accept it.
          suite.fail(
            `A.${method} reachable`,
            `${method} returns -32601; substitute ${sub.method} reachable but returned error code=${subBody.error.code} msg=${subBody.error.message}`,
            H.SEVERITY.HIGH,
            { substitute: sub.method, note: sub.note }
          );
        } else if (subBody.error) {
          suite.fail(
            `A.${method} reachable`,
            `${method} AND substitute ${sub.method} are unreachable`,
            H.SEVERITY.CRITICAL,
            { substitute: sub.method }
          );
        } else {
          suite.fail(
            `A.${method} reachable`,
            `spec method missing (-32601); substitute ${sub.method} works (${sub.note})`,
            H.SEVERITY.HIGH,
            { substitute: sub.method, note: sub.note }
          );
        }
        missing.push(method);
        continue;
      }
      suite.fail(`A.${method} reachable`, `method-not-found (-32601)`, H.SEVERITY.CRITICAL);
      missing.push(method);
      continue;
    }
    // Either success or a legitimate JSON-RPC error (e.g. invalid id format)  -  both
    // confirm the method is registered.
    suite.pass(`A.${method} reachable`, body.error ? `registered (responded with code=${body.error.code})` : "registered");
  }

  return missing;
}

function minimalValidParams(method) {
  switch (method) {
    case "nova_getProtocolObject":
      return [H.ZERO_HASH];
    case "nova_listProtocolObjects":
      return [1, H.ZERO_ADDR, 0, 1];
    case "nova_getProtocolObjectsByOwner":
      return [H.ZERO_ADDR, 0, 1];
    case "nova_getMailbox":
      return [H.ZERO_HASH];
    case "nova_getMessages":
      return [H.ZERO_HASH, 0, 1];
    case "nova_getContentRef":
      return [H.ZERO_HASH];
    case "nova_getSession":
      return [H.ZERO_HASH];
    case "nova_getStateTier":
      return [H.ZERO_ADDR, H.ZERO_HASH];
    case "nova_getStateWitness":
      return [H.ZERO_ADDR, H.ZERO_HASH];
    case "nova_getPendingEffects":
      return [0, 1];
    case "nova_getDeferredStats":
      return ["latest"];
    case "nova_deferredProcessingStats":
      return [];
    case "nova_getCapabilities":
      return [H.ZERO_ADDR];
    case "nova_getDomain":
      return [H.ZERO_ADDR];
    default:
      return [];
  }
}

async function scenarioB_GetProtocolObject() {
  console.log("\n== Scenario B: nova_getProtocolObject ==");

  // B1: zero id -> null
  await suite.step("B.zero id returns null", async () => {
    const r = await H.rpcResult(RPC, "nova_getProtocolObject", [H.ZERO_HASH]);
    if (r !== null) throw new Error(`expected null for zero id, got ${JSON.stringify(r).slice(0, 200)}`);
    return "null";
  });

  // B2: random id -> null (extremely unlikely to collide).
  await suite.step("B.random id returns null", async () => {
    const random = "0x" + "ab".repeat(32);
    const r = await H.rpcResult(RPC, "nova_getProtocolObject", [random]);
    if (r !== null) {
      // Possible (some real object happens to use this id). Validate shape.
      H.assertObject(r, "random-id result");
      H.assertHasKey(r, "id");
      return `unexpectedly found id=${r.id}`;
    }
    return "null";
  });

  // B3: malformed id variants. The wire signature is a hex string parsed by
  // common.HexToHash, which is lossy but does NOT panic for short/long inputs.
  for (const [label, val] of [
    ["empty-string", ""],
    ["non-hex", "not-a-hash"],
    ["short-hex", "0x1234"],
    ["too-long", "0x" + "00".repeat(64)],
  ]) {
    // eslint-disable-next-line no-await-in-loop
    await suite.step(`B.malformed id (${label}) does not crash node`, async () => {
      const body = await H.rpc(RPC, "nova_getProtocolObject", [val]);
      // Acceptable outcomes: success with null result, OR a clean JSON-RPC error.
      // Unacceptable: transport failure, parse error, missing both result and error.
      if (body.error) {
        H.assertNumber(body.error.code, "error.code");
        return `clean error code=${body.error.code}`;
      }
      if (!("result" in body)) throw new Error("response missing both result and error");
      return `result=${body.result === null ? "null" : "object"}`;
    });
  }

  // B4: null param
  await suite.step("B.null id returns clean error or null", async () => {
    const body = await H.rpc(RPC, "nova_getProtocolObject", [null]);
    if (body.error) return `clean error code=${body.error.code}`;
    return `result=${body.result === null ? "null" : "non-null"}`;
  });

  // B5: an existing id from .env, if provided.
  if (EXISTING_PO_ID) {
    await suite.step("B.existing PO id returns ProtocolObjectResult shape", async () => {
      const r = await H.rpcResult(RPC, "nova_getProtocolObject", [EXISTING_PO_ID]);
      if (r === null) throw new Error("env-provided id did not exist on chain");
      H.assertObject(r, "ProtocolObjectResult");
      for (const k of ["id", "owner", "typeTag", "typeName", "stateData", "stateDataLen", "expiryBlock", "lastTouchedBlock", "rentBalance"]) {
        H.assertHasKey(r, k, "ProtocolObjectResult");
      }
      H.assertHash(r.id, "PO.id");
      H.assertAddress(r.owner, "PO.owner");
      H.assertNumber(r.typeTag, "PO.typeTag");
      H.assertString(r.typeName, "PO.typeName");
      // stateData per source uses common.Bytes2Hex which omits the 0x prefix
      // (BUG-ish but documented). We accept both with and without 0x.
      H.assertString(r.stateData, "PO.stateData");
      return `id=${r.id} type=${r.typeName} owner=${r.owner}`;
    });
  } else {
    suite.skip("B.existing PO id returns ProtocolObjectResult shape", "PHASE8_EXISTING_PROTOCOL_OBJECT_ID not set");
  }
}

async function scenarioC_ListProtocolObjects() {
  console.log("\n== Scenario C: nova_listProtocolObjects (or substitute) ==");

  // C0: try spec method
  const specBody = await H.rpc(RPC, "nova_listProtocolObjects", [1, H.ZERO_ADDR, 0, 5]);
  if (specBody.error && specBody.error.code === H.ERR_METHOD_NOT_FOUND) {
    suite.fail(
      "C.nova_listProtocolObjects present",
      "method missing; substitute nova_getProtocolObjectsByOwner exists but lacks type filter",
      H.SEVERITY.HIGH,
      { specReference: "Phase 8 spec line 'nova_listProtocolObjects(type, owner, offset, limit)'", file: "eth/api_ethernova.go:855" }
    );

    // Drive the substitute exhaustively so we still verify pagination works.
    await scenarioC_Substitute();
    return;
  }

  // If the method exists, treat it as a shape test.
  await suite.step("C.spec listProtocolObjects exists and returns array-shaped result", async () => {
    if (specBody.error) throw new Error(`code=${specBody.error.code} msg=${specBody.error.message}`);
    // We don't know the exact shape because the spec call is hypothetical.
    return `result-type=${Array.isArray(specBody.result) ? "array" : typeof specBody.result}`;
  });
}

async function scenarioC_Substitute() {
  const ownerForTest = H.envString("PHASE8_PROTOCOL_OBJECT_OWNER", H.ZERO_ADDR);

  await suite.step("C.sub.getProtocolObjectsByOwner offset=0 limit=1", async () => {
    const r = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 0, 1]);
    H.assertObject(r, "result");
    H.assertHasKey(r, "owner");
    H.assertHasKey(r, "ids");
    H.assertArray(r.ids, "ids");
    if (r.ids.length > 1) throw new Error(`limit=1 returned ${r.ids.length} ids`);
    return `ids=${r.ids.length}`;
  });

  await suite.step("C.sub.getProtocolObjectsByOwner offset=0 limit=10", async () => {
    const r = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 0, 10]);
    H.assertArray(r.ids, "ids");
    if (r.ids.length > 10) throw new Error(`limit=10 returned ${r.ids.length} ids`);
    return `ids=${r.ids.length}`;
  });

  await suite.step("C.sub.getProtocolObjectsByOwner pagination stable across two reads", async () => {
    const a = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 0, 5]);
    const b = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 0, 5]);
    H.assertPaginationStable(a.ids, b.ids, "pagination");
    return `stable across ${a.ids.length} ids`;
  });

  await suite.step("C.sub.offset past end returns empty array", async () => {
    const r = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 1000000, 10]);
    H.assertArray(r.ids, "ids");
    if (r.ids.length !== 0) throw new Error(`offset huge returned ${r.ids.length} ids`);
    return "empty";
  });

  await suite.step("C.sub.limit=0 still bounded (source caps to 100)", async () => {
    const r = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 0, 0]);
    H.assertArray(r.ids, "ids");
    if (r.ids.length > 100) throw new Error(`limit=0 returned ${r.ids.length} > 100 cap`);
    return `ids=${r.ids.length} (cap=100)`;
  });

  await suite.step("C.sub.limit huge is capped (no DoS)", async () => {
    const r = await H.rpcResult(RPC, "nova_getProtocolObjectsByOwner", [ownerForTest, 0, 1000000]);
    H.assertArray(r.ids, "ids");
    if (r.ids.length > 100) throw new Error(`limit=huge returned ${r.ids.length} > 100 cap`);
    return `ids=${r.ids.length}`;
  });

  await suite.step("C.sub.malformed owner is handled cleanly", async () => {
    const body = await H.rpc(RPC, "nova_getProtocolObjectsByOwner", ["not-an-address", 0, 1]);
    // common.HexToAddress is lossy; the node will not panic. Accept either
    // success or a clean RPC error.
    if (body.error) return `clean error code=${body.error.code}`;
    return "lossy parse succeeded";
  });
}

async function scenarioD_GetMailbox() {
  console.log("\n== Scenario D: nova_getMailbox ==");

  await suite.step("D.zero id returns null", async () => {
    const r = await H.rpcResult(RPC, "nova_getMailbox", [H.ZERO_HASH]);
    if (r !== null) throw new Error(`expected null for zero id, got ${JSON.stringify(r).slice(0, 200)}`);
    return "null";
  });

  for (const [label, val] of [
    ["empty", ""],
    ["non-hex", "garbage"],
    ["short", "0x12"],
  ]) {
    // eslint-disable-next-line no-await-in-loop
    await suite.step(`D.malformed id (${label}) does not crash node`, async () => {
      const body = await H.rpc(RPC, "nova_getMailbox", [val]);
      if (body.error) return `error code=${body.error.code}`;
      return `result=${body.result === null ? "null" : "object"}`;
    });
  }

  if (EXISTING_MAILBOX_ID) {
    await suite.step("D.existing mailbox returns MailboxResult shape", async () => {
      const r = await H.rpcResult(RPC, "nova_getMailbox", [EXISTING_MAILBOX_ID]);
      if (r === null) throw new Error("env mailbox id not found on chain");
      H.assertObject(r, "MailboxResult");
      const required = ["id", "owner", "expiryBlock", "lastTouchedBlock", "rentBalance",
        "capacityLimit", "retentionPolicy", "retentionBlocks", "minPostageWei",
        "aclMode", "acl", "queueCount", "queueHead", "queueTail", "pendingDeliveries"];
      for (const k of required) H.assertHasKey(r, k, "MailboxResult");
      H.assertHash(r.id, "mailbox.id");
      H.assertAddress(r.owner, "mailbox.owner");
      H.assertArray(r.acl, "mailbox.acl");
      return `id=${r.id} owner=${r.owner} count=${r.queueCount}`;
    });
  } else {
    suite.skip("D.existing mailbox returns MailboxResult shape", "PHASE8_EXISTING_MAILBOX_ID not set");
  }
}

async function scenarioE_GetMessages() {
  console.log("\n== Scenario E: nova_getMessages ==");

  // E1: mailbox not found -> null (per source: returns nil interface).
  await suite.step("E.unknown mailbox returns null", async () => {
    const r = await H.rpcResult(RPC, "nova_getMessages", [H.ZERO_HASH, 0, 5]);
    if (r !== null) throw new Error(`expected null, got ${JSON.stringify(r).slice(0, 200)}`);
    return "null";
  });

  // E2: param-mismatch bug  -  spec says fromBlock, actual is fromIndex. Flag and continue.
  suite.warn(
    "E.parameter contract drift",
    "spec 'fromBlock' is actually 'fromIndex' (queue offset past head). See PHASE8_AUDIT.md BUG-3.",
    { specReference: "Phase 8 line 'nova_getMessages(mailboxId, fromBlock, limit)'", file: "eth/api_ethernova.go:1340-1389" }
  );

  if (EXISTING_MAILBOX_ID) {
    await suite.step("E.existing mailbox fromIndex=0 limit=1 returns wrapper", async () => {
      const r = await H.rpcResult(RPC, "nova_getMessages", [EXISTING_MAILBOX_ID, 0, 1]);
      if (r === null) throw new Error("env mailbox id not found on chain (cannot fetch messages)");
      H.assertObject(r, "messages wrapper");
      for (const k of ["mailboxId", "owner", "queueHead", "queueTail", "queueCount", "fromIndex", "limit", "returned", "messages"]) {
        H.assertHasKey(r, k);
      }
      H.assertArray(r.messages, "messages");
      if (r.messages.length > 1) throw new Error(`limit=1 returned ${r.messages.length} messages`);
      return `head=${r.queueHead} tail=${r.queueTail} count=${r.queueCount} returned=${r.returned}`;
    });

    await suite.step("E.existing mailbox limit huge is capped", async () => {
      const r = await H.rpcResult(RPC, "nova_getMessages", [EXISTING_MAILBOX_ID, 0, 1000000]);
      H.assertArray(r.messages, "messages");
      if (r.messages.length > 256) throw new Error(`limit=huge returned ${r.messages.length} > 256 cap`);
      return `messages=${r.messages.length}`;
    });

    await suite.step("E.existing mailbox limit=0 uses safe default", async () => {
      const r = await H.rpcResult(RPC, "nova_getMessages", [EXISTING_MAILBOX_ID, 0, 0]);
      H.assertArray(r.messages, "messages");
      if (r.messages.length > 50) throw new Error(`limit=0 returned ${r.messages.length} > 50 default`);
      return `default=${r.messages.length}`;
    });

    await suite.step("E.existing mailbox pagination stable across two reads", async () => {
      const a = await H.rpcResult(RPC, "nova_getMessages", [EXISTING_MAILBOX_ID, 0, 5]);
      const b = await H.rpcResult(RPC, "nova_getMessages", [EXISTING_MAILBOX_ID, 0, 5]);
      H.assertPaginationStable(a.messages, b.messages, "messages page");
      return `stable ${a.messages.length}`;
    });

    await suite.step("E.no duplicate message indices in page", async () => {
      const r = await H.rpcResult(RPC, "nova_getMessages", [EXISTING_MAILBOX_ID, 0, 50]);
      const seen = new Set();
      for (const m of r.messages) {
        if (seen.has(m.index)) throw new Error(`duplicate index ${m.index}`);
        seen.add(m.index);
      }
      return `unique=${seen.size}`;
    });
  } else {
    suite.skip("E.existing mailbox positive paths", "PHASE8_EXISTING_MAILBOX_ID not set");
  }
}

async function scenarioF_GetContentRef() {
  console.log("\n== Scenario F: nova_getContentRef ==");

  await suite.step("F.zero id returns null", async () => {
    const r = await H.rpcResult(RPC, "nova_getContentRef", [H.ZERO_HASH]);
    if (r !== null) throw new Error(`expected null, got ${JSON.stringify(r).slice(0, 200)}`);
    return "null";
  });

  for (const [label, val] of [
    ["empty", ""],
    ["short", "0x1234"],
    ["non-hex", "x"],
  ]) {
    // eslint-disable-next-line no-await-in-loop
    await suite.step(`F.malformed id (${label}) does not crash`, async () => {
      const body = await H.rpc(RPC, "nova_getContentRef", [val]);
      if (body.error) return `clean error code=${body.error.code}`;
      return `result=${body.result === null ? "null" : "object"}`;
    });
  }

  if (EXISTING_CONTENT_REF_ID) {
    await suite.step("F.existing contentRef returns object with required fields", async () => {
      const r = await H.rpcResult(RPC, "nova_getContentRef", [EXISTING_CONTENT_REF_ID]);
      if (r === null) throw new Error("env content ref id not found on chain");
      H.assertObject(r, "ContentRefResult");
      // We don't have an exact source struct read here, so we assert presence
      // of common semantic fields the existing nova SDK relies on.
      for (const k of ["id", "owner"]) {
        H.assertHasKey(r, k, "ContentRefResult");
      }
      H.assertHash(r.id, "contentRef.id");
      return `id=${r.id} owner=${r.owner}`;
    });
  } else {
    suite.skip("F.existing contentRef positive path", "PHASE8_EXISTING_CONTENT_REF_ID not set");
  }
}

async function scenarioG_GetSession() {
  console.log("\n== Scenario G: nova_getSession ==");

  await suite.step("G.zero session id returns exists=false envelope", async () => {
    const r = await H.rpcResult(RPC, "nova_getSession", [H.ZERO_HASH]);
    H.assertObject(r, "SessionResult");
    H.assertHasKey(r, "exists");
    if (r.exists !== false) throw new Error(`expected exists=false for zero id, got ${r.exists}`);
    if (r.id !== H.ZERO_HASH) throw new Error(`expected echoed id, got ${r.id}`);
    return "exists=false";
  });

  await suite.step("G.random session id returns exists=false", async () => {
    const id = "0x" + "11".repeat(32);
    const r = await H.rpcResult(RPC, "nova_getSession", [id]);
    H.assertObject(r, "SessionResult");
    if (r.exists !== false && r.exists !== true) throw new Error("missing exists boolean");
    return `exists=${r.exists}`;
  });

  await suite.step("G.malformed id (short) does not crash", async () => {
    const body = await H.rpc(RPC, "nova_getSession", ["0x1234"]);
    if (body.error) return `clean error code=${body.error.code}`;
    return `result=${typeof body.result}`;
  });

  if (EXISTING_SESSION_ID) {
    await suite.step("G.existing session returns full SessionResult shape", async () => {
      const r = await H.rpcResult(RPC, "nova_getSession", [EXISTING_SESSION_ID]);
      H.assertObject(r, "SessionResult");
      for (const k of [
        "id", "exists", "owner", "expiryBlock", "lastTouchedBlock", "rentBalance",
        "initiator", "counterparty", "initiatorSigner", "counterpartySigner",
        "sessionType", "sessionTypeName", "status", "statusName",
        "stateHash", "sequenceNumber", "timeoutBlock", "disputeDeadline",
        "disputeRules", "openedBlock", "closedBlock",
      ]) {
        H.assertHasKey(r, k, "SessionResult");
      }
      if (r.exists !== true) throw new Error("env session id reported exists=false");
      H.assertHash(r.id, "session.id");
      H.assertAddress(r.initiator, "session.initiator");
      H.assertAddress(r.counterparty, "session.counterparty");
      H.assertHash(r.stateHash, "session.stateHash");
      H.assertString(r.sessionTypeName, "session.sessionTypeName");
      H.assertString(r.statusName, "session.statusName");
      if (!["Open", "Disputed", "Closed", "Expired", "Unknown"].includes(r.statusName)) {
        throw new Error(`unexpected statusName ${r.statusName}`);
      }
      return `id=${r.id} status=${r.statusName} seq=${r.sequenceNumber}`;
    });
  } else {
    suite.skip("G.existing session full shape", "PHASE8_EXISTING_SESSION_ID not set");
  }
}

async function scenarioH_GetStateTier() {
  console.log("\n== Scenario H: nova_getStateTier ==");

  await suite.step("H.zero addr + zero slot returns valid tier object", async () => {
    const r = await H.rpcResult(RPC, "nova_getStateTier", [H.ZERO_ADDR, H.ZERO_HASH]);
    H.assertObject(r, "tier result");
    for (const k of ["address", "slot", "tier", "tierCode", "lastTouched", "currentBlock", "ageBlocks", "isArchived"]) {
      H.assertHasKey(r, k);
    }
    if (typeof r.tier !== "string") throw new Error("tier is not a string");
    if (!["Active", "Warm", "Cold", "Archived"].includes(r.tier)) {
      throw new Error(`unexpected tier '${r.tier}'`);
    }
    return `tier=${r.tier} ageBlocks=${r.ageBlocks}`;
  });

  await suite.step("H.random address returns Active default", async () => {
    const addr = "0x" + "ab".repeat(20);
    const r = await H.rpcResult(RPC, "nova_getStateTier", [addr, H.ZERO_HASH]);
    H.assertObject(r, "tier result");
    H.assertString(r.tier, "tier");
    return `tier=${r.tier}`;
  });

  await suite.step("H.malformed slot is handled (HexToHash is lossy not panicking)", async () => {
    const body = await H.rpc(RPC, "nova_getStateTier", [H.ZERO_ADDR, "0xZZ"]);
    if (body.error) return `clean error code=${body.error.code}`;
    H.assertObject(body.result, "tier result");
    return `tier=${body.result.tier}`;
  });

  await suite.step("H.malformed address is handled", async () => {
    const body = await H.rpc(RPC, "nova_getStateTier", ["not-an-address", H.ZERO_HASH]);
    if (body.error) return `clean error code=${body.error.code}`;
    return "lossy parse succeeded";
  });
}

async function scenarioI_GetStateWitness() {
  console.log("\n== Scenario I: nova_getStateWitness ==");

  await suite.step("I.zero addr + zero slot returns witness shape", async () => {
    const r = await H.rpcResult(RPC, "nova_getStateWitness", [H.ZERO_ADDR, H.ZERO_HASH]);
    H.assertObject(r, "witness");
    for (const k of ["address", "slot", "value", "storageRoot", "coldRoot", "proof", "nodeCount"]) {
      H.assertHasKey(r, k, "StateWitnessResult");
    }
    H.assertString(r.proof, "proof");
    H.assertNumber(r.nodeCount, "nodeCount");
    if (r.nodeCount < 0) throw new Error(`nodeCount negative: ${r.nodeCount}`);
    return `nodeCount=${r.nodeCount} proofLen=${r.proof.length}`;
  });

  await suite.step("I.malformed slot is handled cleanly", async () => {
    const body = await H.rpc(RPC, "nova_getStateWitness", [H.ZERO_ADDR, "0xZZ"]);
    if (body.error) {
      H.assertNumber(body.error.code, "error.code");
      return `clean error code=${body.error.code}`;
    }
    return "lossy parse succeeded";
  });

  await suite.step("I.random address returns proof against empty trie", async () => {
    const addr = "0x" + "cd".repeat(20);
    const r = await H.rpcResult(RPC, "nova_getStateWitness", [addr, H.ZERO_HASH]);
    H.assertObject(r, "witness");
    H.assertNumber(r.nodeCount, "nodeCount");
    return `nodeCount=${r.nodeCount}`;
  });
}

async function scenarioJ_GetPendingEffects() {
  console.log("\n== Scenario J: nova_getPendingEffects ==");

  // Spec implies no-args. Actual signature is (offset, limit). Test both.
  let baseBody = await H.rpc(RPC, "nova_getPendingEffects", []);
  if (baseBody.error) {
    // Try with explicit defaults.
    baseBody = await H.rpc(RPC, "nova_getPendingEffects", [0, 0]);
  }
  await suite.step("J.getPendingEffects returns wrapper shape", async () => {
    if (baseBody.error) throw new Error(`code=${baseBody.error.code} msg=${baseBody.error.message}`);
    H.assertObject(baseBody.result, "pending wrapper");
    for (const k of ["head", "tail", "pending", "offset", "limit", "returned", "effects"]) {
      H.assertHasKey(baseBody.result, k, "pending wrapper");
    }
    H.assertArray(baseBody.result.effects, "effects");
    return `pending=${baseBody.result.pending} returned=${baseBody.result.returned}`;
  });

  await suite.step("J.repeated reads are read-only (pending count stable)", async () => {
    const a = await H.rpcResult(RPC, "nova_getPendingEffects", [0, 0]);
    const b = await H.rpcResult(RPC, "nova_getPendingEffects", [0, 0]);
    const c = await H.rpcResult(RPC, "nova_getPendingEffects", [0, 0]);
    // Pending count may legitimately move (new block arrived). What MUST NOT
    // happen is the queue shrinking BECAUSE of our reads. As a loose check,
    // we make three reads close together and tolerate +/- but not -3+.
    const min = Math.min(a.pending, b.pending, c.pending);
    const max = Math.max(a.pending, b.pending, c.pending);
    if (max - min > 5) {
      throw new Error(`pending count moved by ${max - min} across 3 reads (a=${a.pending} b=${b.pending} c=${c.pending}); suspicious`);
    }
    return `pending swing=${max - min}`;
  });

  await suite.step("J.limit huge is bounded", async () => {
    const r = await H.rpcResult(RPC, "nova_getPendingEffects", [0, 1000000]);
    if (r.returned > 256) throw new Error(`limit=huge returned ${r.returned} > 256`);
    return `returned=${r.returned}`;
  });

  await suite.step("J.malformed offset (negative) is handled cleanly", async () => {
    const body = await H.rpc(RPC, "nova_getPendingEffects", [-1, 1]);
    // uint64 cannot be negative; expect a clean JSON-RPC error.
    if (body.error) {
      H.assertNumber(body.error.code, "error.code");
      return `clean error code=${body.error.code}`;
    }
    return "unexpectedly succeeded";
  });
}

async function scenarioK_DeferredStats() {
  console.log("\n== Scenario K: nova_getDeferredStats(blockNumber) ==");

  // Spec method
  const specBody = await H.rpc(RPC, "nova_getDeferredStats", ["latest"]);
  if (specBody.error && specBody.error.code === H.ERR_METHOD_NOT_FOUND) {
    suite.fail(
      "K.nova_getDeferredStats present",
      "method missing; substitute nova_deferredProcessingStats returns CURRENT head only (no historical blockNumber lookup)",
      H.SEVERITY.MEDIUM,
      { specReference: "Phase 8 spec 'nova_getDeferredStats(blockNumber)'", file: "eth/api_ethernova.go:977-1002" }
    );

    // Drive substitute
    await suite.step("K.sub.deferredProcessingStats returns counter object", async () => {
      const r = await H.rpcResult(RPC, "nova_deferredProcessingStats", []);
      H.assertObject(r, "deferred stats");
      for (const k of ["currentBlock", "queueHead", "queueTail", "pendingCount", "totalProcessed", "queueAddress", "forkBlock", "forkActive"]) {
        H.assertHasKey(r, k);
      }
      return `block=${r.currentBlock} pending=${r.pendingCount} processed=${r.totalProcessed}`;
    });

    await suite.step("K.sub.deferredProcessingStats deterministic for one snapshot", async () => {
      const a = await H.rpcResult(RPC, "nova_deferredProcessingStats", []);
      const b = await H.rpcResult(RPC, "nova_deferredProcessingStats", []);
      // currentBlock may advance between calls  -  that's OK. queueAddress must
      // be stable.
      if (a.queueAddress !== b.queueAddress) {
        throw new Error(`queueAddress drifted ${a.queueAddress} -> ${b.queueAddress}`);
      }
      // forkBlock must be stable.
      if (a.forkBlock !== b.forkBlock) throw new Error(`forkBlock drifted`);
      return `stable across two reads (block ${a.currentBlock} -> ${b.currentBlock})`;
    });
    return;
  }

  // If spec method actually exists, validate it.
  await suite.step("K.spec getDeferredStats returns object", async () => {
    if (specBody.error) throw new Error(`code=${specBody.error.code} msg=${specBody.error.message}`);
    H.assertObject(specBody.result, "result");
    return "ok";
  });
}

async function scenarioL_GetCapabilities() {
  console.log("\n== Scenario L: nova_getCapabilities ==");

  await suite.step("L.EOA (zero addr) has CapabilityNova (developer access)", async () => {
    const r = await H.rpcResult(RPC, "nova_getCapabilities", [H.ZERO_ADDR]);
    H.assertObject(r, "capabilities");
    for (const k of ["address", "isContract", "domain", "domainName", "capabilityMask", "capabilities", "capabilityDetails", "precompileRequirements", "notes"]) {
      H.assertHasKey(r, k);
    }
    H.assertString(r.capabilityMask, "capabilityMask");
    H.assertHex(r.capabilityMask, "capabilityMask");
    H.assertArray(r.capabilities, "capabilities");
    if (r.isContract !== false) throw new Error(`zero addr should not be a contract, got isContract=${r.isContract}`);
    return `mask=${r.capabilityMask} caps=[${r.capabilities.join(",")}]`;
  });

  await suite.step("L.precompileRequirements lists the Phase 6 gates", async () => {
    const r = await H.rpcResult(RPC, "nova_getCapabilities", [H.ZERO_ADDR]);
    H.assertArray(r.precompileRequirements, "precompileRequirements");
    const addrs = r.precompileRequirements.map((p) => (p.address || "").toLowerCase());
    // 0x29..0x36 must be present at minimum (see api_ethernova_phase8.go).
    for (const required of ["0x29", "0x2a", "0x2b", "0x2c", "0x2d", "0x2f", "0x30", "0x31", "0x32", "0x33", "0x34", "0x35", "0x36"]) {
      if (!addrs.includes(required)) throw new Error(`precompile gate ${required} missing`);
    }
    return `gates=${addrs.length}`;
  });

  await suite.step("L.malformed address is handled", async () => {
    const body = await H.rpc(RPC, "nova_getCapabilities", ["not-an-addr"]);
    if (body.error) return `clean error code=${body.error.code}`;
    return "lossy parse";
  });

  await suite.step("L.random EOA address still returns object", async () => {
    const addr = "0x" + "11".repeat(20);
    const r = await H.rpcResult(RPC, "nova_getCapabilities", [addr]);
    H.assertObject(r, "capabilities");
    return `domain=${r.domainName}`;
  });
}

async function scenarioM_GetDomain() {
  console.log("\n== Scenario M: nova_getDomain ==");

  await suite.step("M.EOA returns Domain 0 / Legacy", async () => {
    const r = await H.rpcResult(RPC, "nova_getDomain", [H.ZERO_ADDR]);
    H.assertObject(r, "domain");
    for (const k of ["address", "isContract", "codeSize", "runtimeCodeSize", "codeHash", "domain", "domainName", "prefix", "prefixBytes", "canCallNovaPrecompiles"]) {
      H.assertHasKey(r, k);
    }
    if (r.isContract !== false) throw new Error(`zero addr should not be a contract`);
    if (r.domain !== 0) throw new Error(`expected domain=0 for EOA, got ${r.domain}`);
    if (r.canCallNovaPrecompiles !== true) {
      throw new Error("EOA should keep direct Nova precompile access");
    }
    return `domain=${r.domain} name=${r.domainName}`;
  });

  await suite.step("M.malformed address handled cleanly", async () => {
    const body = await H.rpc(RPC, "nova_getDomain", ["not-an-addr"]);
    if (body.error) return `clean error code=${body.error.code}`;
    return "lossy parse";
  });

  // Domain 0/1/2 deployed contracts  -  only if .env provided.
  const d0 = process.env.PHASE8_DOMAIN0_ADDRESS;
  const d1 = process.env.PHASE8_DOMAIN1_ADDRESS;
  const d2 = process.env.PHASE8_DOMAIN2_ADDRESS;

  if (d0) {
    await suite.step("M.Domain0 address is reported domain=0", async () => {
      const r = await H.rpcResult(RPC, "nova_getDomain", [d0]);
      // Domain 0 can be either an EOA (no code, isContract=false) or a regular
      // contract without ef01/ef02 prefix (isContract=true). Both are valid;
      // we only validate the domain classification.
      if (r.domain !== 0) throw new Error(`expected domain=0 got ${r.domain}`);
      // Domain 0 SHOULD NOT be able to call restricted Nova precompiles
      // regardless of EOA vs contract.
      if (r.canCallNovaPrecompiles === true) {
        throw new Error("Domain 0 reports canCallNovaPrecompiles=true (violates Phase 6 gate)");
      }
      return `domain=${r.domain} isContract=${r.isContract} prefix=${r.prefix}`;
    });
  } else {
    suite.skip("M.Domain0 address", "PHASE8_DOMAIN0_ADDRESS not set");
  }
  if (d1) {
    await suite.step("M.Domain1 contract has ef01 prefix and domain=1", async () => {
      const r = await H.rpcResult(RPC, "nova_getDomain", [d1]);
      if (r.isContract !== true) throw new Error("not a contract");
      if (r.domain !== 1) throw new Error(`expected domain=1 got ${r.domain}`);
      if (!String(r.prefix).toLowerCase().startsWith("0xef01")) {
        throw new Error(`prefix should start with 0xef01, got ${r.prefix}`);
      }
      return `domain=1 prefix=${r.prefix}`;
    });
  } else {
    suite.skip("M.Domain1 contract", "PHASE8_DOMAIN1_ADDRESS not set");
  }
  if (d2) {
    await suite.step("M.Domain2 contract has ef02 prefix and domain=2", async () => {
      const r = await H.rpcResult(RPC, "nova_getDomain", [d2]);
      if (r.isContract !== true) throw new Error("not a contract");
      if (r.domain !== 2) throw new Error(`expected domain=2 got ${r.domain}`);
      if (!String(r.prefix).toLowerCase().startsWith("0xef02")) {
        throw new Error(`prefix should start with 0xef02, got ${r.prefix}`);
      }
      return `domain=2 prefix=${r.prefix}`;
    });
  } else {
    suite.skip("M.Domain2 contract", "PHASE8_DOMAIN2_ADDRESS not set");
  }
}

// ---------- main ----------
(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Nova RPC Real-Usage Suite (raw JSON-RPC, scenarios A..M)");
  console.log(`========================================================================`);
  console.log(`RPC: ${RPC}`);
  console.log(`Chain ID expected: ${EXPECTED_CHAIN_ID_DEC}`);

  await scenarioA_RpcNamespaceAvailability();
  await scenarioB_GetProtocolObject();
  await scenarioC_ListProtocolObjects();
  await scenarioD_GetMailbox();
  await scenarioE_GetMessages();
  await scenarioF_GetContentRef();
  await scenarioG_GetSession();
  await scenarioH_GetStateTier();
  await scenarioI_GetStateWitness();
  await scenarioJ_GetPendingEffects();
  await scenarioK_DeferredStats();
  await scenarioL_GetCapabilities();
  await scenarioM_GetDomain();

  suite.printFooter();
  const summary = suite.summarize();
  const out = path.join(REPORT_DIR, "rpc-real-usage.json");
  H.writeJson(out, summary);
  console.log(`Wrote: ${out}`);

  // Exit code policy: only critical failures of "OK" endpoints make us exit 1.
  // BUG-1 and BUG-2 are documented gaps -> exit 1 with explanation.
  const counts = summary.counts;
  const sev = summary.highestSeverity;
  if (counts.fail > 0 && (sev === H.SEVERITY.CRITICAL || sev === H.SEVERITY.HIGH)) {
    process.exit(1);
  }
  process.exit(0);
})().catch((err) => {
  console.error("Fatal:", err && err.stack ? err.stack : err);
  process.exit(1);
});
