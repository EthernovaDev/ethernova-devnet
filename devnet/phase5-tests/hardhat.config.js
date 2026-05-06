require("dotenv").config();
require("@nomicfoundation/hardhat-toolbox");

const PRIMARY_RPC = process.env.PRIMARY_RPC || "http://127.0.0.1:8545";
const PRIMARY_CHAIN_ID = parseInt(process.env.PRIMARY_CHAIN_ID || "121526", 10);
const PRIVATE_KEY = process.env.PRIVATE_KEY || "";

if (!PRIVATE_KEY || !/^0x[0-9a-fA-F]{64}$/.test(PRIVATE_KEY)) {
  console.warn(
    "\x1b[33m[hardhat.config] WARNING: PRIVATE_KEY not set or invalid in .env." +
      " Hardhat scripts that need a signer will fail.\x1b[0m"
  );
}

module.exports = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: { enabled: true, runs: 200 },
    },
  },
  networks: {
    ethernova: {
      url: PRIMARY_RPC,
      chainId: PRIMARY_CHAIN_ID,
      accounts: PRIVATE_KEY && /^0x[0-9a-fA-F]{64}$/.test(PRIVATE_KEY) ? [PRIVATE_KEY] : [],
      timeout: 120_000,
    },
  },
};
