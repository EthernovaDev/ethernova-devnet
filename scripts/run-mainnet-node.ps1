param(
    [string]$DataDir = "",
    [int]$HttpPort = 8545,
    [int]$WsPort = 8546,
    [string]$BootnodesFile = "",
    [string]$Bootnodes = "",
    [switch]$Mine
)

$ErrorActionPreference = "Stop"

function Resolve-FirstPath {
    param([string[]]$Candidates)
    foreach ($path in $Candidates) {
        if (Test-Path $path) {
            return (Resolve-Path $path).Path
        }
    }
    return $null
}

function Test-PortInUse {
    param([int]$Port)
    if (Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue) {
        try {
            $matches = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction Stop
            return ($matches.Count -gt 0)
        } catch {
        }
    }

    try {
        $lines = netstat -ano -p tcp | Select-String -Pattern "LISTENING"
        foreach ($line in $lines) {
            if ($line -match "[:\\.]$Port\\s") {
                return $true
            }
        }
    } catch {
    }

    return $false
}

function Write-Command {
    param([string]$Exe, [string[]]$CmdArgs)
    if ($CmdArgs) {
        Write-Host ("Running: {0} {1}" -f $Exe, ($CmdArgs -join " "))
    } else {
        Write-Host ("Running: {0}" -f $Exe)
    }
}

function Show-LogTail {
    param([string]$Path)
    if (Test-Path $Path) {
        Write-Host ""
        Write-Host ("Last 50 lines from {0}:" -f $Path)
        Get-Content -Path $Path -Tail 50 | ForEach-Object { Write-Host $_ }
    }
}

function Show-Logo {
    $logo = @(
        "      .       /\       .      ",
        "         .   /  \   .         ",
        "            / /\\ \\            ",
        "     .     / /  \\ \\     .     ",
        "          / / /\\ \\ \\          ",
        "         /_/ /  \\ \\_\\         ",
        "         \\ \\ \\  / / /         ",
        "          \\ \\ \\/ / /          ",
        "     .     \\  /\\  /     .     ",
        "             \\/  \\/             ",
        "             /\\  /\\             ",
        "            /  \\/  \\            ",
        "           / /\\  /\\ \\           ",
        "     .    /_/  \\/  \\_\\    .    "
    )
    $colors = @("DarkCyan", "Cyan", "Blue", "White", "Blue", "Cyan", "DarkCyan")
    for ($i = 0; $i -lt $logo.Count; $i++) {
        $color = $colors[$i % $colors.Count]
        Write-Host $logo[$i] -ForegroundColor $color
    }
}

function Show-Banner {
    param(
        [string]$DataDir,
        [int]$HttpPort,
        [int]$WsPort,
        [string]$BootnodesUsed,
        [string]$LogPath
    )
    Write-Host ""
    Write-Host "=============================================================" -ForegroundColor DarkCyan
    Write-Host "                 ETHERNOVA MAINNET NODE" -ForegroundColor White
    Write-Host "=============================================================" -ForegroundColor DarkCyan
    Show-Logo
    Write-Host ""
    Write-Host ("Data dir : {0}" -f $DataDir)
    Write-Host ("HTTP RPC : 127.0.0.1:{0}" -f $HttpPort)
    Write-Host ("WS RPC   : 127.0.0.1:{0}" -f $WsPort)
    if ($BootnodesUsed) {
        Write-Host ("Bootnodes: {0}" -f $BootnodesUsed)
    } else {
        Write-Host "Bootnodes: (none)"
    }
    Write-Host ("Logs     : {0}" -f $LogPath)
    Write-Host "Press Ctrl+C to stop."
    Write-Host ""
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..")).Path

$Ethernova = Resolve-FirstPath @(
    (Join-Path $RepoRoot "bin\\ethernova.exe"),
    (Join-Path $RepoRoot "ethernova.exe")
)
if (-not $Ethernova) { throw "ethernova.exe not found (expected bin\\ethernova.exe or root)." }

if (-not $DataDir) {
    $DataDir = Join-Path $RepoRoot "data-mainnet"
}

if (-not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
}

$GenesisMainnet = Resolve-FirstPath @(
    (Join-Path $RepoRoot "genesis\\genesis-mainnet.json"),
    (Join-Path $RepoRoot "genesis-mainnet.json")
)
if (-not $GenesisMainnet) { throw "genesis-mainnet.json not found." }

$ChainDataDir = Join-Path $DataDir "geth\\chaindata"
if (-not (Test-Path $ChainDataDir)) {
    Write-Host "Initializing datadir (idempotent init, no wipe)..."
    Write-Command -Exe $Ethernova -CmdArgs @("--datadir", $DataDir, "init", $GenesisMainnet)
    & $Ethernova --datadir $DataDir init $GenesisMainnet | Out-Null
}

$ConfigPath = ""
$StaticNodesSrc = Join-Path $RepoRoot "network\static-nodes.json"
$DeprecatedStatic = Join-Path $DataDir "geth\\static-nodes.json"
if ((Test-Path $DeprecatedStatic) -and (Test-Path $StaticNodesSrc)) {
    $deprecatedContent = (Get-Content -Path $DeprecatedStatic -Raw).Trim()
    $sourceContent = (Get-Content -Path $StaticNodesSrc -Raw).Trim()
    if ($deprecatedContent -and ($deprecatedContent -eq $sourceContent)) {
        $deprecatedBackup = Join-Path $DataDir "geth\\static-nodes.deprecated.json"
        Move-Item -Path $DeprecatedStatic -Destination $deprecatedBackup -Force
        Write-Host ("Moved deprecated static-nodes.json to {0}" -f $deprecatedBackup) -ForegroundColor Yellow
    } else {
        Write-Host ("Warning: deprecated static-nodes.json exists at {0} (client ignores it)." -f $DeprecatedStatic) -ForegroundColor Yellow
    }
}

if (Test-Path $StaticNodesSrc) {
    try {
        $StaticNodes = Get-Content -Path $StaticNodesSrc -Raw | ConvertFrom-Json
    } catch {
        $StaticNodes = @()
        Write-Host ("Warning: invalid static-nodes.json at {0}" -f $StaticNodesSrc) -ForegroundColor Yellow
    }

    $StaticList = @()
    if ($StaticNodes -is [System.Collections.IEnumerable]) {
        foreach ($node in $StaticNodes) {
            if ($node) { $StaticList += $node.ToString().Trim() }
        }
    }

    $ValidStatic = @()
    foreach ($node in $StaticList) {
        if ($node -match "^enode://[0-9a-fA-F]+@") {
            $ValidStatic += $node
        }
    }

    if ($StaticList.Count -gt 0 -and $ValidStatic.Count -eq 0) {
        Write-Host "Warning: static-nodes.json has placeholders or invalid enodes; skipping config." -ForegroundColor Yellow
    }

    if ($ValidStatic.Count -gt 0) {
        $ConfigPath = Join-Path $DataDir "config.mainnet.toml"
        $escaped = $ValidStatic | ForEach-Object { '"' + ($_.Replace('"', '\"')) + '"' }
        $content = "[Node.P2P]`nStaticNodes = [" + ($escaped -join ", ") + "]`n"
        Set-Content -Path $ConfigPath -Value $content -Encoding ASCII
        Write-Host ("Static nodes (config): {0}" -f $ConfigPath)
    }
}

$LogPath = Join-Path $RepoRoot "node.log"
if (Test-Path $LogPath) {
    Clear-Content -Path $LogPath
} else {
    New-Item -ItemType File -Force -Path $LogPath | Out-Null
}

$BootnodeList = @()
if ($Bootnodes) {
    $BootnodeList = $Bootnodes -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ }
} else {
    if (-not $BootnodesFile) {
        $BootnodesFile = Join-Path $RepoRoot "network\bootnodes.txt"
    }
    if (Test-Path $BootnodesFile) {
        $BootnodeList = Get-Content $BootnodesFile | ForEach-Object { $_.Trim() } | Where-Object { $_ -and -not $_.StartsWith("#") }
    }
}

