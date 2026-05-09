"use strict";

const sdk = require("../nova-sdk");

async function deployDomainRuntime(hre, runtimeBytecode, domain = 1, signer) {
  const [defaultSigner] = signer ? [signer] : await hre.ethers.getSigners();
  const tx = await defaultSigner.sendTransaction({
    data: sdk.buildDomainInitcode(domain, runtimeBytecode),
  });
  const receipt = await tx.wait();
  return {
    address: receipt.contractAddress,
    transactionHash: receipt.transactionHash,
    receipt,
  };
}

async function deployArtifactDomain(hre, artifactName, domain = 1, signer) {
  const artifact = await hre.artifacts.readArtifact(artifactName);
  if (!artifact.deployedBytecode || artifact.deployedBytecode === "0x") {
    throw new Error(`${artifactName} has empty deployedBytecode`);
  }
  return deployDomainRuntime(hre, artifact.deployedBytecode, domain, signer);
}

try {
  const { extendEnvironment } = require("hardhat/config");
  extendEnvironment((hre) => {
    hre.nova = {
      ...sdk,
      deployDomainRuntime: (runtimeBytecode, domain, signer) =>
        deployDomainRuntime(hre, runtimeBytecode, domain, signer),
      deployArtifactDomain: (artifactName, domain, signer) =>
        deployArtifactDomain(hre, artifactName, domain, signer),
      provider: (rpcUrl) => new sdk.NovaProvider(rpcUrl || hre.network.config.url),
    };
  });
} catch (_) {
  // Allow requiring this file from plain Node scripts where Hardhat is absent.
}

module.exports = {
  ...sdk,
  deployDomainRuntime,
  deployArtifactDomain,
};
