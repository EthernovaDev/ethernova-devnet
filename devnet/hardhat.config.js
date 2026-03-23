require("@nomicfoundation/hardhat-toolbox");

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.24",
  networks: {
    ethernova_devnet: {
      url: process.env.RPC_URL || "https://devrpc.ethnova.net",
      chainId: 121526,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
    ethernova_local: {
      url: "http://127.0.0.1:8545",
      chainId: 121526,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },
  etherscan: {
    apiKey: {
      ethernova_devnet: "no-api-key-needed",
    },
    customChains: [
      {
        network: "ethernova_devnet",
        chainId: 121526,
        urls: {
          apiURL: "https://devexplorer.ethnova.net/api",
          browserURL: "https://devexplorer.ethnova.net",
        },
      },
    ],
  },
};
