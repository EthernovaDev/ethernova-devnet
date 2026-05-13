"use strict";

// Phase 8  -  Scenario N: Malformed RPC Brutality
//
// For every nova_* method that exists, send malformed parameters and verify:
//   1. The node never panics or crashes (still responds to eth_blockNumber after).
//   2. JSON-RPC error responses are well-formed (code:number, message:string).
//   3. Methods never hang (every call has a strict timeout).

const path = require("path");
const H = require("./phase8-helpers");

const RPC = H.envString("PHASE8_RPC_URL", "http://127.0.0.1:8545");
const REPORT_DIR = H.envString("PHASE8_REPORT_DIR_RUN", path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual"));

const suite = new H.Suite("phase8-rpc-malformed");

// Methods we know exist (per audit). We deliberately do NOT include the two
// missing methods (nova_listProtocolObjects, nova_getDeferredStats) here  - 
// they would return -32601 for every call and pollute the malformed-call
// signal.
const REAL_METHODS = [
  { method: "nova_getProtocolObject", validParams: [H.ZERO_HASH] },
  { method: "nova_getProtocolObjectsByOwner", validParams: [H.ZERO_ADDR, 0, 1] },
  { method: "nova_getMailbox", validParams: [H.ZERO_HASH] },
  { method: "nova_getMessages", validParams: [H.ZERO_HASH, 0, 1] },
  { method: "nova_getContentRef", validParams: [H.ZERO_HASH] },
  { method: "nova_getSession", validParams: [H.ZERO_HASH] },
  { method: "nova_getStateTier", validParams: [H.ZERO_ADDR, H.ZERO_HASH] },
  { method: "nova_getStateWitness", validParams: [H.ZERO_ADDR, H.ZERO_HASH] },
  { method: "nova_getPendingEffects", validParams: [0, 1] },
  { method: "nova_deferredProcessingStats", validParams: [] },
  { method: "nova_getCapabilities", validParams: [H.ZERO_ADDR] },
  { method: "nova_getDomain", validParams: [H.ZERO_ADDR] },
];

const MALFORMED_VARIANTS = [
  { label: "no-params", params: [] },
  { label: "too-many-params", params: [H.ZERO_HASH, H.ZERO_HASH, H.ZERO_HASH, H.ZERO_HASH, H.ZERO_HASH] },
  { label: "wrong-type-string-where-num", params: ["latest", "latest", "latest"] },
  { label: "wrong-type-num-where-string", params: [42, 42, 42] },
  { label: "null-element", params: [null] },
  { label: "object-as-param", params: [{ foo: "bar" }] },
  { label: "array-of-arrays", params: [[1, 2, 3]] },
  { label: "negative-number", params: [-1, -1, -1] },
  { label: "huge-number", params: [Number.MAX_SAFE_INTEGER, Number.MAX_SAFE_INTEGER, Number.MAX_SAFE_INTEGER] },
  { label: "invalid-hex", params: ["0xZZZZ", "0xZZZZ"] },
  { label: "very-long-string", params: ["0x" + "a".repeat(10000)] },
  { label: "empty-string", params: ["", "", ""] },
  { label: "boolean", params: [true, false] },
  { label: "scientific-notation-number", params: [1e20, 1e20] },
];

async function nodeStillAlive() {
  const body = await H.rpc(RPC, "eth_blockNumber", [], { timeoutMs: 5000 });
  if (body.error) throw new Error(`eth_blockNumber error: code=${body.error.code} msg=${body.error.message}`);
  H.assertHex(body.result, "eth_blockNumber");
  return parseInt(body.result, 16);
}

(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Scenario N: Malformed RPC Brutality");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  await suite.step("N.node alive before brutality", async () => {
    const bn = await nodeStillAlive();
    return `block=${bn}`;
  });

  let totalCalls = 0;
  let crashes = 0;
  let hangs = 0;
  let invalidEnvelopes = 0;

  for (const { method, validParams } of REAL_METHODS) {
    for (const variant of MALFORMED_VARIANTS) {
      totalCalls++;
      // Each individual call: must respond within 5s (no hang), must have a
      // well-formed JSON-RPC envelope (either result OR error), and the node
      // must still be alive after.
      // eslint-disable-next-line no-await-in-loop
      await suite.step(`N.${method} :: ${variant.label}`, async () => {
        let body;
        try {
          body = await H.rpc(RPC, method, variant.params, { timeoutMs: 5000 });
        } catch (err) {
          if (/Timeout/.test(String(err.message || err))) {
            hangs++;
            throw new Error(`HANG: ${err.message}`);
          }
          // Transport-level failure means the node may be wedged. Confirm
          // immediately by trying eth_blockNumber. If THAT also fails, this
          // is a critical crash.
          try {
            // eslint-disable-next-line no-await-in-loop
            await nodeStillAlive();
          } catch (e2) {
            crashes++;
            throw new Error(`CRASH-AFTER-${variant.label}: ${e2.message}`);
          }
          // Transport hiccup but node alive  -  count as warn.
          return `transport error tolerated: ${err.message}`;
        }
        // Validate envelope.
        if (!body || typeof body !== "object" || body.jsonrpc !== "2.0") {
          invalidEnvelopes++;
          throw new Error(`bad envelope: ${JSON.stringify(body).slice(0, 200)}`);
        }
        if (body.error) {
          if (typeof body.error.code !== "number" || typeof body.error.message !== "string") {
            invalidEnvelopes++;
            throw new Error(`malformed error object: ${JSON.stringify(body.error)}`);
          }
          return `clean error code=${body.error.code}`;
        }
        // Some malformed inputs land in a defaulted state (e.g. wrong type
        // coerced to zero) and return a normal result. That's acceptable  - 
        // the node didn't crash.
        return `accepted (lossy parse), result type=${body.result === null ? "null" : typeof body.result}`;
      }, { severity: H.SEVERITY.HIGH });
    }
  }

  await suite.step("N.node still alive after brutality", async () => {
    const bn = await nodeStillAlive();
    return `block=${bn}`;
  });

  console.log(`Total malformed calls: ${totalCalls}, crashes: ${crashes}, hangs: ${hangs}, invalid envelopes: ${invalidEnvelopes}`);
  suite.printFooter();

  const summary = suite.summarize();
  summary.totalCalls = totalCalls;
  summary.crashes = crashes;
  summary.hangs = hangs;
  summary.invalidEnvelopes = invalidEnvelopes;
  const out = path.join(REPORT_DIR, "malformed-rpc.json");
  H.writeJson(out, summary);
  console.log(`Wrote: ${out}`);

  // Exit policy: any crash or hang is critical.
  if (crashes > 0 || hangs > 0 || invalidEnvelopes > 0) {
    process.exit(1);
  }
  process.exit(0);
})().catch((err) => {
  console.error("Fatal:", err && err.stack ? err.stack : err);
  process.exit(1);
});
