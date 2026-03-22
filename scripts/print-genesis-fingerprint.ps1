Param(
    [string]$Endpoint = "\\.\\pipe\\ethernova-mainnet.ipc"
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path $PSScriptRoot -Parent
$Binary   = Join-Path $RepoRoot "bin\ethernova.exe"
if (-not (Test-Path $Binary)) { throw "Binary not found at $Binary" }

Write-Host "Reading genesis fingerprint from $Endpoint ..."
$expr = @"
var b = eth.getBlock(0);
var cfg = {};
if (typeof admin !== 'undefined' && admin.nodeInfo && admin.nodeInfo.protocols && admin.nodeInfo.protocols.eth) {
  cfg = admin.nodeInfo.protocols.eth.config;
}
if (!cfg.chainId && eth.chainId) { cfg.chainId = eth.chainId(); }
if (!cfg.networkId && net && net.version) { cfg.networkId = parseInt(net.version); }
JSON.stringify({hash:b.hash, gasLimit:b.gasLimit, baseFeePerGas:b.baseFeePerGas, coinbase:b.miner, config:cfg});
"@
$out = & $Binary attach --exec $expr -- $Endpoint 2>&1

$jsonLine = ($out -split "`r?`n") | Where-Object { $_ -match '\{.*\}' } | Select-Object -Last 1
if (-not $jsonLine) {
    Write-Host "---- RAW OUTPUT ----"
    $out | ForEach-Object { Write-Host $_ }
    throw "Could not parse fingerprint JSON."
}

$jsonClean = $jsonLine
if ($jsonClean.StartsWith('"') -and $jsonClean.EndsWith('"')) {
    $jsonClean = $jsonClean.Trim('"')
}
$jsonClean = $jsonClean -replace '\\\"','"'

$fingerprint = $jsonClean | ConvertFrom-Json

Write-Host "Block0 hash:      $($fingerprint.hash)"
Write-Host "Gas limit:        $($fingerprint.gasLimit)"
Write-Host "BaseFeePerGas:    $($fingerprint.baseFeePerGas)"
Write-Host "Coinbase:         $($fingerprint.coinbase)"
Write-Host "Config.chainId:   $($fingerprint.config.chainId)"
Write-Host "Config.networkId: $($fingerprint.config.networkId)"
Write-Host "BaseFeeVault:     $($fingerprint.config.baseFeeVault)"
Write-Host "Forks: eip155=$($fingerprint.config.eip155Block) london/eip1559=$($fingerprint.config.eip1559FBlock)"
Write-Host "Full config JSON:"
Write-Host ($jsonLine)
