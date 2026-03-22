# UPGRADE v1.2.8

## Mandatory Upgrade Before Block 105000

Ethernova v1.2.8 activates the Constantinople/Petersburg/Istanbul EVM fork at
block **105000**. All operators must upgrade **before** this block.

ChainId/networkId remain **121525**. No genesis re-init is required.

---

## 1) Stop the node

**Windows:**
```
Stop-Process -Name ethernova
```

**Linux:**
```
killall ethernova
```

---

## 2) Replace the binary

- Download the v1.2.8 binary for your OS.
- Replace the existing `ethernova`/`ethernova.exe`.

---

## 3) Restart

**Windows:**
```
ethernova.exe --datadir <your-datadir> --networkid 121525
```

**Linux:**
```
ethernova --datadir <your-datadir> --networkid 121525
```

---

## 4) Verify

Confirm:

- `web3_clientVersion` shows `Ethernova/v1.2.8/...`
- Fork schedule log shows `fork scheduled at 105000 (Constantinople)`
- Enforcement log shows `Ethernova fork enforcement block=105,000`

Run the verification script:

**Windows:**
```
.\scripts\verify-fork-windows.ps1 -Endpoint http://127.0.0.1:8545
```

**Linux:**
```
./scripts/verify-fork-linux.sh http://127.0.0.1:8545
```

Enforcement verification (checks chainId/genesis/log output):

**Windows:**
```
.\scripts\verify-enforcement-windows.ps1
```

**Linux:**
```
./scripts/verify-enforcement-linux.sh
```

Expected results:

- Pre-fork (block 104999): SHL fails with invalid opcode.
- Post-fork (block 105000): SHL succeeds.

---

## 5) If you are already past block 105000

If the node refuses to start with a message about missing fork blocks, upgrade
immediately. The client will hard-fail on misconfigured chain configs at or
after the fork to prevent accidental divergence.
