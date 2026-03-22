Param(
    [string]$Rpc = "http://127.0.0.1:8545",
    [string]$Pass = "nova-smoke",
    [string]$Root = ""
)

$ErrorActionPreference = "Stop"

$RepoRoot  = Split-Path $PSScriptRoot -Parent
if (-not $Root) { $Root = $RepoRoot }
$Binary    = Join-Path $RepoRoot "bin\ethernova.exe"
$Vault     = "0x3a38560b66205bb6a31decbcb245450b2f15d4fd"
$Miner     = "0x3a38560b66205bb6a31decbcb245450b2f15d4fd"
$ScriptPath = Join-Path $RepoRoot "scripts\smoke-test-fees.js"

if (-not (Test-Path $Binary)) { throw "Binary not found at $Binary" }
if (-not (Test-Path $ScriptPath)) { throw "Smoke test JS not found at $ScriptPath" }

Write-Host "Running baseFee vault smoke test against $Rpc ..."

$_oldPref = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$cmdOut = & $Binary attach --preload $ScriptPath --exec "runSmoke('$Vault','$Miner','$Pass')" $Rpc 2>&1
$ErrorActionPreference = $_oldPref
if (-not $cmdOut) {
    Write-Error "No response from attach."
    exit 1
}

$jsonLine = ($cmdOut -split "`r?`n") | Where-Object { $_ -match '\{.*\}' } | Select-Object -Last 1
if (-not $jsonLine) {
    Write-Host "---- RAW OUTPUT ----"
    $cmdOut | ForEach-Object { Write-Host $_ }
    Write-Error "No JSON payload returned."
    exit 1
}

$jsonClean = $jsonLine
if ($jsonClean.StartsWith('"') -and $jsonClean.EndsWith('"')) {
    $jsonClean = $jsonClean.Trim('"')
}
$jsonClean = $jsonClean -replace '\\\"','"'

$result = $jsonClean | ConvertFrom-Json

if (-not $result.ok) {
    Write-Host "---- RAW OUTPUT ----"
    $cmdOut | ForEach-Object { Write-Host $_ }
    Write-Error "Smoke test failed: $($result.error) Hint: $($result.hint)"
    exit 1
}

$before   = [System.Numerics.BigInteger]::Parse($result.before)
$after    = [System.Numerics.BigInteger]::Parse($result.after)
$expected = [System.Numerics.BigInteger]::Parse($result.expectedDelta)
$delta    = $after - $before

Write-Host "Tx hash: $($result.txHash)"
Write-Host "Block: $($result.blockNumber) baseFee: $($result.baseFeePerGas) gasUsed: $($result.gasUsed)"
Write-Host "Vault delta: $delta"
Write-Host "Expected:    $expected"

if ($delta -ne $expected) {
    Write-Host "---- txpool.status ----"
    & $Binary attach --exec "txpool.status" $Rpc
    Write-Host "---- tx ----"
    & $Binary attach --exec "eth.getTransaction('$($result.txHash)')" $Rpc
    Write-Host "---- pending ----"
    & $Binary attach --exec "txpool.inspect.pending" $Rpc
    Write-Error "Vault delta mismatch."
    exit 1
}

Write-Host $jsonLine
Write-Host "Smoke test passed: baseFee was redirected to vault and miner kept the tip."
