$ErrorActionPreference = "Stop"

Write-Host "Building ethernova (Linux amd64, CGO disabled)..."

Set-Location (Split-Path $PSScriptRoot -Parent)

$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

if (-not (Test-Path "bin")) { New-Item -ItemType Directory -Force -Path "bin" | Out-Null }

go build -o "bin\\ethernova" .\\cmd\\geth
go build -o "bin\\evmcheck" .\\cmd\\evmcheck

Write-Host "Built bin\\ethernova and bin\\evmcheck"
