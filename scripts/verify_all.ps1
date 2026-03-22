param()

$ErrorActionPreference = "Stop"

$scriptDir = $PSScriptRoot
$ok = $true

Write-Host "== Running full verification pack =="

& (Join-Path $scriptDir "verify_p2p_gate.ps1")
if ($LASTEXITCODE -ne 0) { $ok = $false }

& (Join-Path $scriptDir "verify-fork-windows.ps1")
if ($LASTEXITCODE -ne 0) { $ok = $false }

if ($ok) {
    Write-Host "VERIFY_ALL: PASS"
    exit 0
}

Write-Host "VERIFY_ALL: FAIL"
exit 1
