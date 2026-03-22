Param(
    [ValidateSet("dev", "mainnet")]
    [string]$Mode = "dev",
    [string]$Genesis = "",
    [string]$Bootnodes = "",
    [int]$Port = 30304,
    [int]$HttpPort = 8547,
    [int]$WsPort = 8548,
    [string]$Root = ""
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path $PSScriptRoot -Parent
if (-not $Root) { $Root = $RepoRoot }

$Binary = Join-Path $RepoRoot "bin\ethernova.exe"
if (-not (Test-Path $Binary)) { throw "Binary not found at $Binary" }

$GenesisPath = $Genesis
if (-not $GenesisPath) {
    $GenesisPath = if ($Mode -eq "dev") { Join-Path $RepoRoot "genesis-dev.json" } else { Join-Path $RepoRoot "genesis-mainnet.json" }
}
$GenesisPath = (Resolve-Path $GenesisPath).Path

$genesisJson = Get-Content $GenesisPath -Raw | ConvertFrom-Json
$chainId = [uint64]$genesisJson.config.chainId
$networkId = if ($genesisJson.config.networkId) { [uint64]$genesisJson.config.networkId } else { $chainId }

if ($Mode -eq "mainnet" -and $chainId -ne 121525) { throw "Mainnet mode requires chainId 121525" }
if ($Mode -eq "dev" -and $chainId -ne 77778) { throw "Dev/test mode requires chainId 77778 (avoid using mainnet genesis)" }

if ($Mode -eq "mainnet") {
    $networkId = 121525
}

$DataDir = Join-Path $Root "data-node2-$Mode"
$LogsDir = Join-Path $Root "logs"
$NodeLog = Join-Path $LogsDir "node2.log"
$NodeErr = Join-Path $LogsDir "node2.err.log"
$IpcPath = "\\.\pipe\ethernova-node2-$Mode.ipc"

if (-not (Test-Path $LogsDir)) { New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null }
if (-not (Test-Path $DataDir)) { New-Item -ItemType Directory -Force -Path $DataDir | Out-Null }

if (-not (Test-Path (Join-Path $DataDir "geth\chaindata"))) {
    Write-Host "Datadir empty; running genesis init for node2..."
    & $Binary --datadir $DataDir init $GenesisPath
}

$apis = if ($Mode -eq "dev") { "eth,net,web3,personal,miner,txpool,admin,debug" } else { "eth,net,web3" }
$args = @(
    "--datadir", $DataDir,
    "--networkid", "$networkId",
    "--authrpc.addr", "127.0.0.1", "--authrpc.port", "8552",
    "--ipcpath", $IpcPath,
    "--port", "$Port",
    "--http", "--http.addr", "127.0.0.1", "--http.port", "$HttpPort", "--http.api", $apis, "--http.vhosts", "localhost",
    "--ws", "--ws.addr", "127.0.0.1", "--ws.port", "$WsPort", "--ws.api", $apis,
    "--mine",
    "--miner.threads", "1",
    "--miner.etherbase", "0x3a38560b66205bb6a31decbcb245450b2f15d4fd",
    "--verbosity", "3"
)

if ($Mode -eq "dev") {
    $args += @("--allow-insecure-unlock", "--miner.gasprice", "0", "--txpool.pricelimit", "0", "--txpool.pricebump", "0")
} else {
    $args += @("--miner.gasprice", "1000000000")
}

if ($Bootnodes) { $args += @("--bootnodes", $Bootnodes) }

Write-Host "Starting second node (mode=$Mode, port=$Port, http=$HttpPort)..."
Start-Process -FilePath $Binary `
    -ArgumentList $args `
    -RedirectStandardOutput $NodeLog `
    -RedirectStandardError $NodeErr `
    -NoNewWindow | Out-Null

Write-Host "Node2 started. Logs: $NodeLog / $NodeErr"
