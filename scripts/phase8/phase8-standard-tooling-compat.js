"use strict";

// Phase 8  -  Scenario R: Compatibility with standard Ethereum tooling.
//
// Validates that the eth_* RPC surface remains intact after nova_* was
// added, and that an ethers v6 provider still functions if available.

const path = require("path");
const H = require("./phase8-helpers");

const RPC = H.envString("PHASE8_RPC_URL", "http://127.0.0.1:8545");
const EXPECTED_CHAIN_ID_DEC = H.envNumber("PHASE8_CHAIN_ID", 121526);
const REPORT_DIR = H.envString("PHASE8_REPORT_DIR_RUN", path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual"));

const suite = new H.Suite("phase8-standard-tooling-compat");

(async () => {
  console.log("========================================================================");
  console.log(" Phase 8 - Scenario R: Standard Tooling Compatibility");
  console.log("========================================================================");
  console.log(`RPC: ${RPC}`);

  // Pre-condition: every eth_* method must work AFTER nova_* was called once,
  // i.e. namespace co-existence holds.
  await H.rpcResult(RPC, "nova_getDomain", [H.ZERO_ADDR]).catch(() => {});

  await suite.step("R.eth_chainId still returns expected value", async () => {
    const cid = await H.rpcResult(RPC, "eth_chainId", []);
    const dec = parseInt(cid, 16);
    if (dec !== EXPECTED_CHAIN_ID_DEC) throw new Error(`got ${dec} expected ${EXPECTED_CHAIN_ID_DEC}`);
    return cid;
  });

  await suite.step("R.eth_blockNumber returns hex string", async () => {
    const r = await H.rpcResult(RPC, "eth_blockNumber", []);
    H.assertHex(r, "eth_blockNumber");
    return r;
  });

  await suite.step("R.eth_getBalance returns hex string", async () => {
    const r = await H.rpcResult(RPC, "eth_getBalance", [H.ZERO_ADDR, "latest"]);
    H.assertHex(r, "eth_getBalance");
    return r;
  });

  await suite.step("R.eth_getCode returns hex string", async () => {
    const r = await H.rpcResult(RPC, "eth_getCode", [H.ZERO_ADDR, "latest"]);
    H.assertHex(r, "eth_getCode");
    return r;
  });

  await suite.step("R.eth_getBlockByNumber('latest') returns block object", async () => {
    const r = await H.rpcResult(RPC, "eth_getBlockByNumber", ["latest", false]);
    H.assertObject(r, "block");
    H.assertHasKey(r, "hash");
    H.assertHasKey(r, "number");
    H.assertHex(r.hash, "block.hash");
    return `block=${parseInt(r.number, 16)}`;
  });

  await suite.step("R.net_version responds", async () => {
    const r = await H.rpcResult(RPC, "net_version", []);
    if (typeof r !== "string" || r.length === 0) throw new Error(`bad net_version: ${r}`);
    return r;
  });

  await suite.step("R.web3_clientVersion responds", async () => {
    const r = await H.rpcResult(RPC, "web3_clientVersion", []);
    if (typeof r !== "string" || r.length === 0) throw new Error(`bad clientVersion: ${r}`);
    return r;
  });

  await suite.step("R.eth_call to precompile 0x29 (read-only) is callable", async () => {
    // Don't assert success  -  just that it doesn't crash the server.
    const body = await H.rpc(RPC, "eth_call", [
      { from: H.ZERO_ADDR, to: "0x0000000000000000000000000000000000000029", data: "0x" },
      "latest",
    ]);
    if (body.error) {
      H.assertNumber(body.error.code, "error.code");
      return `clean error code=${body.error.code}`;
    }
    H.assertHex(body.result, "eth_call result");
    return `result=${body.result.slice(0, 20)}...`;
  });

  // Optional ethers v6 test if available in node_modules.
  let ethersAvailable = false;
  let ethers;
  try {
    ethers = require("ethers");
    ethersAvailable = true;
  } catch (_) {
    ethersAvailable = false;
  }
  if (!ethersAvailable) {
    suite.skip("R.ethers v6 provider compat", "ethers not installed (npm install ethers in scripts/phase8 if desired)");
  } else {
    const ethersProvider = new ethers.JsonRpcProvider(RPC);
    await suite.step("R.ethers getNetwork().chainId matches", async () => {
      const net = await ethersProvider.getNetwork();
      const dec = Number(net.chainId);
      if (dec !== EXPECTED_CHAIN_ID_DEC) throw new Error(`got ${dec} expected ${EXPECTED_CHAIN_ID_DEC}`);
      return `chainId=${dec}`;
    });
    await suite.step("R.ethers getBlockNumber works", async () => {
      const bn = await ethersProvider.getBlockNumber();
      H.assertNumber(bn, "blockNumber");
      return `block=${bn}`;
    });
    await suite.step("R.ethers getBalance works", async () => {
      const bal = await ethersProvider.getBalance(H.ZERO_ADDR);
      // Returns BigInt in v6.
      return `balance=${bal.toString()}`;
    });
    await suite.step("R.ethers getCode works", async () => {
      const code = await ethersProvider.getCode(H.ZERO_ADDR);
      H.assertHex(code, "code");
      return `code-len=${(code.length - 2) / 2}`;
    });

    // Use ethers to call nova_* via send()  -  this is what real apps do.
    await suite.step("R.ethers send('nova_getDomain') matches raw", async () => {
      const r = await ethersProvider.send("nova_getDomain", [H.ZERO_ADDR]);
      H.assertObject(r, "domain");
      if (r.domain !== 0) throw new Error(`expected domain=0 for EOA, got ${r.domain}`);
      return `domain=${r.domain}`;
    });
  }

  suite.printFooter();
  H.writeJson(path.join(REPORT_DIR, "standard-tooling-compat.json"), suite.summarize());

  const summary = suite.summarize();
  if (summary.counts.fail > 0) process.exit(1);
  process.exit(0);
})().catch((err) => {
  console.error("Fatal:", err && err.stack ? err.stack : err);
  process.exit(1);
});
