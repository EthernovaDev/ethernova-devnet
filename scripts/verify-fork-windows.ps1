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

function Invoke-RpcRaw {
  param(
    [string]$Endpoint,
    [string]$Method,
    [object[]]$Params
  )
  $payload = @{
    jsonrpc = "2.0"
    method  = $Method
    params  = $Params
    id      = 1
  } | ConvertTo-Json -Compress -Depth 8

  $resp = $payload | & curl.exe -sS --fail --max-time 2 -H "Content-Type: application/json" --data-binary '@-' $Endpoint 2>&1
  if ($LASTEXITCODE -ne 0) {
    throw ("curl.exe failed: {0}" -f $resp)
  }
  if (-not $resp) {
    throw "empty RPC response"
  }
  return ($resp | ConvertFrom-Json)
}

function Invoke-Rpc {
  param(
    [string]$Endpoint,
    [string]$Method,
    [object[]]$Params
  )
  $resp = Invoke-RpcRaw -Endpoint $Endpoint -Method $Method -Params $Params
  if ($resp.error) {
    throw ("RPC error: {0}" -f ($resp.error | ConvertTo-Json -Compress))
  }
  return $resp.result
}

if (-not (Get-Command curl.exe -ErrorAction SilentlyContinue)) {
  throw "curl.exe not found on PATH"
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
  $DataDir = Join-Path $env:TEMP ("ethernova-verify-fork-" + [System.Guid]::NewGuid().ToString("N"))
}
if (-not (Test-Path $DataDir)) { New-Item -ItemType Directory -Force -Path $DataDir | Out-Null }

& $EthernovaExe --datadir $DataDir init $GenesisPath | Out-Null

$Port = Get-FreePort -StartPort $HttpPort
$Endpoint = "http://127.0.0.1:$Port"
$LogOut = Join-Path $DataDir "ethernova.out.log"
$LogErr = Join-Path $DataDir "ethernova.err.log"

$Args = @(
  "--datadir", $DataDir,
  "--networkid", "121525",
  "--http", "--http.addr", "127.0.0.1", "--http.port", "$Port",
  "--http.api", "eth,net,web3,debug",
  "--nodiscover",
  "--ipcdisable",
  "--log.file", (Join-Path $DataDir "ethernova.log")
)

Write-Host "== Ethernova Fork Verify ==" -ForegroundColor Cyan
Write-Host ("exe={0}" -f $EthernovaExe)
Write-Host ("datadir={0}" -f $DataDir)
Write-Host ("endpoint={0}" -f $Endpoint)

$proc = $null
try {
  $proc = Start-Process -FilePath $EthernovaExe -ArgumentList $Args -RedirectStandardOutput $LogOut -RedirectStandardError $LogErr -PassThru -WindowStyle Hidden

  $clientVersion = $null
  $lastErr = ""
  $deadline = (Get-Date).AddSeconds(30)
  while ((Get-Date) -lt $deadline) {
    try {
      $clientVersion = Invoke-Rpc -Endpoint $Endpoint -Method "web3_clientVersion" -Params @()
      if ($clientVersion) { break }
    } catch {
      $lastErr = $_.Exception.Message
    }
    Start-Sleep -Milliseconds 500
  }
  if (-not $clientVersion) { throw ("RPC did not respond on {0}. Last error: {1}" -f $Endpoint, $lastErr) }

  $chainIdHex = Invoke-Rpc -Endpoint $Endpoint -Method "eth_chainId" -Params @()
  if ((Normalize-Hex $chainIdHex) -ne "0x1dab5") {
    throw ("unexpected chainId: {0}" -f $chainIdHex)
  }

  $block0 = Invoke-Rpc -Endpoint $Endpoint -Method "eth_getBlockByNumber" -Params @("0x0", $false)
  if (-not $block0) { throw "eth_getBlockByNumber(0x0) failed" }
  $genesisHash = $block0.hash
  if ((Normalize-Hex $genesisHash) -ne (Normalize-Hex $ExpectedGenesisHash)) {
    throw ("unexpected genesis hash: {0}" -f $genesisHash)
  }

  function Trace-Opcode {
    param(
      [string]$Label,
      [string]$CodeHex,
      [string]$BlockHex,
      [bool]$ExpectInvalid
    )
    $call = @{
      from = "0x0000000000000000000000000000000000000000"
      gas  = "0x2dc6c0"
      data = $CodeHex
    }
    $config = @{}
    if ($BlockHex) {
      $config.blockOverrides = @{ number = $BlockHex }
    }
    $resp = Invoke-RpcRaw -Endpoint $Endpoint -Method "debug_traceCall" -Params @($call, "latest", $config)
    if ($resp.error) {
      throw ("{0}: RPC error {1}" -f $Label, ($resp.error | ConvertTo-Json -Compress))
    }
    $callErr = $null
    $failed = $false
    if ($resp.result -and ($resp.result.PSObject.Properties.Name -contains "failed")) {
      $failed = [bool]$resp.result.failed
    }
    if ($resp.result -and ($resp.result.PSObject.Properties.Name -contains "error")) {
      $callErr = $resp.result.error
    }
    if ($ExpectInvalid) {
      if (-not $failed -and -not $callErr) {
        throw ("{0}: expected invalid opcode, got {1}" -f $Label, ($resp.result | ConvertTo-Json -Compress -Depth 6))
      }
    } else {
      if ($failed -or $callErr) {
        throw ("{0}: expected success, got error {1}" -f $Label, $callErr)
      }
    }
    Write-Host ("{0}: OK" -f $Label)
  }

  $shlCode = "0x600160021b00" # PUSH1 1, PUSH1 2, SHL, STOP
  $chainIdCode = "0x4600"     # CHAINID, STOP
  $selfBalanceCode = "0x4700" # SELFBALANCE, STOP

  try {
    Trace-Opcode "latest (head=0) SHL" $shlCode $null $true
  } catch {
    Write-Warning ("latest SHL did not fail as expected; using explicit block overrides for pre/post verification. Details: {0}" -f $_.Exception.Message)
  }
  Trace-Opcode "pre-fork 104999 SHL" $shlCode "0x19a27" $true
  Trace-Opcode "pre-fork 104999 CHAINID" $chainIdCode "0x19a27" $true
  Trace-Opcode "pre-fork 104999 SELFBALANCE" $selfBalanceCode "0x19a27" $true

  Trace-Opcode "post-fork 105000 SHL" $shlCode "0x19a28" $false
  Trace-Opcode "post-fork 105000 CHAINID" $chainIdCode "0x19a28" $false
  Trace-Opcode "post-fork 105000 SELFBALANCE" $selfBalanceCode "0x19a28" $false

  Write-Host ("clientVersion={0}" -f $clientVersion)
  Write-Host ("chainId={0}" -f $chainIdHex)
  Write-Host ("genesis={0}" -f $genesisHash)
  Write-Host "OK: fork verification passed." -ForegroundColor Green
  exit 0
} finally {
  if ($proc -and -not $proc.HasExited) {
    Stop-Process -Id $proc.Id -Force | Out-Null
    Start-Sleep -Milliseconds 300
  }
}
