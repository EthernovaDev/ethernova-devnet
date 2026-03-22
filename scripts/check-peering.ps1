Param(
    [string]$RpcA = "http://127.0.0.1:8545",
    [string]$RpcB = "http://127.0.0.1:8547"
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path $PSScriptRoot -Parent
$Binary = Join-Path $RepoRoot "bin\ethernova.exe"
if (-not (Test-Path $Binary)) { throw "Binary not found at $Binary" }

function Get-PeerInfo($rpc) {
    $_old = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & $Binary attach --exec "JSON.stringify({peerCount:net.peerCount, peers:admin.peers.map(p=>p.enode)})" $rpc 2>&1
    $ErrorActionPreference = $_old
    $line = ($out -split "`r?`n") | Where-Object { $_ -match '\{.*\}' } | Select-Object -Last 1
    if (-not $line) { return $null }
    $jsonClean = $line
    if ($jsonClean.StartsWith('"') -and $jsonClean.EndsWith('"')) { $jsonClean = $jsonClean.Trim('"') }
    $jsonClean = $jsonClean -replace '\\\"','"'
    return $jsonClean | ConvertFrom-Json
}

$a = Get-PeerInfo $RpcA
$b = Get-PeerInfo $RpcB

Write-Host "Node A ($RpcA): peers=$($a.peerCount) enodes=$($a.peers -join ', ')"
Write-Host "Node B ($RpcB): peers=$($b.peerCount) enodes=$($b.peers -join ', ')"

if ($a.peerCount -gt 0 -and $b.peerCount -gt 0) {
    Write-Host "Peering looks healthy (both nodes report peers)."
} else {
    Write-Warning "Peering incomplete. Inspect admin.peers on both nodes."
}
