Param(
    [ValidateSet("dev", "mainnet")]
    [string]$Mode = "dev",
    [string]$Genesis = "",
    [string]$Bootnodes = "",
    [string]$Root = ""
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path $PSScriptRoot -Parent
if (-not $Root) { $Root = $RepoRoot }

$Binary = Join-Path $RepoRoot "bin\ethernova.exe"
if (-not (Test-Path $Binary)) {
    throw "Binary not found at $Binary. Run scripts\build-windows.ps1 first."
}

$GenesisPath = $Genesis
if (-not $GenesisPath) {
    $GenesisPath = if ($Mode -eq "dev") { Join-Path $RepoRoot "genesis-dev.json" } else { Join-Path $RepoRoot "genesis-mainnet.json" }
}
$GenesisPath = (Resolve-Path $GenesisPath).Path

$genesisJson = Get-Content $GenesisPath -Raw | ConvertFrom-Json
$chainId = [uint64]$genesisJson.config.chainId
$networkId = if ($genesisJson.config.networkId) { [uint64]$genesisJson.config.networkId } else { $chainId }

if ($Mode -eq "mainnet" -and $chainId -ne 121525) {
    throw "Mode=mainnet requires chainId 121525, got $chainId. Use the correct genesis file."
}
if ($Mode -eq "dev" -and $chainId -ne 77778) {
    throw "Mode=dev/test requires chainId 77778 (or non-mainnet), got $chainId. Avoid mixing genesis files."
}

if ($Mode -eq "mainnet") {
    $networkId = 121525
}

$dataDirName = if ($Mode -eq "mainnet") { "data-mainnet" } else { "data-dev" }
$DataDir = Join-Path $Root $dataDirName
$KeystorePath = Join-Path $DataDir "keystore"
$BackupRoot = Join-Path $Root "backups"
$Timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$LogsDir = Join-Path $Root "logs"
$NodeLog = Join-Path $LogsDir "node.log"
$NodeErr = Join-Path $LogsDir "node.err.log"
$HttpPortPreferred = 8545

$Miner = "0x3a38560b66205bb6a31decbcb245450b2f15d4fd"
$StaticNodesSrc = if ($Mode -eq "mainnet") { Join-Path $RepoRoot "networks\mainnet\static-nodes.json" } else { Join-Path $RepoRoot "networks\dev\static-nodes.json" }
$StaticNodesDst = Join-Path $DataDir "geth\static-nodes.json"
$BootnodesFile = if ($Mode -eq "mainnet") { Join-Path $RepoRoot "networks\mainnet\bootnodes.txt" } else { Join-Path $RepoRoot "networks\dev\bootnodes.txt" }

Write-Host "Mode: $Mode"
Write-Host "Genesis: $GenesisPath"
Write-Host "chainId: $chainId  networkId: $networkId"
Write-Host "Datadir: $DataDir"

if (-not (Test-Path $LogsDir)) { New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null }

$running = Get-Process -Name "ethernova" -ErrorAction SilentlyContinue
if ($running) {
    Write-Host "Stopping existing ethernova process..."
    $running | Stop-Process -Force
}

$BackupTarget = $null
if (Test-Path $KeystorePath) {
    $BackupTarget = Join-Path $BackupRoot "keystore-$($Mode)-$Timestamp"
    Write-Host "Backing up keystore to $BackupTarget"
    New-Item -ItemType Directory -Force -Path $BackupTarget | Out-Null
    Copy-Item -Path $KeystorePath -Destination $BackupTarget -Recurse -Force
}

if (Test-Path $DataDir) {
    Write-Host "Wiping datadir $DataDir"
    $retries = 3
    while ($retries -gt 0) {
        try {
            Remove-Item -Path $DataDir -Recurse -Force -ErrorAction Stop
            break
        } catch {
            $retries--
            if ($retries -le 0) { throw }
            Start-Sleep -Seconds 1
        }
    }
}
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

if ($BackupTarget -and (Test-Path $BackupTarget)) {
    Write-Host "Restoring keystore backup"
    Copy-Item -Path (Join-Path $BackupTarget "keystore") -Destination $DataDir -Recurse -Force
}

Write-Host "Running genesis init..."
& $Binary --datadir $DataDir init $GenesisPath
$initExit = $LASTEXITCODE
if ($initExit -ne 0) {
    throw "Genesis init failed with exit code $initExit"
}

if ($StaticNodesSrc -and (Test-Path $StaticNodesSrc)) {
    $dstDir = Split-Path $StaticNodesDst -Parent
    if (-not (Test-Path $dstDir)) { New-Item -ItemType Directory -Force -Path $dstDir | Out-Null }
    Copy-Item $StaticNodesSrc $StaticNodesDst -Force
    Write-Host "Placed static-nodes.json from $StaticNodesSrc"
}

$bootnodeList = @()
if ($Bootnodes) { $bootnodeList += $Bootnodes }
if (Test-Path $BootnodesFile) {
    $fileContent = Get-Content $BootnodesFile | Where-Object {
        -not [string]::IsNullOrWhiteSpace($_) -and -not $_.Trim().StartsWith("#")
    }
    if ($fileContent) { $bootnodeList += $fileContent }
}

function Test-PortFree([int]$port) {
    try {
        $listener = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, $port)
        $listener.Start()
        $listener.Stop()
        return $true
    } catch {
        return $false
    }
}

