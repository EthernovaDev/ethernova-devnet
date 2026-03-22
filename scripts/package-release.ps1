param(
    [string]$Version = "",
    [switch]$WindowsOnly,
    [switch]$LinuxOnly
)

$ErrorActionPreference = "Stop"

function Ensure-File {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        throw "Required file not found: $Path"
    }
}

function Reset-Dir {
    param([string]$Path)
    if (Test-Path $Path) { Remove-Item -Recurse -Force $Path }
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Copy-Into {
    param([string]$Source, [string]$Destination)
    Ensure-File -Path $Source
    $destDir = Split-Path -Parent $Destination
    if ($destDir -and -not (Test-Path $destDir)) {
        New-Item -ItemType Directory -Force -Path $destDir | Out-Null
    }
    Copy-Item $Source $Destination -Force
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..")).Path

if (-not $Version) {
    $VersionPath = Join-Path $RepoRoot "VERSION"
    Ensure-File -Path $VersionPath
    $Version = (Get-Content $VersionPath -Raw).Trim()
}
if (-not $Version) { throw "Version not set." }

Write-Host "Packaging release $Version"

if ($WindowsOnly -and $LinuxOnly) {
    throw "Select only one of -WindowsOnly or -LinuxOnly."
}

$buildWindows = $true
$buildLinux = $true
if ($WindowsOnly) { $buildLinux = $false }
if ($LinuxOnly) { $buildWindows = $false }

if ($buildLinux -and -not (Get-Command python -ErrorAction SilentlyContinue)) {
    throw "python not found in PATH (required to build linux tar.gz)."
}

if ($buildWindows) {
    & (Join-Path $RepoRoot "scripts\\build-windows.ps1")
}
if ($buildLinux) {
    & (Join-Path $RepoRoot "scripts\\build-linux.ps1")
}

$DistDir = Join-Path $RepoRoot "dist"
$StageWin = Join-Path $DistDir "stage-win"
$StageLinux = Join-Path $DistDir "stage-linux"
Reset-Dir -Path $DistDir
if ($buildWindows) { Reset-Dir -Path $StageWin }
if ($buildLinux) { Reset-Dir -Path $StageLinux }

$winDirs = @("bin", "genesis", "network", "scripts", "docs")
$linuxDirs = @("bin", "genesis", "network", "scripts", "docs", "systemd")
if ($buildWindows) {
    foreach ($dir in $winDirs) { New-Item -ItemType Directory -Force -Path (Join-Path $StageWin $dir) | Out-Null }
}
if ($buildLinux) {
    foreach ($dir in $linuxDirs) { New-Item -ItemType Directory -Force -Path (Join-Path $StageLinux $dir) | Out-Null }
}

# Binaries
if ($buildWindows) {
    Copy-Into -Source (Join-Path $RepoRoot "bin\\ethernova.exe") -Destination (Join-Path $StageWin "bin\\ethernova.exe")
    Copy-Into -Source (Join-Path $RepoRoot "bin\\evmcheck.exe") -Destination (Join-Path $StageWin "bin\\evmcheck.exe")
}
if ($buildLinux) {
    Copy-Into -Source (Join-Path $RepoRoot "bin\\ethernova") -Destination (Join-Path $StageLinux "bin\\ethernova")
    Copy-Into -Source (Join-Path $RepoRoot "bin\\evmcheck") -Destination (Join-Path $StageLinux "bin\\evmcheck")
}

# Genesis files
$genesisFiles = @("genesis-mainnet.json", "genesis-dev.json")
foreach ($name in $genesisFiles) {
    if ($buildWindows) { Copy-Into -Source (Join-Path $RepoRoot $name) -Destination (Join-Path $StageWin "genesis\\$name") }
    if ($buildLinux) { Copy-Into -Source (Join-Path $RepoRoot $name) -Destination (Join-Path $StageLinux "genesis\\$name") }
}

# Docs (repo -> package root)
$rootDocs = @(
    @{ Source = "docs\\runbooks\\OPERATOR_RUNBOOK.md"; Dest = "OPERATOR_RUNBOOK.md" },
    @{ Source = "docs\\README_QUICKSTART.md"; Dest = "README_QUICKSTART.md" },
    @{ Source = "docs\\releases\\RELEASE-NOTES.md"; Dest = "RELEASE-NOTES.md" },
    @{ Source = "docs\\releases\\RELEASE_NOTES_v1.2.7.md"; Dest = "RELEASE_NOTES_v1.2.7.md" },
    @{ Source = "docs\\README-WINDOWS.txt"; Dest = "README-WINDOWS.txt" },
    @{ Source = "docs\\README-LINUX.txt"; Dest = "README-LINUX.txt" },
    @{ Source = "docs\\releases\\RELEASE_v1.2.7.md"; Dest = "RELEASE_v1.2.7.md" }
)
foreach ($doc in $rootDocs) {
    $src = Join-Path $RepoRoot $doc.Source
    if ($buildWindows) { Copy-Into -Source $src -Destination (Join-Path $StageWin $doc.Dest) }
    if ($buildLinux) { Copy-Into -Source $src -Destination (Join-Path $StageLinux $doc.Dest) }
}

# Network files
if (Test-Path (Join-Path $RepoRoot "network")) {
    if ($buildWindows) { Copy-Item (Join-Path $RepoRoot "network\\*") (Join-Path $StageWin "network\\") -Force }
    if ($buildLinux) { Copy-Item (Join-Path $RepoRoot "network\\*") (Join-Path $StageLinux "network\\") -Force }
}

# Scripts
if ($buildWindows) {
    Copy-Item (Join-Path $RepoRoot "scripts\\*.ps1") (Join-Path $StageWin "scripts\\") -Force
    Copy-Item (Join-Path $RepoRoot "scripts\\*.bat") (Join-Path $StageWin "scripts\\") -Force
    Copy-Item (Join-Path $RepoRoot "scripts\\*.sh") (Join-Path $StageWin "scripts\\") -Force
}

if ($buildLinux) {
    Copy-Item (Join-Path $RepoRoot "scripts\\*.ps1") (Join-Path $StageLinux "scripts\\") -Force
    Copy-Item (Join-Path $RepoRoot "scripts\\*.bat") (Join-Path $StageLinux "scripts\\") -Force
    Copy-Item (Join-Path $RepoRoot "scripts\\*.sh") (Join-Path $StageLinux "scripts\\") -Force
}

# Root launch/update helpers
if ($buildWindows) {
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\run-node.bat") -Destination (Join-Path $StageWin "run-node.bat")
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\update.bat") -Destination (Join-Path $StageWin "update.bat")
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\update-1.2.7.bat") -Destination (Join-Path $StageWin "update-1.2.7.bat")
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\update.ps1") -Destination (Join-Path $StageWin "update.ps1")
    Copy-Into -Source (Join-Path $RepoRoot "docs\\README-WINDOWS.txt") -Destination (Join-Path $StageWin "README-WINDOWS.txt")
}

if ($buildLinux) {
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\update.sh") -Destination (Join-Path $StageLinux "update.sh")
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\update-1.2.7.sh") -Destination (Join-Path $StageLinux "update-1.2.7.sh")
    Copy-Into -Source (Join-Path $RepoRoot "scripts\\install.sh") -Destination (Join-Path $StageLinux "install.sh")
    Copy-Into -Source (Join-Path $RepoRoot "docs\\README-LINUX.txt") -Destination (Join-Path $StageLinux "README-LINUX.txt")
}

$systemdService = Join-Path $RepoRoot "systemd\\ethernova.service"
if ($buildLinux -and (Test-Path $systemdService)) {
    Copy-Into -Source $systemdService -Destination (Join-Path $StageLinux "systemd\\ethernova.service")
}

# Archive outputs
$zipName = "ethernova-windows-amd64-$Version.zip"
$zipPath = Join-Path $DistDir $zipName
if ($buildWindows) {
    if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
    Write-Host ("Creating {0}..." -f $zipName)
    Compress-Archive -Path (Join-Path $StageWin "*") -DestinationPath $zipPath -Force
}

$tarName = "ethernova-linux-amd64-$Version.tar.gz"
$tarPath = Join-Path $DistDir $tarName
if ($buildLinux) {
    if (Test-Path $tarPath) { Remove-Item $tarPath -Force }
    Write-Host ("Creating {0}..." -f $tarName)
}

$pyScript = @"
import os
import tarfile
import sys

stage = sys.argv[1]
out = sys.argv[2]

def is_exec(rel_path):
    rel_path = rel_path.replace('\\\\', '/')
    return (
        rel_path.startswith('bin/') or
        rel_path.startswith('scripts/') or
        rel_path in ('update.sh', 'update-1.2.7.sh', 'install.sh')
    )

def filter_info(ti):
    rel = ti.name.lstrip('./')
    if ti.isdir():
        ti.mode = 0o755
    elif is_exec(rel):
        ti.mode = 0o755
    else:
        ti.mode = 0o644
    return ti

with tarfile.open(out, "w:gz") as tar:
    tar.add(stage, arcname=".", filter=filter_info)
"@

$pyPath = Join-Path $DistDir "build-tar.py"
if ($buildLinux) {
    Set-Content -Path $pyPath -Value $pyScript -Encoding ASCII
    try {
        & python $pyPath $StageLinux $tarPath
    } finally {
        Remove-Item $pyPath -Force -ErrorAction SilentlyContinue
    }
}

$checksums = @()
if ($buildWindows) {
    $zipHash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
    $checksums += "$zipHash  $zipName"
}
if ($buildLinux) {
    $tarHash = (Get-FileHash -Algorithm SHA256 $tarPath).Hash.ToLower()
    $checksums += "$tarHash  $tarName"
}
if ($checksums.Count -gt 0) {
    $checksumPath = Join-Path $DistDir "checksums-sha256.txt"
    $checksums | Out-File -FilePath $checksumPath -Encoding ASCII
}

$releaseDocName = "RELEASE_$Version.md"
$releaseDocSrc = Join-Path $RepoRoot "docs\\releases\\$releaseDocName"
if (Test-Path $releaseDocSrc) {
    Copy-Into -Source $releaseDocSrc -Destination (Join-Path $DistDir $releaseDocName)
}

Write-Host "Artifacts:"
Get-ChildItem -Path $DistDir -File | ForEach-Object {
    Write-Host ("- {0} ({1} bytes)" -f $_.Name, $_.Length)
}
