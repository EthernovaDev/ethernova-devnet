# UPGRADE v1.3.0 (Ethernova Mainnet 121525)

## Summary
This upgrade activates the **Mega Fork (EVM-compat)** at block **118200**, enabling missing historical EVM forks (Homestead/Tangerine/Spurious/Byzantium) and the Petersburg fix (EIP-1706). It also aligns London sibling fields with existing `eip1559FBlock=0`. **No genesis reset and no resync** are required.

## Pre-checks
- Confirm chainId/networkId = **121525**
- Confirm genesis hash:
  `0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9`

## Upgrade (RPC/Explorer VPS)
1. **Backup current binary + service files**
   ```bash
   cp /usr/local/bin/ethernova /usr/local/bin/ethernova.backup.$(date +%Y%m%d)
   cp /etc/systemd/system/ethernova.service /etc/systemd/system/ethernova.service.bak
   cp /etc/systemd/system/ethernova-archive.service /etc/systemd/system/ethernova-archive.service.bak
   ```
2. **Stop services**
   ```bash
   systemctl stop ethernova.service
   systemctl stop ethernova-archive.service
   ```
3. **Install new binary**
   ```bash
   cp /path/to/ethernova-v1.3.0-linux-amd64 /usr/local/bin/ethernova
   chmod +x /usr/local/bin/ethernova
   ```
4. **If startup fails due to lock**, remove lock files (no resync):
   ```bash
   rm -f /var/lib/ethernova/geth/ethernova.lock.json
   rm -f /var/lib/ethernova-archive/geth/ethernova.lock.json
   ```
5. **Start services**
   ```bash
   systemctl start ethernova.service
   systemctl start ethernova-archive.service
   ```
6. **Verify**
   ```bash
   systemctl status ethernova.service --no-pager
   systemctl status ethernova-archive.service --no-pager
   curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"web3_clientVersion","params":[]}' http://127.0.0.1:8545
   curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' http://127.0.0.1:8545
   ```

## Upgrade (NovaPool VPS)
1. **Backup current binary + service files**
   ```bash
   cp /usr/local/bin/ethernova /usr/local/bin/ethernova.backup.$(date +%Y%m%d)
   cp /etc/systemd/system/ethernova.service /etc/systemd/system/ethernova.service.bak
   ```
2. **Stop service**
   ```bash
   systemctl stop ethernova.service
   ```
3. **Install new binary**
   ```bash
   cp /path/to/ethernova-v1.3.0-linux-amd64 /usr/local/bin/ethernova
   chmod +x /usr/local/bin/ethernova
   ```
4. **If startup fails due to lock**, remove lock file (no resync):
   ```bash
   rm -f /var/lib/ethernova/geth/ethernova.lock.json
   ```
5. **Start services**
   ```bash
   systemctl start ethernova.service
   systemctl status ethernova.service --no-pager
   systemctl status miningcore.service --no-pager
   ```
6. **Verify**
   ```bash
   curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"web3_clientVersion","params":[]}' http://127.0.0.1:8545
   curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' http://127.0.0.1:8545
   ```

## Rollback
1. Stop services.
2. Restore previous binary from backup:
   ```bash
   cp /usr/local/bin/ethernova.backup.YYYYMMDD /usr/local/bin/ethernova
   chmod +x /usr/local/bin/ethernova
   ```
3. Start services and re-check RPC.
