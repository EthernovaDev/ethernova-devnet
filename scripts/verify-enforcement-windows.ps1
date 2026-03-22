[CmdletBinding()]
param(
  [string]$EthernovaExe = "",
  [string]$DataDir = "",
  [int]$HttpPort = 8545,
  [string]$ExpectedGenesisHash = "0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"
)

$ErrorActionPreference = "Stop"

function Resolve-FirstPath {
  param([string[]]$Candidates)
  foreach ($path in $Candidates) {
    if ($path -and (Test-Path $path)) {
      return (Resolve-Path $path).Path
    }
  }
  return $null
}

function Normalize-Hex([string]$v) {
  if (-not $v) { return "" }
  $s = $v.Trim().Trim('"').ToLower()
  if (-not $s.StartsWith("0x")) { $s = "0x$s" }
  return $s
}

function HexToUInt64([string]$hex) {
  if (-not $hex) { return 0 }
  $h = $hex.Trim()
  if ($h.StartsWith("0x")) { $h = $h.Substring(2) }
  if ($h.Length -eq 0) { return 0 }
  return [Convert]::ToUInt64($h, 16)
}

function Test-PortInUse {
  param([int]$Port)
  if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
    try {
      $matches = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction Stop
      return ($matches.Count -gt 0)
    } catch {
    }
  }

  try {
    $lines = netstat -ano -p tcp | Select-String -Pattern "LISTENING"
    foreach ($line in $lines) {
      if ($line -match "[:\\.]$Port\\s") {
        return $true
      }
    }
  } catch {
  }
  return $false
}

function Get-FreePort {
  param([int]$StartPort)
  for ($p = $StartPort; $p -lt ($StartPort + 20); $p++) {
    if (-not (Test-PortInUse -Port $p)) { return $p }
  }
  throw "No free port found in range $StartPort-$($StartPort + 19)"
}

function Call-Rpc([string]$Endpoint, [string]$method, [object[]]$params) {
  $payload = @{
    jsonrpc = "2.0"
    method  = $method
    params  = $params
    id      = 1
  } | ConvertTo-Json -Compress -Depth 8

  $resp = Invoke-RestMethod -Method Post -Uri $Endpoint -Body $payload -ContentType "application/json"
  if ($resp -and $resp.error) {
    throw ("RPC error: {0}" -f ($resp.error | ConvertTo-Json -Compress))
  }
  return $resp.result
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

if (-not $EthernovaExe) {
  $EthernovaExe = Resolve-FirstPath @(
    (Join-Path $RepoRoot "dist\\ethernova-v1.2.8-windows-amd64.exe"),
    (Join-Path $RepoRoot "dist\\ethernova.exe"),
    (Join-Path $RepoRoot "ethernova.exe")
  )
}
if (-not $EthernovaExe) { throw "Ethernova executable not found." }

$EmbeddedGenesisPath = Join-Path $RepoRoot "params\\ethernova\\genesis-121525-alloc.json"
if (Test-Path $EmbeddedGenesisPath) {
  try {
    $GenesisTarget = Join-Path (Split-Path $EthernovaExe -Parent) "genesis-121525-alloc.json"
    Copy-Item -Path $EmbeddedGenesisPath -Destination $GenesisTarget -Force
  } catch {
    Write-Warning ("Failed to refresh genesis-121525-alloc.json: {0}" -f $_.Exception.Message)
  }
}

$GenesisPath = Join-Path $RepoRoot "genesis-mainnet.json"
if (-not (Test-Path $GenesisPath)) { throw "Genesis not found: $GenesisPath" }

if (-not $DataDir) {
  $DataDir = Join-Path $env:TEMP ("ethernova-verify-enforcement-" + [System.Guid]::NewGuid().ToString("N"))
}
if (-not (Test-Path $DataDir)) { New-Item -ItemType Directory -Force -Path $DataDir | Out-Null }

$ChainDataDir = Join-Path $DataDir "geth\\chaindata"
if (-not (Test-Path $ChainDataDir)) {
  & $EthernovaExe --datadir $DataDir init $GenesisPath | Out-Null
}

$Port = Get-FreePort -StartPort $HttpPort
$Endpoint = "http://127.0.0.1:$Port"
$LogPath = Join-Path $DataDir "ethernova.out.log"
$ErrPath = Join-Path $DataDir "ethernova.err.log"

$Args = @(
  "--datadir", $DataDir,
  "--networkid", "121525",
  "--http", "--http.addr", "127.0.0.1", "--http.port", "$Port",
  "--http.api", "eth,net,web3,debug",
  "--nodiscover"
)

Write-Host "== Ethernova Enforcement Verify ==" -ForegroundColor Cyan
Write-Host ("exe={0}" -f $EthernovaExe)
Write-Host ("datadir={0}" -f $DataDir)
Write-Host ("endpoint={0}" -f $Endpoint)

$proc = $null
try {
  $proc = Start-Process -FilePath $EthernovaExe -ArgumentList $Args -RedirectStandardOutput $LogPath -RedirectStandardError $ErrPath -PassThru -WindowStyle Hidden

  $clientVersion = $null
  $deadline = (Get-Date).AddSeconds(30)
  while ((Get-Date) -lt $deadline) {
    try {
      $clientVersion = Call-Rpc $Endpoint "web3_clientVersion" @()
      if ($clientVersion) { break }
    } catch {
    }
    Start-Sleep -Milliseconds 500
  }
  if (-not $clientVersion) { throw "RPC did not respond on $Endpoint" }

  $chainIdHex = Call-Rpc $Endpoint "eth_chainId" @()
  $chainId = HexToUInt64 $chainIdHex
  if ($chainId -ne 121525) { throw ("unexpected chainId: {0}" -f $chainIdHex) }

  $block0 = Call-Rpc $Endpoint "eth_getBlockByNumber" @("0x0", $false)
  if (-not $block0) { throw "eth_getBlockByNumber(0x0) failed" }
  $genesisHash = $block0.hash
  if ((Normalize-Hex $genesisHash) -ne (Normalize-Hex $ExpectedGenesisHash)) {
    throw ("unexpected genesis hash: {0}" -f $genesisHash)
  }

  $expectedLine = "Ethernova fork enforcement block=105,000"
  $found = $false
  $logDeadline = (Get-Date).AddSeconds(10)
  while ((Get-Date) -lt $logDeadline) {
    if (Test-Path $LogPath) {
      $match = Select-String -Path @($LogPath, $ErrPath) -Pattern $expectedLine -SimpleMatch -ErrorAction SilentlyContinue
      if ($match) { $found = $true; break }
    }
    Start-Sleep -Milliseconds 300
  }
  if (-not $found) {
    throw ("log line not found: {0}" -f $expectedLine)
  }

  Write-Host ("clientVersion={0}" -f $clientVersion)
  Write-Host ("chainId={0}" -f $chainIdHex)
  Write-Host ("genesis={0}" -f $genesisHash)
  Write-Host ("log: {0}" -f $expectedLine)
  Write-Host "OK: enforcement verification passed." -ForegroundColor Green
  exit 0
} finally {
  if ($proc -and -not $proc.HasExited) {
    Stop-Process -Id $proc.Id -Force | Out-Null
    Start-Sleep -Milliseconds 300
  }
}
