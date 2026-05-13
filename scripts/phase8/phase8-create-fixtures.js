"use strict";

// phase8-create-fixtures.js
//
// OPTIONAL fixture creator. Only runs when PHASE8_ENABLE_FIXTURE_CREATION=true
// AND PHASE8_PRIVATE_KEY_A is set. Produces a Mailbox and a ContentRef so
// downstream Phase 8 scenarios can drive positive-path lookups against fresh
// on-chain objects without depending on prior devnet state.
//
// What this script does NOT do (intentional - see PHASE8_AUDIT.md):
//   * It does not create a Session. The SDK's buildOpenChatSessionInput is
//     stale (BUG-4: produces 5 words, precompile needs 7). Tests use the
//     PHASE8_EXISTING_SESSION_ID env var instead.
//   * It does not invent precompile encodings. Mailbox / ContentRef inputs
//     come straight from the SDK builders.
//
// Output: $PHASE8_REPORT_DIR_RUN/fixtures.json + create-fixtures.json
// Exit code: always 0 (creation is best-effort).

const fs = require("fs");
const path = require("path");

const H = require("./phase8-helpers");

const ENV_PATH = path.join(__dirname, ".env");
H.loadEnv(ENV_PATH);

const REPORT_DIR = H.envString(
  "PHASE8_REPORT_DIR_RUN",
  path.resolve(__dirname, "..", "..", "reports", "phase8", "real-usage", "manual")
);

const suite = new H.Suite("phase8-create-fixtures");

