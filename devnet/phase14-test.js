// Ethernova Devnet - Phase 14: Comprehensive Feature Test Suite
// Tests all features across multiple nodes to verify consensus
const http = require("http");
const https = require("https");

const RPC = "http://127.0.0.1:28545";
const FROM = "0x246Cbae156Cf083F635C0E1a01586b730678f5Cb";
const NODES = [
  { name: "Node1", url: "http://75.86.96.101:9545" },
  { name: "VPS", url: "http://127.0.0.1:28545" },
];

let passed = 0, failed = 0, total = 0;

function rpc(url, method, params) {
  return new Promise((resolve, reject) => {
    const isHttps = url.startsWith("https");
    const mod = isHttps ? https : http;
    const data = JSON.stringify({ jsonrpc: "2.0", method, params: params || [], id: 1 });
    const parsed = new URL(url);
    const req = mod.request({
      hostname: parsed.hostname, port: parsed.port, path: "/",
      method: "POST", headers: { "Content-Type": "application/json" }, timeout: 10000
    }, res => {
      let body = "";
      res.on("data", c => body += c);
      res.on("end", () => { try { resolve(JSON.parse(body)); } catch(e) { reject(body); } });
    });
    req.on("error", reject);
    req.on("timeout", () => { req.destroy(); reject("timeout"); });
    req.write(data);
    req.end();
  });
}

