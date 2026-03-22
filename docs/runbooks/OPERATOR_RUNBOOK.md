# OPERATOR_RUNBOOK.md

## Scheduled Hard Fork at Block 105000

This runbook covers the mandatory upgrade to Ethernova v1.2.8.
The fork at block 105000 enables Constantinople + Petersburg + Istanbul EVM rules
(SHL/SHR/SAR, CREATE2, EXTCODEHASH, CHAINID, SELFBALANCE, plus Istanbul gas repricing).

No genesis re-init is required. ChainId/networkId remain 121525.

---

## 1. Stop the Node

**Windows:**
```
Stop-Process -Name ethernova
```
Or close the terminal/window running the node.

**Linux:**
```
killall ethernova
```
Or use your process manager (systemd, supervisor, etc).

---

## 2. Backup the Data Directory

**Windows:**
```
robocopy C:\path\to\datadir C:\path\to\backup\datadir /MIR
```

**Linux:**
```
cp -a /path/to/datadir /path/to/backup/datadir
```

---

## 3. Replace the Binary

- Download the v1.2.8 binary for your OS.
- Replace the existing `ethernova`/`ethernova.exe` in your service path.

---

## 4. Verify Genesis & Version

**Windows:**
```
ethernova.exe print-genesis
ethernova.exe sanitycheck --datadir <your-datadir>
```

**Linux:**
```
ethernova print-genesis
ethernova sanitycheck --datadir <your-datadir>
```

- If you see **WRONG GENESIS**, delete the datadir and re-init with the correct `genesis-mainnet.json` (or rely on the embedded genesis).
- Confirm `web3_clientVersion` shows `Ethernova/v1.2.8/...`.

---

## 5. Restart the Node

**Windows:**
```
ethernova.exe --datadir <your-datadir> --networkid 121525 --mine ...
```

**Linux:**
```
ethernova --datadir <your-datadir> --networkid 121525 --mine ...
```

---

## 6. Verify Fork Behavior (pre/post)

**Windows:**
```
.\scripts\verify-fork-windows.ps1 -Endpoint http://127.0.0.1:8545
```

**Linux:**
```
./scripts/verify-fork-linux.sh http://127.0.0.1:8545
```

Expected:
- Pre-fork (block 104999): SHL fails with invalid opcode.
- Post-fork (block 105000): SHL succeeds.

---

## What happens if you do not upgrade

- Nodes that skip the fork will reject post-fork blocks and follow a minority chain.
- ForkID mismatch will cause automatic peer disconnects.

---

**Note:** If you see a genesis hash mismatch, STOP and restore your backup. Do not proceed.
