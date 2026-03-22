$ErrorActionPreference = "Stop"

Write-Host "Building Ethernova (Windows amd64, CGO disabled)..."

Set-Location (Split-Path $PSScriptRoot -Parent)

$versionTag = (Get-Content -Path "VERSION" -TotalCount 1).Trim()
$versionDate = Get-Date -Format "yyyyMMdd"
$outDir = "dist"
$exeName = "ethernova-$versionTag-windows-amd64.exe"
$ldflags = "-X github.com/ethereum/go-ethereum/internal/version.gitCommit=$versionTag -X github.com/ethereum/go-ethereum/internal/version.gitDate=$versionDate"

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

if (-not (Test-Path $outDir)) { New-Item -ItemType Directory -Force -Path $outDir | Out-Null }

go build -trimpath -buildvcs=false -ldflags $ldflags -o "$outDir\\$exeName" .\\cmd\\geth

Write-Host "Built $outDir\\$exeName"
