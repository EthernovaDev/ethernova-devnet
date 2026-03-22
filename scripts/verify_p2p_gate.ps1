param()

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path $PSScriptRoot -Parent

function Resolve-Go {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($goCmd) {
        return $goCmd.Path
    }
    $localGo = Join-Path $RepoRoot ".tools\go\bin\go.exe"
    if (Test-Path $localGo) {
        $env:GOROOT = Join-Path $RepoRoot ".tools\go"
        $env:PATH = "$env:GOROOT\bin;$env:PATH"
        return $localGo
    }
    return $null
}

$go = Resolve-Go
if (-not $go) {
    Write-Host "FAIL: go not found. Install Go 1.21+ or place it at .tools\go."
    exit 1
}

Write-Host "== P2P version gate verification =="
$output = & $go test ./eth -run TestVerifyPeerVersionGate -v 2>&1
$output | ForEach-Object { Write-Host $_ }

$rejectLine = $output | Select-String -SimpleMatch 'VERIFY_P2P_GATE: name="CoreGeth/v1.2.6'
$acceptLine = $output | Select-String -SimpleMatch 'VERIFY_P2P_GATE: name="CoreGeth/v1.2.7'
$rejectOk = $rejectLine -and ($rejectLine -match "rejected")
$acceptOk = $acceptLine -and ($acceptLine -match "accepted")

if ($rejectOk -and $acceptOk) {
    Write-Host "PASS: P2P version gate"
    exit 0
}

Write-Host "FAIL: P2P version gate"
exit 1
