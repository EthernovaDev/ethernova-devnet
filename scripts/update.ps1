param(
    [string]$ReleaseVersion = "v1.2.7",
    [string]$GitHubRepo = "",
    [string]$BaseUrl = ""
)

$ErrorActionPreference = "Stop"

function Resolve-Root {
    param([string]$ScriptDir)
    if ((Split-Path $ScriptDir -Leaf) -ieq "scripts") {
        return (Resolve-Path (Join-Path $ScriptDir "..")).Path
    }
    return (Resolve-Path $ScriptDir).Path
}

function Write-Info([string]$Message) {
    Write-Host $Message
}

function Resolve-GitHubRepoFromGit {
    param([string]$Root)
    $git = Get-Command git -ErrorAction SilentlyContinue
    if (-not $git) { return $null }
    try {
        $url = & git -C $Root remote get-url origin 2>$null
    } catch {
        return $null
    }
    if (-not $url) { return $null }
    if ($url -match "github\\.com[:/](?<owner>[^/]+)/(?<repo>[^/.]+)") {
        return "$($Matches.owner)/$($Matches.repo)"
    }
    return $null
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Resolve-Root -ScriptDir $ScriptDir

$DryRun = $false
if ($env:DRY_RUN -eq "1") { $DryRun = $true }

if (-not $GitHubRepo) { $GitHubRepo = $env:GITHUB_REPO }
if (-not $BaseUrl) { $BaseUrl = $env:RELEASE_URL_BASE }
if (-not $GitHubRepo) { $GitHubRepo = Resolve-GitHubRepoFromGit -Root $Root }
if (-not $GitHubRepo) { $GitHubRepo = "EthernovaDev/ethernova-coregeth" }

if (-not $BaseUrl) {
    $BaseUrl = "https://github.com/$GitHubRepo/releases/download"
}

$ReleaseUrlBase = $BaseUrl
Write-Info "Using GitHub repo: $GitHubRepo"

$ZipName = "ethernova-windows-amd64-$ReleaseVersion.zip"
$ZipUrl = "$ReleaseUrlBase/$ReleaseVersion/$ZipName"
$ChecksumsUrl = "$ReleaseUrlBase/$ReleaseVersion/checksums-sha256.txt"

$TempDir = Join-Path $Root "update-temp"
if (Test-Path $TempDir) { Remove-Item $TempDir -Recurse -Force }
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

$ZipPath = Join-Path $TempDir $ZipName
$ChecksumsPath = Join-Path $TempDir "checksums-sha256.txt"

Write-Info "Downloading $ZipUrl"
Invoke-WebRequest -Uri $ZipUrl -OutFile $ZipPath

Write-Info "Downloading $ChecksumsUrl"
try {
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $ChecksumsPath
} catch {
    Write-Info "Checksums file not available; skipping verification."
    $ChecksumsPath = ""
}

if ($ChecksumsPath) {
    $expected = $null
    foreach ($line in Get-Content $ChecksumsPath) {
        $parts = $line -split "\s+"
        if ($parts.Count -ge 2 -and $parts[1] -eq $ZipName) {
            $expected = $parts[0].ToLower()
            break
        }
    }
    if ($expected) {
        $actual = (Get-FileHash -Algorithm SHA256 -Path $ZipPath).Hash.ToLower()
        if ($actual -ne $expected) {
            throw "Checksum mismatch for $ZipName"
        }
        Write-Info "Checksum OK."
    } else {
        Write-Info "Checksum entry not found; skipping verification."
    }
}

if ($DryRun) {
    Write-Info "DRY_RUN=1 set. Skipping extraction and install."
    exit 0
}

Write-Info "Backing up existing binaries (if any)..."
$backupRoot = Join-Path $Root "backup"
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backupDir = Join-Path $backupRoot $timestamp
$candidates = @(
    (Join-Path $Root "bin\\ethernova.exe"),
    (Join-Path $Root "ethernova.exe")
)
$backedUp = $false
foreach ($candidate in $candidates) {
    if (Test-Path $candidate) {
        if (-not $backedUp) {
            New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
            $backedUp = $true
        }
        Copy-Item $candidate (Join-Path $backupDir (Split-Path $candidate -Leaf)) -Force
    }
}
if ($backedUp) {
    Write-Info "Backup stored at $backupDir"
}

Write-Info "Stopping running ethernova process (if any)..."
Get-Process ethernova -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

Write-Info "Extracting update..."
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

$newBinary = Get-ChildItem -Path $TempDir -Recurse -Filter "ethernova.exe" | Select-Object -First 1
if (-not $newBinary) { throw "ethernova.exe not found in update package." }

$destBin = Join-Path $Root "bin"
if (-not (Test-Path $destBin)) { New-Item -ItemType Directory -Force -Path $destBin | Out-Null }
Copy-Item $newBinary.FullName (Join-Path $destBin "ethernova.exe") -Force
Write-Info "Updated bin\\ethernova.exe"

$genesisDir = Join-Path $Root "genesis"
if (-not (Test-Path $genesisDir)) { New-Item -ItemType Directory -Force -Path $genesisDir | Out-Null }
foreach ($name in @("genesis-mainnet.json")) {
    $src = Get-ChildItem -Path $TempDir -Recurse -Filter $name | Select-Object -First 1
    if ($src) {
        Copy-Item $src.FullName (Join-Path $genesisDir $name) -Force
        Write-Info "Updated genesis\\$name"
    }
}

$srcScriptsDir = Get-ChildItem -Path $TempDir -Recurse -Directory -Filter "scripts" |
    Where-Object { Test-Path (Join-Path $_.FullName "run-mainnet-node.ps1") } |
    Select-Object -First 1
$destScriptsDir = Join-Path $Root "scripts"
if ($srcScriptsDir) {
    if (-not (Test-Path $destScriptsDir)) { New-Item -ItemType Directory -Force -Path $destScriptsDir | Out-Null }
    Copy-Item (Join-Path $srcScriptsDir.FullName "*") $destScriptsDir -Recurse -Force
    Write-Info "Updated scripts\\"
}

foreach ($name in @("run-node.bat", "update.bat", "update-1.2.7.bat", "update.ps1", "README-WINDOWS.txt")) {
    $src = Get-ChildItem -Path $TempDir -Recurse -Filter $name | Select-Object -First 1
    if ($src) {
        Copy-Item $src.FullName (Join-Path $Root $name) -Force
        Write-Info "Updated $name"
    }
}

$runScript = Join-Path $Root "run-node.bat"
if (-not (Test-Path $runScript)) {
    $runScript = Join-Path $Root "scripts\\run-mainnet-node.bat"
}

if (Test-Path $runScript) {
    Write-Info "Restarting node..."
    Start-Process -FilePath $runScript -WorkingDirectory $Root
} else {
    Write-Info "Run script not found; start the node manually."
}

Write-Info "Update complete."