$Args = @(
    "--datadir", $DataDir,
    "--networkid", "121525",
    "--http", "--http.addr", "127.0.0.1", "--http.port", "$HttpPort",
    "--http.api", "eth,net,web3,debug",
    "--ws", "--ws.addr", "127.0.0.1", "--ws.port", "$WsPort",
    "--ws.api", "eth,net,web3,debug"
)

if ($BootnodeList.Count -gt 0) {
    $BootnodesUsed = $BootnodeList -join ","
} else {
    $BootnodesUsed = ""
}

if ($Mine) {
    $Args += @("--mine")
}

if (Test-PortInUse -Port $HttpPort) {
    throw ("Port {0} is already in use. Stop the process using it or set -HttpPort." -f $HttpPort)
}
if (Test-PortInUse -Port $WsPort) {
    throw ("Port {0} is already in use. Stop the process using it or set -WsPort." -f $WsPort)
}

if ($BootnodesUsed) {
    $Args += @("--bootnodes", $BootnodesUsed)
}
if ($ConfigPath) {
    $Args += @("--config", $ConfigPath)
}

Show-Banner -DataDir $DataDir -HttpPort $HttpPort -WsPort $WsPort -BootnodesUsed $BootnodesUsed -LogPath $LogPath

$exitCode = 0
$prevErrPref = $ErrorActionPreference
$prevNativeErrPref = $null
if (Test-Path variable:PSNativeCommandUseErrorActionPreference) {
    $prevNativeErrPref = $PSNativeCommandUseErrorActionPreference
}
try {
    Write-Command -Exe $Ethernova -CmdArgs $Args
    Write-Host "Streaming logs..."
    $ErrorActionPreference = "Continue"
    if ($null -ne $prevNativeErrPref) {
        $PSNativeCommandUseErrorActionPreference = $false
    }
    & $Ethernova @Args 2>&1 | ForEach-Object {
        $line = $_
        Add-Content -Path $LogPath -Value $line
        if ($line -match "ERROR") {
            Write-Host $line -ForegroundColor Red
        } elseif ($line -match "WARN") {
            Write-Host $line -ForegroundColor Yellow
        } elseif ($line -match "INFO") {
            Write-Host $line -ForegroundColor Cyan
        } else {
            Write-Host $line
        }
    }
    $exitCode = $LASTEXITCODE
} catch {
    Write-Error $_.Exception.Message
    $exitCode = 1
} finally {
    if ($null -ne $prevErrPref) {
        $ErrorActionPreference = $prevErrPref
    }
    if ($null -ne $prevNativeErrPref) {
        $PSNativeCommandUseErrorActionPreference = $prevNativeErrPref
    }
    Show-LogTail -Path $LogPath
}

exit $exitCode
