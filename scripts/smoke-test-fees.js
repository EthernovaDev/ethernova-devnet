(function () {
  function toHexOrNull(val) {
    return val === null || val === undefined ? null : val.toString();
  }

  this.runSmoke = function (vault, minerAddrInput, pass) {
    const vaultAddr = vault;
    var minerAddr = minerAddrInput;

    if (!minerAddr || minerAddr.toLowerCase() === vaultAddr.toLowerCase()) {
      minerAddr = personal.newAccount(pass);
    }

    const sender = minerAddr;
    const receiver = personal.newAccount(pass);

    personal.unlockAccount(sender, pass, 300);
    personal.unlockAccount(receiver, pass, 300);

    miner.setGasPrice(0);
    miner.setEtherbase(minerAddr);
    miner.start();
    var target = eth.blockNumber + 2;
    var startWait = new Date().getTime();
    while (eth.blockNumber < target && (new Date().getTime() - startWait) < 60000) {
      if (!eth.mining) {
        miner.start();
      }
      eth.blockNumber;
    }
    if (eth.blockNumber < target) {
      return JSON.stringify({
        ok: false,
        error: "timeout waiting for funding blocks",
        blockNumber: eth.blockNumber
      });
    }

    const before = eth.getBalance(vaultAddr);

    const base = eth.getBlock("pending").baseFeePerGas || eth.gasPrice;
    const tip = web3.toBigNumber("2000000000"); // 2 gwei
    const maxFee = base.mul(10).add(tip);

    miner.start();
    const txHash = eth.sendTransaction({
      from: sender,
      to: receiver,
      gas: 21000,
      maxFeePerGas: maxFee,
      maxPriorityFeePerGas: tip,
      value: 0
    });

    var receipt = null;
    var waitTx = new Date().getTime();
    while (receipt === null && (new Date().getTime() - waitTx) < 90000) {
      if (!eth.mining) {
        miner.start();
      }
      receipt = eth.getTransactionReceipt(txHash);
      if (receipt) {
        break;
      }
      eth.blockNumber;
    }
    if (!receipt) {
      return JSON.stringify({
        ok: false,
        error: "timeout waiting for receipt",
        txHash: txHash
      });
    }

    const block = eth.getBlock(receipt.blockNumber);
    const after = eth.getBalance(vaultAddr);

    const expectedDelta = block.baseFeePerGas.mul(receipt.gasUsed);
    const delta = after.sub(before);

    const beforeStr = before.toFixed ? before.toFixed() : before.toString();
    const afterStr = after.toFixed ? after.toFixed() : after.toString();
    const deltaStr = delta.toFixed ? delta.toFixed() : delta.toString();
    const expectedStr = expectedDelta.toFixed ? expectedDelta.toFixed() : expectedDelta.toString();

    return JSON.stringify({
      ok: true,
      txHash: txHash,
      blockNumber: receipt.blockNumber,
      txType: receipt.type,
      baseFeePerGas: toHexOrNull(block.baseFeePerGas),
      gasUsed: receipt.gasUsed,
      tipPerGas: toHexOrNull(tip),
      maxFeePerGas: toHexOrNull(maxFee),
      vault: vaultAddr,
      miner: minerAddr,
      before: beforeStr,
      after: afterStr,
      delta: deltaStr,
      expectedDelta: expectedStr
    });
  };
})();
