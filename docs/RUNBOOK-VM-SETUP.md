# RUNBOOK-VM-SETUP

## Repo Location

- Repo path: `C:\dev\Ethernova\core-geth-src`
- Expected branch: `main`

Verify:
```powershell
cd C:\dev\Ethernova\core-geth-src
git status -sb
git branch --show-current
git remote -v
git tag --list --sort=-creatordate | Select-Object -First 10
git log -5 --oneline
```

## Required Tools

- Git for Windows (adds `git` to PATH)
- Go 1.21.x (matches `go.mod`)
- Python 3 (needed by `scripts\package-release.ps1` for linux tar)
- Optional: mingw-w64 (only if CGO is enabled or specific tests require it)

## Codex Skills Location

- Primary: `C:\Users\DevVM\.codex\skills\`
- If using another Windows user, replicate skills under that profile's `.codex\skills\`.

## Build (Windows + Linux Binaries)

Windows binary:
```powershell
cd C:\dev\Ethernova\core-geth-src
.\scripts\build-windows.ps1
```

Linux binary (cross-compile on Windows):
```powershell
$ver = (Get-Content .\VERSION -Raw).Trim()
$date = Get-Date -Format "yyyyMMdd"
$ld = "-X github.com/ethereum/go-ethereum/internal/version.gitCommit=$ver -X github.com/ethereum/go-ethereum/internal/version.gitDate=$date"
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -trimpath -buildvcs=false -ldflags $ld -o "dist\\ethernova-$ver-linux-amd64" .\\cmd\\geth
```

SHA256 sums file:
```powershell
$ver = (Get-Content .\VERSION -Raw).Trim()
$files = @(
  "dist\\ethernova-$ver-windows-amd64.exe",
  "dist\\ethernova-$ver-linux-amd64"
)
$hashes = $files | ForEach-Object {
  (Get-FileHash -Algorithm SHA256 $_).Hash.ToLower() + "  " + (Split-Path $_ -Leaf)
}
$hashes | Set-Content "dist\\SHA256SUMS-$ver.txt" -Encoding ASCII
```

## Tests

```powershell
go test ./...
go test ./core/...
go test ./core/forkid
```

## Release Steps (Local)

1. Update `VERSION` and release notes (`docs/RELEASE-NOTES-vX.Y.Z.md`, `docs/UPGRADE-vX.Y.Z.md`).
2. Build binaries and `SHA256SUMS-vX.Y.Z.txt` as above.
3. Verify artifacts in `dist\`.
4. Tag and push:
```powershell
git tag -a vX.Y.Z -m "Ethernova vX.Y.Z"
git push origin vX.Y.Z
```

## VPS Upgrade (RPC/Explorer + NovaPool)

Baseline checks:
```powershell
ssh -i C:\Users\Dev\.ssh\rpcandexplorer root@207.180.230.125 "uname -a"
ssh -i C:\Users\Dev\.ssh\rpcandexplorer root@207.180.230.125 "systemctl status ethernova* --no-pager"
ssh -i C:\Users\Dev\.ssh\rpcandexplorer root@207.180.230.125 "curl -s -X POST -H 'Content-Type: application/json' --data '{\"jsonrpc\":\"2.0\",\"method\":\"web3_clientVersion\",\"params\":[],\"id\":1}' http://127.0.0.1:8545"
ssh -i C:\Users\Dev\.ssh\rpcandexplorer root@207.180.230.125 "curl -s -X POST -H 'Content-Type: application/json' --data '{\"jsonrpc\":\"2.0\",\"method\":\"eth_chainId\",\"params\":[],\"id\":1}' http://127.0.0.1:8545"
```

Repeat for NovaPool:
```powershell
ssh -i C:\Users\Dev\.ssh\novapool root@207.180.211.179 "uname -a"
ssh -i C:\Users\Dev\.ssh\novapool root@207.180.211.179 "systemctl status ethernova* --no-pager"
ssh -i C:\Users\Dev\.ssh\novapool root@207.180.211.179 "curl -s -X POST -H 'Content-Type: application/json' --data '{\"jsonrpc\":\"2.0\",\"method\":\"web3_clientVersion\",\"params\":[],\"id\":1}' http://127.0.0.1:8545"
ssh -i C:\Users\Dev\.ssh\novapool root@207.180.211.179 "curl -s -X POST -H 'Content-Type: application/json' --data '{\"jsonrpc\":\"2.0\",\"method\":\"eth_chainId\",\"params\":[],\"id\":1}' http://127.0.0.1:8545"
```

Upgrade (summary):
1. Upload new `ethernova` binary to each VPS.
2. Stop service: `systemctl stop ethernova` (or service name in use).
3. Replace binary and set permissions.
4. Start service: `systemctl start ethernova`
5. Verify: `systemctl status ethernova* --no-pager` and RPC checks above.

## Rollback

1. Stop service.
2. Restore previous binary from backup.
3. Start service and re-check RPC + logs.