async function main() {
  console.log("========================================================================");
  console.log(" Phase 8 - Fixture Creation (Mailbox + ContentRef)");
  console.log("========================================================================");

  const fixtures = {
    createdAt: new Date().toISOString(),
    chainId: null,
    creatorAddress: null,
    mailbox: null,
    contentRef: null,
    session: null,
    domain0: null,
    domain1: null,
    domain2: null,
    skipped: { session: "BUG-4: SDK builder stale (5 words vs required 7)" },
    envExport: {},
  };

  const enable = H.envBool("PHASE8_ENABLE_FIXTURE_CREATION", true);
  const rpcUrl = H.envString("PHASE8_RPC_URL", "");
  const pkA = H.envString("PHASE8_PRIVATE_KEY_A", "");
  const chainIdEnv = Number(H.envString("PHASE8_CHAIN_ID", "121526"));

  if (!enable) {
    suite.skip("fixture-creation", "PHASE8_ENABLE_FIXTURE_CREATION=false");
    return finalize(fixtures);
  }
  if (!rpcUrl) {
    suite.skip("fixture-creation", "PHASE8_RPC_URL not set");
    return finalize(fixtures);
  }
  if (!pkA) {
    suite.skip(
      "fixture-creation",
      "PHASE8_PRIVATE_KEY_A not set (fixtures optional; downstream tests will use PHASE8_EXISTING_* env vars)"
    );
    return finalize(fixtures);
  }

  let ethers;
  try {
    ethers = require("ethers");
  } catch (err) {
    // Node only walks UP from this script when resolving modules. If the user
    // ran `npm install` inside scripts/phase8/hardhat-test-project/ (the
    // recommended setup), ethers won't be on the walk path. Use createRequire
    // anchored at hardhat-test-project/package.json so Node honors ethers v6's
    // `exports` field correctly.
    try {
      const { createRequire } = require("module");
      const projectPkg = path.join(__dirname, "hardhat-test-project", "package.json");
      const localRequire = createRequire(projectPkg);
      ethers = localRequire("ethers");
    } catch (err2) {
      suite.fail(
        "ethers-import",
        "ethers is not installed; cannot create fixtures (walked up from " +
          __dirname + " and tried createRequire(" +
          path.join(__dirname, "hardhat-test-project", "package.json") +
          "); inner error: " + (err2 && err2.message),
        H.SEVERITY.MEDIUM,
        { hint: "cd scripts/phase8/hardhat-test-project && npm install" }
      );
      return finalize(fixtures);
    }
  }
  if (typeof ethers.JsonRpcProvider !== "function") {
    suite.fail("ethers-version", "ethers v6 required (JsonRpcProvider not found)", H.SEVERITY.MEDIUM);
    return finalize(fixtures);
  }

  let sdk;
  try {
    const sdkPath = H.envString("PHASE8_SDK_PATH", path.join(__dirname, "..", "..", "devnet", "nova-sdk"));
    sdk = require(path.resolve(sdkPath));
  } catch (err) {
    suite.fail(
      "sdk-import",
      "could not require devnet/nova-sdk: " + (err && err.message),
      H.SEVERITY.MEDIUM
    );
    return finalize(fixtures);
  }

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  try {
    const net = await provider.getNetwork();
    fixtures.chainId = Number(net.chainId);
    if (chainIdEnv && fixtures.chainId !== chainIdEnv) {
      suite.warn(
        "chain-id-mismatch",
        "node chainId " + fixtures.chainId + " does not match PHASE8_CHAIN_ID " + chainIdEnv
      );
    } else {
      suite.pass("chain-id", "chainId=" + fixtures.chainId);
    }
  } catch (err) {
    suite.fail(
      "rpc-network",
      "could not fetch chainId from " + rpcUrl + ": " + (err && err.message),
      H.SEVERITY.HIGH
    );
    return finalize(fixtures);
  }

  let wallet;
  try {
    wallet = new ethers.Wallet(pkA, provider);
    fixtures.creatorAddress = await wallet.getAddress();
    suite.pass("wallet-derive", "address=" + fixtures.creatorAddress);
  } catch (err) {
    suite.fail("wallet-derive", "invalid PHASE8_PRIVATE_KEY_A: " + (err && err.message), H.SEVERITY.HIGH);
    return finalize(fixtures);
  }

  // Mailbox
  try {
    const input = sdk.buildCreateMailboxInput({
      capacityLimit: 64n,
      retentionPolicy: 0n,
      retentionBlocks: 0n,
      minPostageWei: 0n,
      aclMode: 0n,
      expiryBlock: 0n,
      rentPrepay: 0n,
      acl: [],
    });
    const to = sdk.PRECOMPILES.mailboxManager;
    const simResult = await provider.call({
      to,
      from: fixtures.creatorAddress,
      data: input,
    });
    if (!simResult || simResult.length < 66) {
      throw new Error(
        "createMailbox simulation returned " +
          (simResult ? simResult.length : 0) +
          " hex chars; expected >= 66"
      );
    }
    const mailboxId = "0x" + simResult.slice(2, 66);
    const tx = await wallet.sendTransaction({ to, data: input });
    const receipt = await tx.wait();
    if (!receipt || Number(receipt.status) !== 1) {
      throw new Error("createMailbox tx not successful: status=" + (receipt && receipt.status));
    }
    fixtures.mailbox = {
      id: mailboxId,
      txHash: tx.hash,
      blockNumber: Number(receipt.blockNumber),
    };
    fixtures.envExport.PHASE8_EXISTING_MAILBOX_ID = mailboxId;
    suite.pass("create-mailbox", "id=" + mailboxId + " tx=" + tx.hash);
  } catch (err) {
    suite.fail(
      "create-mailbox",
      "mailbox creation failed: " + (err && err.message),
      H.SEVERITY.MEDIUM
    );
  }

  // ContentRef
  try {
    const contentHash = "0x" + "a".repeat(64);
    const input = sdk.buildContentRefInput({
      contentHash,
      size: 1024n,
      contentType: "application/octet-stream",
      availabilityProof: "phase8-fixture",
      rentPrepay: 1n,
      expiryBlock: 0n,
    });
    const to = sdk.PRECOMPILES.contentRegistry;
    const simResult = await provider.call({
      to,
      from: fixtures.creatorAddress,
      data: input,
    });
    if (!simResult || simResult.length < 66) {
      throw new Error(
        "createContentRef simulation returned " +
          (simResult ? simResult.length : 0) +
          " hex chars; expected >= 66"
      );
    }
    const contentRefId = "0x" + simResult.slice(2, 66);
    const tx = await wallet.sendTransaction({ to, data: input });
    const receipt = await tx.wait();
    if (!receipt || Number(receipt.status) !== 1) {
      throw new Error("createContentRef tx not successful: status=" + (receipt && receipt.status));
    }
    fixtures.contentRef = {
      id: contentRefId,
      txHash: tx.hash,
      blockNumber: Number(receipt.blockNumber),
    };
    fixtures.envExport.PHASE8_EXISTING_CONTENT_REF_ID = contentRefId;
    suite.pass("create-content-ref", "id=" + contentRefId + " tx=" + tx.hash);
  } catch (err) {
    suite.fail(
      "create-content-ref",
      "contentRef creation failed: " + (err && err.message),
      H.SEVERITY.MEDIUM
    );
  }

  // Session - skipped (BUG-4)
  suite.skip(
    "create-session",
    "intentional skip - SDK buildOpenChatSessionInput stale (BUG-4). " +
      "Set PHASE8_EXISTING_SESSION_ID if a session already exists on-chain."
  );

  // ----- Domain 0 address -----
  // Domain 0 is any EOA without ef01/ef02 prefix. The deployer wallet is
  // already a perfectly valid Domain 0 - use it unless the user supplied an
  // override in the env.
  const userDomain0 = H.envString("PHASE8_DOMAIN0_ADDRESS", "");
  if (userDomain0) {
    suite.pass("domain0-address", "using user-supplied " + userDomain0);
    fixtures.domain0 = { address: userDomain0, source: "env" };
  } else {
    fixtures.domain0 = { address: fixtures.creatorAddress, source: "deployer-eoa" };
    fixtures.envExport.PHASE8_DOMAIN0_ADDRESS = fixtures.creatorAddress;
    suite.pass("domain0-address", "using deployer EOA " + fixtures.creatorAddress);
  }

  // ----- Domain 1 stub deploy -----
  // Deploys a minimal Domain 1 contract (runtime = 0xef01 + STOP). The contract
  // is never called - tests only inspect its code prefix via nova_getDomain /
  // nova_getCapabilities. User-supplied PHASE8_DOMAIN1_ADDRESS takes priority.
  const userDomain1 = H.envString("PHASE8_DOMAIN1_ADDRESS", "");
  if (userDomain1) {
    suite.pass("domain1-deploy", "skipped - user supplied PHASE8_DOMAIN1_ADDRESS=" + userDomain1);
    fixtures.domain1 = { address: userDomain1, source: "env" };
  } else {
    try {
      const initcode = sdk.buildDomainInitcode(1, "0x00");
      const tx = await wallet.sendTransaction({ data: initcode });
      const receipt = await tx.wait();
      if (!receipt || Number(receipt.status) !== 1) {
        throw new Error("Domain 1 deploy tx status=" + (receipt && receipt.status));
      }
      const addr = receipt.contractAddress;
      if (!addr) throw new Error("Domain 1 deploy receipt has no contractAddress");
      fixtures.domain1 = {
        address: addr,
        txHash: tx.hash,
        blockNumber: Number(receipt.blockNumber),
        source: "auto-deployed",
      };
      fixtures.envExport.PHASE8_DOMAIN1_ADDRESS = addr;
      suite.pass("domain1-deploy", "addr=" + addr + " tx=" + tx.hash);
    } catch (err) {
      suite.fail(
        "domain1-deploy",
        "Domain 1 stub deploy failed: " + (err && err.message),
        H.SEVERITY.MEDIUM
      );
    }
  }

  // ----- Domain 2 stub deploy -----
  const userDomain2 = H.envString("PHASE8_DOMAIN2_ADDRESS", "");
  if (userDomain2) {
    suite.pass("domain2-deploy", "skipped - user supplied PHASE8_DOMAIN2_ADDRESS=" + userDomain2);
    fixtures.domain2 = { address: userDomain2, source: "env" };
  } else {
    try {
      const initcode = sdk.buildDomainInitcode(2, "0x00");
      const tx = await wallet.sendTransaction({ data: initcode });
      const receipt = await tx.wait();
      if (!receipt || Number(receipt.status) !== 1) {
        throw new Error("Domain 2 deploy tx status=" + (receipt && receipt.status));
      }
      const addr = receipt.contractAddress;
      if (!addr) throw new Error("Domain 2 deploy receipt has no contractAddress");
      fixtures.domain2 = {
        address: addr,
        txHash: tx.hash,
        blockNumber: Number(receipt.blockNumber),
        source: "auto-deployed",
      };
      fixtures.envExport.PHASE8_DOMAIN2_ADDRESS = addr;
      suite.pass("domain2-deploy", "addr=" + addr + " tx=" + tx.hash);
    } catch (err) {
      suite.fail(
        "domain2-deploy",
        "Domain 2 stub deploy failed: " + (err && err.message),
        H.SEVERITY.MEDIUM
      );
    }
  }


  return finalize(fixtures);
}

function finalize(fixtures) {
  suite.printFooter();
  const summary = suite.summarize();
  H.writeJson(path.join(REPORT_DIR, "create-fixtures.json"), summary);
  H.writeJson(path.join(REPORT_DIR, "fixtures.json"), fixtures);
  console.log("Wrote: " + path.join(REPORT_DIR, "fixtures.json"));
  console.log("Wrote: " + path.join(REPORT_DIR, "create-fixtures.json"));
  if (fixtures.envExport && Object.keys(fixtures.envExport).length > 0) {
    console.log("\nEnv exports (paste into scripts/phase8/.env for stable IDs):");
    for (const k of Object.keys(fixtures.envExport)) {
      console.log("  " + k + "=" + fixtures.envExport[k]);
    }
  }
  process.exit(0);
}

main().catch((err) => {
  console.error("phase8-create-fixtures crashed:", err && err.stack ? err.stack : err);
  try {
    suite.fail("uncaught", String((err && err.message) || err), H.SEVERITY.HIGH);
    const summary = suite.summarize();
    H.writeJson(path.join(REPORT_DIR, "create-fixtures.json"), summary);
  } catch (_) {}
  process.exit(0);
});
