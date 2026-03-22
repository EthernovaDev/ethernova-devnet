$ErrorActionPreference = "Stop"

Param(
    [int]$Port = 30303,
    [int]$HttpPort = 8550,
    [int]$WsPort = 8551,
    [string]$Root = ""
)

$RepoRoot = Split-Path $PSScriptRoot -Parent
if (-not $Root) { $Root = $RepoRoot }

$Binary = Join-Path $RepoRoot "bin\ethernova.exe"
$DataDir = Join-Path $Root "bootnode"
$LogsDir = Join-Path $Root "logs"
$NodeLog = Join-Path $LogsDir "bootnode.log"
$NodeErr = Join-Path $LogsDir "bootnode.err.log"

if (-not (Test-Path $LogsDir)) { New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null }
if (-not (Test-Path $DataDir)) { New-Item -ItemType Directory -Force -Path $DataDir | Out-Null }

Write-Host "Starting bootnode (p2p port=$Port, HTTP admin=$HttpPort)..."
$proc = Start-Process -FilePath $Binary `
    -ArgumentList @(
        "--datadir", $DataDir,
        "--nodiscover",
        "--port", "$Port",
        "--http", "--http.addr", "127.0.0.1", "--http.port", "$HttpPort", "--http.api", "net,admin",
        "--ws", "--ws.addr", "127.0.0.1", "--ws.port", "$WsPort", "--ws.api", "net",
        "--verbosity", "3"
    ) `
    -RedirectStandardOutput $NodeLog `
    -RedirectStandardError $NodeErr `
    -NoNewWindow `
    -PassThru

Start-Sleep -Seconds 2
$enodeOut = & $Binary attach --exec "admin.nodeInfo.enode" "http://127.0.0.1:$HttpPort" 2>$null
Write-Host "Bootnode enode: $enodeOut"
Write-Host "Logs: $NodeLog / $NodeErr"
