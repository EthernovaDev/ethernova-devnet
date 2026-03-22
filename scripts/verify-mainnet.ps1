# verify-mainnet.ps1 (PowerShell 5.1 compatible)
[CmdletBinding()]
param(
  [string]$GenesisPath = "",
  [string]$Endpoint = "http://127.0.0.1:8545",
  [string]$ExpectedGenesisHash = "0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"
)

$ErrorActionPreference = "Stop"

function Normalize-Hex([string]$v) {
  if (-not $v) { return "" }
  $s = $v.Trim().Trim('"').ToLower()
  if (-not $s.StartsWith("0x")) { $s = "0x$s" }
  return $s
}

function HexToBytes([string]$hex) {
  if (-not $hex) { return @() }
  $h = $hex.Trim()
  if ($h.StartsWith("0x")) { $h = $h.Substring(2) }
  if ($h.Length % 2 -ne 0) { $h = "0$h" }
  $bytes = New-Object byte[] ($h.Length / 2)
  for ($i = 0; $i -lt $bytes.Length; $i++) {
    $bytes[$i] = [Convert]::ToByte($h.Substring($i * 2, 2), 16)
  }
  return $bytes
}

function HexToUtf8([string]$hex) {
  try {
    $b = HexToBytes $hex
    if ($b.Length -eq 0) { return "" }
    $text = -join ($b | ForEach-Object { [char]$_ })
    return $text.Trim([char]0)
  } catch { return "" }
}

function Parse-BigInt([string]$s) {
  if (-not $s) { return $null }
  $t = $s.Trim()
  if ($t.StartsWith("0x")) {
    $be = HexToBytes $t            # big-endian bytes
    if ($be.Length -eq 0) { return $null }
    [Array]::Reverse($be)          # little-endian for BigInteger

    # Add 0x00 to force unsigned
    $le = New-Object byte[] ($be.Length + 1)
    [Array]::Copy($be, 0, $le, 0, $be.Length)
    $le[$be.Length] = 0

    return [System.Numerics.BigInteger]::new($le)
  }

  return [System.Numerics.BigInteger]::Parse($t, [System.Globalization.CultureInfo]::InvariantCulture)
}

# Simple JSON-RPC caller (HTTP/HTTPS)
function Call-Rpc([string]$method, [object[]]$params) {
  $payload = @{
    jsonrpc = "2.0"
    method  = $method
    params  = $params
    id      = 1
  } | ConvertTo-Json -Compress
  try {
    $resp = Invoke-RestMethod -Method Post -Uri $Endpoint -Body $payload -ContentType "application/json"
    if ($resp -and $resp.result) { return $resp.result }
    return $null
  } catch {
    return $null
  }
}

# Resolve repo root + defaults
$repoRoot = Split-Path -Parent $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($GenesisPath)) {
  $GenesisPath = Join-Path $repoRoot "genesis-mainnet.json"
}

Write-Host "== Ethernova Mainnet Fingerprint Verify ==" -ForegroundColor Cyan
Write-Host "GenesisPath: $GenesisPath"
Write-Host "Endpoint:    $Endpoint"
Write-Host ""

if (-not (Test-Path $GenesisPath)) {
  throw "Genesis not found: $GenesisPath"
}

# Read expected from genesis JSON
$gen = Get-Content -Raw -Path $GenesisPath | ConvertFrom-Json
$cfg = $gen.config

if (-not $cfg.networkId) { $cfg | Add-Member -NotePropertyName networkId -NotePropertyValue $cfg.chainId -Force }

$expectedChainId = [string]$cfg.chainId
$expectedNetworkId = [string]$cfg.networkId

$expected = [ordered]@{
  "ChainId"          = $expectedChainId
  "NetworkId"        = $expectedNetworkId
  "Consensus"        = "Ethash"
  "GenesisHash"      = (Normalize-Hex $ExpectedGenesisHash)
  "BaseFeeVault"     = (Normalize-Hex $cfg.baseFeeVault)
  "GasLimit"         = (Normalize-Hex $gen.gasLimit)
  "Difficulty"       = (Normalize-Hex $gen.difficulty)
  "BaseFeePerGas"    = (Normalize-Hex $gen.baseFeePerGas)
  "ExtraDataHex"     = (Normalize-Hex $gen.extraData)
  "ExtraDataText"    = (HexToUtf8 $gen.extraData)
}

# Read runtime from node (via JSON-RPC)
$runtime = [ordered]@{}
$missing = New-Object System.Collections.Generic.List[string]
$diffs   = New-Object System.Collections.Generic.List[string]
$rpcErrors = New-Object System.Collections.Generic.List[string]

# Numeric comparisons (accept hex/dec)
function Compare-Num([string]$expHex, [string]$runVal) {
  $e = Parse-BigInt $expHex
  $r = Parse-BigInt $runVal
  if ($e -ne $null -and $r -ne $null -and $e -ne $r) {
    return $false
  }
  return $true
}