function test(name, fn) {
  return fn().then(r => {
    total++; passed++;
    console.log(`  [PASS] ${name}: ${r}`);
  }).catch(e => {
    total++; failed++;
    console.log(`  [FAIL] ${name}: ${e}`);
  });
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

async function main() {
  console.log("================================================================");
  console.log("  ETHERNOVA DEVNET v1.0.7 - PHASE 14 COMPREHENSIVE TEST SUITE");
  console.log("  " + new Date().toISOString());
  console.log("================================================================\n");

  // === SECTION 1: Core Network ===
  console.log("=== 1. Core Network ===");
  await test("Chain ID", async () => {
    const r = await rpc(RPC, "eth_chainId");
    if (r.result !== "0x1dab6") throw "wrong chainId: " + r.result;
    return "121526";
  });
  await test("Version", async () => {
    const r = await rpc(RPC, "web3_clientVersion");
    if (!r.result.includes("v1.0.7")) throw "wrong version: " + r.result;
    return r.result;
  });
  await test("Mining active", async () => {
    const r = await rpc(NODES[0].url, "eth_mining");
    return r.result ? "yes" : "no";
  });
  await test("Peer count >= 1", async () => {
    const r = await rpc(RPC, "net_peerCount");
    const peers = parseInt(r.result, 16);
    if (peers < 1) throw "no peers: " + peers;
    return peers + " peers";
  });

  // === SECTION 2: RPC Endpoints ===
  console.log("\n=== 2. Custom RPC Endpoints ===");
  const endpoints = [
    "ethernova_forkStatus", "ethernova_chainConfig", "ethernova_nodeHealth",
    "ethernova_evmProfile", "ethernova_adaptiveGas", "ethernova_optimizer",
    "ethernova_callCache", "ethernova_precompiles", "ethernova_executionMode",
    "ethernova_tempoConfig", "ethernova_stateExpiry"
  ];
  for (const ep of endpoints) {
    await test(ep, async () => {
      const r = await rpc(RPC, ep);
      if (r.error) throw r.error.message;
      return "OK";
    });
  }

  // === SECTION 3: Precompile Calls ===
  console.log("\n=== 3. Precompile Calls ===");
  await test("novaBatchHash (0x20)", async () => {
    const r = await rpc(RPC, "eth_call", [{
      to: "0x0000000000000000000000000000000000000020",
      data: "0x00000000000000000000000000000000000000000000000000000000deadbeef"
    }, "latest"]);
    if (r.error) throw r.error.message;
    return r.result.substring(0, 18) + "...";
  });
  await test("novaBatchVerify (0x21)", async () => {
    const r = await rpc(RPC, "eth_call", [{
      to: "0x0000000000000000000000000000000000000021",
      data: "0x0000000000000000000000000000000000000000000000000000000000000001"
    }, "latest"]);
    return r.result ? "OK" : "empty";
  });
  await test("novaFrameApprove (0x23)", async () => {
    const r = await rpc(RPC, "eth_call", [{
      to: "0x0000000000000000000000000000000000000023",
      data: "0x02"
    }, "latest"]);
    if (r.error) throw r.error.message;
    return "OK";
  });
  await test("novaFrameIntrospect (0x24)", async () => {
    const r = await rpc(RPC, "eth_call", [{
      to: "0x0000000000000000000000000000000000000024",
      data: "0x000000000000000000000000000000000000000000000000000000000000000001"
    }, "latest"]);
    return r.result ? "OK" : (r.error ? r.error.message.substring(0, 40) : "empty");
  });

  // === SECTION 4: Contract Deployment ===
  console.log("\n=== 4. Contract Deployment (consensus critical) ===");

  // Simple storage contract
  const storageBytecode = "0x608060405234801561001057600080fd5b5060c78061001f6000396000f3fe6080604052348015600f57600080fd5b506004361060325760003560e01c806360fe47b11460375780636d4ce63c14604f575b600080fd5b604d60048036038101906049919060689565b6065565b005b6055606f565b6040516060919060909565b60405180910390f35b8060008190555050565b60008054905090565b60008135905060868160a9565b92915050565b600060208284031215609d57600080fd5b600060a9848285016079565b91505092915050565b6000819050919050565b60b08160a0565b811460ba57600080fd5b50";

  let contractAddr = null;
  let deployBlock = null;

  await test("Deploy SimpleStorage", async () => {
    const r = await rpc(RPC, "eth_sendTransaction", [{
      from: FROM, data: storageBytecode, gas: "0x100000"
    }]);
    if (r.error) throw r.error.message;
    const txHash = r.result;
    await sleep(15000);
    const receipt = await rpc(RPC, "eth_getTransactionReceipt", [txHash]);
    if (!receipt.result) throw "tx not mined after 15s";
    contractAddr = receipt.result.contractAddress;
    deployBlock = receipt.result.blockNumber;
    return "contract=" + contractAddr.substring(0, 10) + "... block=" + parseInt(deployBlock, 16) + " gas=" + parseInt(receipt.result.gasUsed, 16);
  });

  // === SECTION 5: Contract Interaction ===
  console.log("\n=== 5. Contract Interaction ===");
  if (contractAddr) {
    // store(42) - function selector 0x60fe47b1, value 42 (0x2a)
    await test("Call store(42)", async () => {
      const r = await rpc(RPC, "eth_sendTransaction", [{
        from: FROM, to: contractAddr,
        data: "0x60fe47b1000000000000000000000000000000000000000000000000000000000000002a",
        gas: "0x50000"
      }]);
      if (r.error) throw r.error.message;
      await sleep(15000);
      const receipt = await rpc(RPC, "eth_getTransactionReceipt", [r.result]);
      if (!receipt.result) throw "not mined";
      return "gas=" + parseInt(receipt.result.gasUsed, 16) + " status=" + receipt.result.status;
    });

    // get() - should return 42
    await test("Call get() returns 42", async () => {
      const r = await rpc(RPC, "eth_call", [{
        to: contractAddr,
        data: "0x6d4ce63c"
      }, "latest"]);
      if (r.error) throw r.error.message;
      const val = parseInt(r.result, 16);
      if (val !== 42) throw "expected 42, got " + val;
      return "42";
    });
  }

  // === SECTION 6: Multiple Transfers ===
  console.log("\n=== 6. Batch Transfers (10 txs) ===");
  let txHashes = [];
  await test("Send 10 transfers", async () => {
    for (let i = 0; i < 10; i++) {
      const addr = "0x" + require("crypto").randomBytes(20).toString("hex");
      const r = await rpc(RPC, "eth_sendTransaction", [{
        from: FROM, to: addr, value: "0xDE0B6B3A7640000", gas: "0x5208"
      }]);
      if (r.error) throw r.error.message;
      txHashes.push(r.result);
    }
    return txHashes.length + " sent";
  });

  await sleep(20000);

  await test("All 10 mined", async () => {
    let mined = 0;
    for (const tx of txHashes) {
      const r = await rpc(RPC, "eth_getTransactionReceipt", [tx]);
      if (r.result && r.result.status === "0x1") mined++;
    }
    if (mined < 10) throw "only " + mined + "/10 mined";
    return mined + "/10";
  });

  // === SECTION 7: Consensus Verification ===
  console.log("\n=== 7. Consensus Verification (10 blocks) ===");
  const latestR = await rpc(RPC, "eth_blockNumber");
  const latest = parseInt(latestR.result, 16);
  let consensusPass = 0;

  for (let offset = 0; offset < 10; offset++) {
    const bn = latest - offset;
    const bh = "0x" + bn.toString(16);
    await test("Block " + bn + " hash match", async () => {
      const hashes = [];
      for (const node of NODES) {
        try {
          const r = await rpc(node.url, "eth_getBlockByNumber", [bh, false]);
          if (r.result) hashes.push({ name: node.name, hash: r.result.hash, txs: r.result.transactions.length });
        } catch(e) { /* skip unreachable */ }
      }
      if (hashes.length < 2) throw "only " + hashes.length + " nodes reachable";
      const ref = hashes[0].hash;
      for (const h of hashes) {
        if (h.hash !== ref) throw "MISMATCH " + h.name + "=" + h.hash.substring(0, 18);
      }
      consensusPass++;
      return hashes.map(h => h.name).join("+") + " txs=" + hashes[0].txs;
    });
  }

  // === SECTION 8: Fork Status ===
  console.log("\n=== 8. Fork Configuration ===");
  await test("NovenForkBlock", async () => {
    const r = await rpc(RPC, "ethernova_forkStatus");
    return JSON.stringify(r.result).substring(0, 80) + "...";
  });
  await test("TempoConfig", async () => {
    const r = await rpc(RPC, "ethernova_tempoConfig");
    return "forkBlock=" + r.result.forkBlock + " erc20Gas=" + r.result.erc20Gas;
  });
  await test("StateExpiry", async () => {
    const r = await rpc(RPC, "ethernova_stateExpiry");
    return "forkBlock=" + r.result.forkBlock + " period=" + r.result.expiryPeriod;
  });

  // === RESULTS ===
  console.log("\n================================================================");
  console.log("  RESULTS");
  console.log("================================================================");
  console.log("  Passed: " + passed + "/" + total);
  console.log("  Failed: " + failed + "/" + total);
  console.log("  Consensus: " + consensusPass + "/10 blocks matched");
  console.log("");
  if (failed === 0) {
    console.log("  >>> ALL TESTS PASSED - v1.0.7 VERIFIED <<<");
  } else {
    console.log("  >>> " + failed + " TESTS FAILED <<<");
  }
  console.log("================================================================");
}

main().catch(e => console.log("FATAL:", e));
