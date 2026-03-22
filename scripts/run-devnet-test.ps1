param(
    [string]$DataDir = "",
    [switch]$KeepRunning
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

function Format-Args {
    param(
        [string[]]$CmdArgs,
        [switch]$RedactPk
    )
    $out = New-Object System.Collections.Generic.List[string]
    for ($i = 0; $i -lt $CmdArgs.Count; $i++) {
        $arg = $CmdArgs[$i]
        if ($RedactPk -and $arg -eq "--pk" -and ($i + 1) -lt $CmdArgs.Count) {
            $out.Add($arg)
            $out.Add("[redacted]")
            $i++
            continue
        }
        if ($arg -match "\s") {
            $out.Add('"' + $arg + '"')
        } else {
            $out.Add($arg)
        }
    }
    return ($out -join " ")
}

function Write-Command {
    param(
        [string]$Exe,
        [string[]]$CmdArgs,
        [switch]$RedactPk
    )
    $display = Format-Args -CmdArgs $CmdArgs -RedactPk:$RedactPk
    if ($display) {
        Write-Host ("Running: {0} {1}" -f $Exe, $display)
    } else {
        Write-Host ("Running: {0}" -f $Exe)
    }
}

function Run-Command {
    param(
        [string]$Exe,
        [string[]]$CmdArgs,
        [switch]$RedactPk
    )
    Write-Command -Exe $Exe -CmdArgs $CmdArgs -RedactPk:$RedactPk
    & $Exe @CmdArgs
}

function Ensure-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "go not found in PATH. Install Go or use prebuilt binaries in bin\."
    }
}

function Ensure-Binaries {
    param([string]$RepoRoot)

    $binDir = Join-Path $RepoRoot "bin"
    $eth = Resolve-FirstPath @(
        (Join-Path $binDir "ethernova.exe"),
        (Join-Path $RepoRoot "ethernova.exe")
    )
    $evm = Resolve-FirstPath @(
        (Join-Path $binDir "evmcheck.exe"),
        (Join-Path $RepoRoot "evmcheck.exe")
    )
    if ($eth -and $evm) {
        return @{ Ethernova = $eth; Evmcheck = $evm }
    }

    Ensure-Go
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null

    $prevCgo = $env:CGO_ENABLED
    $env:CGO_ENABLED = "0"
    Push-Location $RepoRoot
    try {
        Run-Command -Exe "go" -CmdArgs @("build", "-o", (Join-Path $binDir "ethernova.exe"), ".\cmd\geth")
        Run-Command -Exe "go" -CmdArgs @("build", "-o", (Join-Path $binDir "evmcheck.exe"), ".\cmd\evmcheck")
    } finally {
        Pop-Location
        if ($null -eq $prevCgo) {
            Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
        } else {
            $env:CGO_ENABLED = $prevCgo
        }
    }

    $eth = Resolve-FirstPath @((Join-Path $binDir "ethernova.exe"))
    $evm = Resolve-FirstPath @((Join-Path $binDir "evmcheck.exe"))
    if (-not $eth -or -not $evm) {
        throw "Failed to build binaries in bin\."
    }
    return @{ Ethernova = $eth; Evmcheck = $evm }
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..")).Path

$binaries = Ensure-Binaries -RepoRoot $RepoRoot
$Ethernova = $binaries.Ethernova
$Evmcheck = $binaries.Evmcheck

$GenesisPath = Resolve-FirstPath @(
    (Join-Path $RepoRoot "genesis\genesis-devnet-fork20.json"),
    (Join-Path $RepoRoot "genesis-devnet-fork20.json")
)
if (-not $GenesisPath) { throw "genesis-devnet-fork20.json not found." }

$KeyPath = Resolve-FirstPath @(
    (Join-Path $RepoRoot "genesis\devnet-testkey.txt"),
    (Join-Path $RepoRoot "devnet-testkey.txt")
)
if (-not $KeyPath) { throw "devnet-testkey.txt not found." }

$KeyData = @{}
Get-Content $KeyPath | ForEach-Object {
    if ($_ -match '^\s*([A-Z_]+)\s*=\s*(.+)\s*$') {
        $KeyData[$matches[1]] = $matches[2].Trim()
    }
}

$PrivKey = $KeyData["PRIVATE_KEY"]
$DevAddr = $KeyData["ADDRESS"]
$ChainID = [uint64]$KeyData["CHAINID"]

if (-not $PrivKey -or -not $DevAddr -or -not $ChainID) {
    throw "devnet-testkey.txt missing PRIVATE_KEY, ADDRESS, or CHAINID."
}

if (-not $DataDir) {
    $DataDir = Join-Path $RepoRoot "data-devnet"
}

$RpcUrl = "http://127.0.0.1:8545"
$ForkBlock = 20
$TargetBlock = $ForkBlock + 2

if (-not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
}

$ChainData = Join-Path $DataDir "geth"
if (-not (Test-Path $ChainData)) {
    Write-Command -Exe $Ethernova -CmdArgs @("--datadir", $DataDir, "init", $GenesisPath)
    & $Ethernova --datadir $DataDir init $GenesisPath | Out-Null
}

$LogsDir = Join-Path $RepoRoot "logs"
if (-not (Test-Path $LogsDir)) {
    New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null
}
$LogPath = Join-Path $LogsDir "devnet-test.log"
$Args = @(
    "--datadir", $DataDir,
    "--http", "--http.addr", "127.0.0.1", "--http.port", "8545",
    "--http.api", "eth,net,web3,debug",
    "--ws", "--ws.addr", "127.0.0.1", "--ws.port", "8546",
    "--ws.api", "eth,net,web3,debug",
    "--nodiscover", "--maxpeers", "0",
    "--networkid", "$ChainID",
    "--mine", "--fakepow", "--miner.threads", "1", "--miner.etherbase", $DevAddr
)

Write-Command -Exe $Ethernova -CmdArgs $Args
Write-Host "Starting devnet node..."
$ErrLogPath = Join-Path $LogsDir "devnet-test.err.log"
$proc = Start-Process -FilePath $Ethernova -ArgumentList $Args -PassThru -RedirectStandardOutput $LogPath -RedirectStandardError $ErrLogPath

function Get-BlockNumber {
    param([string]$Url)
    $body = '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
    try {
        $resp = Invoke-RestMethod -Method Post -Uri $Url -ContentType "application/json" -Body $body -TimeoutSec 5
        if ($resp.result) {
            return [Convert]::ToInt64($resp.result.Replace("0x", ""), 16)
        }
    } catch {
        return $null
    }
    return $null
}

$deadline = (Get-Date).AddMinutes(3)
try {
    Write-Host "Waiting for RPC..."
    do {
        $bn = Get-BlockNumber -Url $RpcUrl
        if ($bn -ne $null) { break }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)

    if ($bn -eq $null) { throw "RPC did not become ready in time. Check $LogPath." }

    Write-Host ("Mining until block >= {0}..." -f $TargetBlock)
    do {
        Start-Sleep -Seconds 1
        $bn = Get-BlockNumber -Url $RpcUrl
    } while ($bn -lt $TargetBlock)

    Run-Command -Exe $Evmcheck -CmdArgs @("--rpc", $RpcUrl, "--pk", $PrivKey, "--chainid", "$ChainID", "--forkblock", "$ForkBlock") -RedactPk
    $exitCode = $LASTEXITCODE
} finally {
    if (-not $KeepRunning) {
        Write-Host "Stopping devnet node..."
        try { Stop-Process -Id $proc.Id -Force } catch {}
    } else {
        Write-Host "Devnet left running. Logs: $LogPath"
    }
}

exit $exitCode