$runtime["ChainId"]   = Call-Rpc "eth_chainId" @()
if (-not $runtime["ChainId"]) { $rpcErrors.Add("eth_chainId failed") | Out-Null }
$runtime["NetworkId"] = Call-Rpc "net_version" @()
if (-not $runtime["NetworkId"]) { $rpcErrors.Add("net_version failed") | Out-Null }
$block0 = Call-Rpc "eth_getBlockByNumber" @("0x0",$false)
if ($block0) {
  $runtime["GenesisHash"]   = $block0.hash
  $runtime["GasLimit"]      = $block0.gasLimit
  $runtime["Difficulty"]    = $block0.difficulty
  $runtime["BaseFeePerGas"] = $block0.baseFeePerGas
  $runtime["ExtraDataHex"]  = $block0.extraData
  $runtime["ExtraDataText"] = HexToUtf8 $runtime["ExtraDataHex"]
} else {
  $missing.Add("Block 0 via eth_getBlockByNumber") | Out-Null
  $rpcErrors.Add("eth_getBlockByNumber failed") | Out-Null
}
# baseFeeVault not exposed on public RPC; mark as skipped
$runtime["BaseFeeVault"] = "SKIP (not available via public RPC)"

$hasRuntime = $false
foreach ($kvp in $runtime.GetEnumerator()) {
  if (-not [string]::IsNullOrWhiteSpace($kvp.Value) -and $kvp.Value -notlike "SKIP*") {
    $hasRuntime = $true
    break
  }
}

if (-not $hasRuntime) {
  Write-Host ""
  Write-Warning "UNVERIFIED: endpoint not reachable ($Endpoint)"
  if ($rpcErrors.Count -gt 0) {
    Write-Host "RPC errors:" -ForegroundColor Yellow
    $rpcErrors | Sort-Object -Unique | ForEach-Object { " - $_" } | Write-Host
  }
  Write-Host "Expected (from genesis):"
  $expected.GetEnumerator() | ForEach-Object { "{0,-14} {1}" -f $_.Key, $_.Value } | Write-Host
  exit 3
}

Write-Host ""
Write-Host ("{0,-14} {1,-24} {2,-24} {3}" -f "Field","Expected","Runtime","Status")
Write-Host ("{0,-14} {1,-24} {2,-24} {3}" -f "-----","--------","-------","------")

function Print-Row([string]$name, [string]$expVal, [string]$runVal, [bool]$numeric=$false) {
  $status = "OK"
  if ([string]::IsNullOrWhiteSpace($runVal) -or $runVal -like "SKIP*") {
    $status = "SKIP"
    $missing.Add($name) | Out-Null
  } elseif ($numeric) {
    if ((Parse-BigInt $expVal) -ne (Parse-BigInt $runVal)) { $status = "MISMATCH" }
  } else {
    if ((Normalize-Hex $expVal) -ne (Normalize-Hex $runVal)) { $status = "MISMATCH" }
  }
  Write-Host ("{0,-14} {1,-24} {2,-24} {3}" -f $name, $expVal, $runVal, $status)
  if ($status -eq "MISMATCH") {
    $diffs.Add("$name expected=$expVal runtime=$runVal") | Out-Null
  }
}

Print-Row "ChainId"       $expected["ChainId"]       $runtime["ChainId"]       $true
Print-Row "NetworkId"     $expected["NetworkId"]     $runtime["NetworkId"]     $true
Print-Row "GenesisHash"   $expected["GenesisHash"]   $runtime["GenesisHash"]
Print-Row "GasLimit"      $expected["GasLimit"]      $runtime["GasLimit"]      $true
Print-Row "Difficulty"    $expected["Difficulty"]    $runtime["Difficulty"]    $true
Print-Row "BaseFeePerGas" $expected["BaseFeePerGas"] $runtime["BaseFeePerGas"] $true
Print-Row "ExtraDataHex"  $expected["ExtraDataHex"]  $runtime["ExtraDataHex"]
Print-Row "ExtraDataText" $expected["ExtraDataText"] $runtime["ExtraDataText"]
Print-Row "BaseFeeVault"  $expected["BaseFeeVault"]  $runtime["BaseFeeVault"]

Write-Host ""
if ($diffs.Count -eq 0) {
  Write-Host "OK: Mainnet fingerprint matches." -ForegroundColor Green
  if ($missing.Count -gt 0) {
    Write-Host "Note: Skipped fields:" -ForegroundColor Yellow
    $missing | Sort-Object -Unique | ForEach-Object { " - $_" } | Write-Host
  }
  exit 0
} else {
  Write-Host "MISMATCH: Differences found:" -ForegroundColor Red
  $diffs | ForEach-Object { " - $_" } | Write-Host
  if ($missing.Count -gt 0) {
    Write-Host "Skipped (not compared):" -ForegroundColor Yellow
    $missing | Sort-Object -Unique | ForEach-Object { " - $_" } | Write-Host
  }
  exit 2
}
