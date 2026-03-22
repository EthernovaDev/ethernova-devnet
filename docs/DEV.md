# Ethernova Dev Workflow (Windows)

## Build
Optional helper script:
```
powershell -ExecutionPolicy Bypass -File scripts/build-windows.ps1
```
Manual build (equivalent):
```
go build -o bin\\ethernova.exe .\\cmd\\geth
```
Output: `bin\ethernova.exe`

## Dev genesis and fast mining (chainId 77778)
Use `genesis-dev.json` (difficulty=0x1, forks at block 0, block reward halving baked in, permissive txpool).

## Init + start (dev mode)
Manual (example):
```
bin\\ethernova.exe init --datadir .\\data-dev .\\genesis-dev.json
bin\\ethernova.exe --datadir .\\data-dev --networkid 77778 --http --http.addr 127.0.0.1 --http.port 8545 --http.api eth,net,web3,personal,miner,txpool,admin,debug --ws --ws.addr 127.0.0.1 --ws.port 8546 --ws.api eth,net,web3,personal,miner,txpool,admin,debug --mine
```
Optional helper script (convenience only):
```
powershell -ExecutionPolicy Bypass -File scripts/init-ethernova.ps1 -Mode dev
```
- Datadir: `data-dev\`
- Logs: `logs\node.log` and `logs\node.err`
- RPC: `http://127.0.0.1:8545`, `ws://127.0.0.1:8546`
- APIs: `eth,net,web3,personal,miner,txpool,admin,debug`
- txpool/miner gasprice set to 0 for easy inclusion.

## Smoke test (baseFeeVault)
Optional helper script (convenience only):
```
powershell -ExecutionPolicy Bypass -File scripts/smoke-test-fees.ps1 -Rpc http://127.0.0.1:8545 -Pass "nova-smoke"
```
Pass criteria: type-2 tx mined, gasUsed>0, vault delta == baseFeePerGas * gasUsed.
> Tip: If you have PowerShell 7 installed, `pwsh` works too; examples above assume Windows PowerShell with `powershell -ExecutionPolicy Bypass -File ...`.

## Local mining (with Miningcore RPC)
- Start the dev node (mining enabled by default):  
  `bin\\ethernova.exe --datadir .\\data-dev --networkid 77778 --mine ...`
- RPC stays on `http://127.0.0.1:8545`; keep it localhost-only.
- Miningcore JSON-RPC target: `http://127.0.0.1:8545` (chainId 77778 in dev). Configure whitelist/auth on Miningcore side.
- Verify mining:  
  `Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8545 -Body '{"jsonrpc":"2.0","id":1,"method":"eth_mining","params":[]}' -ContentType "application/json"`
- Optional checks: `eth_hashrate`, `net_peerCount`.
## Useful attach commands
```
bin\ethernova.exe attach --exec "eth.blockNumber" http://127.0.0.1:8545
bin\ethernova.exe attach --exec "txpool.status"   http://127.0.0.1:8545
bin\ethernova.exe attach --exec "admin.nodeInfo.protocols.eth.config" http://127.0.0.1:8545
```

## Keystore safety
- Helper scripts may back up `data-*/keystore` to `backups/`.
- Never delete keystore without a backup. Keep passwords secure.

## Test suite
```
set PATH=C:\msys64\mingw64\bin;%PATH%
set CC=C:\msys64\mingw64\bin\gcc.exe
set CGO_ENABLED=1
go test (go list ./... excluding cmd/geth, consensus/ethash, tests)
```
Focus tests: `core/basefee_vault_test.go`, `params/types/ctypes/ethash_reward_test.go`.

## CI policy
- CI runs the fast subset above (excludes `cmd/geth`, `consensus/ethash`, `tests`).
- To run the full integration suite locally: `go test ./...` (expect longer time and ensure no port conflicts with authrpc/HTTP).