$HttpPort = $HttpPortPreferred
while (-not (Test-PortFree $HttpPort)) {
    $HttpPort++
    if ($HttpPort -gt ($HttpPortPreferred + 10)) { throw "No free HTTP port found near $HttpPortPreferred" }
}
$WsPort = $HttpPort + 1

Write-Host "Starting ethernova node..."
$apis = if ($Mode -eq "dev") { "eth,net,web3,personal,miner,txpool,admin,debug" } else { "eth,net,web3" }
$ipcPath = "\\.\pipe\ethernova-$Mode.ipc"
$args = @(
    "--datadir", $DataDir,
    "--networkid", "$networkId",
    "--authrpc.addr", "127.0.0.1", "--authrpc.port", "8551",
    "--ipcpath", $ipcPath,
    "--http", "--http.addr", "127.0.0.1", "--http.port", "$HttpPort",
    "--http.vhosts", "localhost",
    "--http.api", $apis,
    "--ws", "--ws.addr", "127.0.0.1", "--ws.port", "$WsPort",
    "--ws.api", $apis,
    "--mine",
    "--miner.threads", "1",
    "--miner.etherbase", $Miner,
    "--verbosity", "4",
    "--vmodule", "miner=5,txpool=5"
)

if ($Mode -eq "dev") {
    $args += @("--allow-insecure-unlock", "--miner.gasprice", "0", "--txpool.pricelimit", "0", "--txpool.pricebump", "0")
} else {
    $args += @("--miner.gasprice", "1000000000")
}

if ($bootnodeList.Count -gt 0) {
    $args += @("--bootnodes", ($bootnodeList -join ","))
    Write-Host "Bootnodes: $($bootnodeList -join ', ')"
}

$startInfo = @{
    FilePath               = $Binary
    ArgumentList           = $args
    RedirectStandardOutput = $NodeLog
    RedirectStandardError  = $NodeErr
    NoNewWindow            = $true
}

Start-Process @startInfo | Out-Null
Write-Host "Node started. Logs: $NodeLog / $NodeErr"
Write-Host "HTTP RPC: http://127.0.0.1:$HttpPort"
Write-Host "WS RPC:   ws://127.0.0.1:$WsPort"

# Wait for RPC to come up
$maxWait = 30
$ready = $false
$chainIdResp = $null
for ($i=0; $i -lt $maxWait; $i++) {
    try {
        $payload = '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
        $resp = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:$HttpPort" -Body $payload -ContentType "application/json" -TimeoutSec 5
        if ($resp -and $resp.result) {
            $chainIdResp = $resp.result
            $ready = $true
            break
        }
    } catch { Start-Sleep -Seconds 1 }
    Start-Sleep -Seconds 1
}

if (-not $ready) {
    Write-Warning "RPC not reachable on http://127.0.0.1:$HttpPort after $maxWait seconds."
} else {
    Write-Host "RPC reachable. eth_chainId = $chainIdResp"
}

# netstat check
$netstatOut = netstat -ano | Select-String ":$HttpPort"
if ($netstatOut) {
    Write-Host "netstat: port $HttpPort is LISTENING (or in use):"
    $netstatOut | ForEach-Object { Write-Host "  $_" }
} else {
    Write-Warning "netstat could not find a listener on :$HttpPort"
}
