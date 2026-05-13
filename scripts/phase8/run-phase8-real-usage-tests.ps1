# ============================================================================
# Phase 8 - Nova RPC Namespace and Tooling - Real-Usage Test Runner
# ============================================================================
#
# Orchestrates the full Phase 8 test suite on Windows PowerShell.
# - Loads scripts/phase8/.env into the current process environment.
# - Validates required env vars (PHASE8_RPC_URL, PHASE8_CHAIN_ID).
# - Creates a timestamped report directory under PHASE8_REPORT_DIR.
# - Sets PHASE8_REPORT_DIR_RUN so all node scripts write into the same folder.
# - Runs the test sequence and captures per-scenario logs.
# - Generates summary.json (aggregate) and summary.md (human-readable).
# - Prints PASS / PARTIAL / FAIL verdict and exits 0 or 1 accordingly.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\phase8\run-phase8-real-usage-tests.ps1
#
# This script uses only PowerShell built-ins, node, and npx. No WSL, no Bash.
# All output is ASCII; no em-dashes, no smart quotes.
# ============================================================================

[CmdletBinding()]
param()

$ErrorActionPreference = "Continue"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Get-Item (Join-Path $ScriptDir "..\..")).FullName
Set-Location $RepoRoot

Write-Host "========================================================================"
Write-Host " Phase 8 - Nova RPC and Tooling - Real-Usage Test Runner"
Write-Host "========================================================================"
Write-Host "Repo root: $RepoRoot"
Write-Host "Script dir: $ScriptDir"

# ----------------------------------------------------------------------------
# Step 1. Load .env
# ----------------------------------------------------------------------------
$EnvFile = Join-Path $ScriptDir ".env"
if (Test-Path $EnvFile) {
    Write-Host "Loading env from: $EnvFile"
    Get-Content $EnvFile | ForEach-Object {
        $line = $_.Trim()
        if ($line.Length -eq 0) { return }
        if ($line.StartsWith("#")) { return }
        $eqIdx = $line.IndexOf("=")
        if ($eqIdx -lt 1) { return }
        $key = $line.Substring(0, $eqIdx).Trim()
        $val = $line.Substring($eqIdx + 1).Trim()
        # Strip optional surrounding quotes.
        if ($val.Length -ge 2) {
            $first = $val.Substring(0, 1)
            $last = $val.Substring($val.Length - 1, 1)
            if (($first -eq '"' -and $last -eq '"') -or ($first -eq "'" -and $last -eq "'")) {
                $val = $val.Substring(1, $val.Length - 2)
            }
        }
        [System.Environment]::SetEnvironmentVariable($key, $val, "Process")
    }
} else {
    Write-Host "WARN: $EnvFile not found. Using existing process environment."
    Write-Host "      Copy scripts\phase8\.env.example to scripts\phase8\.env first."
}

# ----------------------------------------------------------------------------
# Step 2. Validate required vars
# ----------------------------------------------------------------------------
$RpcUrl = [System.Environment]::GetEnvironmentVariable("PHASE8_RPC_URL", "Process")
$ChainId = [System.Environment]::GetEnvironmentVariable("PHASE8_CHAIN_ID", "Process")

if ([string]::IsNullOrWhiteSpace($RpcUrl)) {
    Write-Host "FATAL: PHASE8_RPC_URL is not set. Aborting."
    exit 2
}
if ([string]::IsNullOrWhiteSpace($ChainId)) {
    Write-Host "WARN: PHASE8_CHAIN_ID is not set. Defaulting to 121526."
    [System.Environment]::SetEnvironmentVariable("PHASE8_CHAIN_ID", "121526", "Process")
    $ChainId = "121526"
}

Write-Host "RPC URL: $RpcUrl"
Write-Host "Chain ID: $ChainId"

# ----------------------------------------------------------------------------
# Step 3. Verify node and npx are available
# ----------------------------------------------------------------------------
$NodeVersion = & node --version 2>$null
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($NodeVersion)) {
    Write-Host "FATAL: node is not on PATH. Install Node.js 18+ first."
    exit 2
}
Write-Host "node: $NodeVersion"

$NpxVersion = & npx --version 2>$null
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($NpxVersion)) {
    Write-Host "WARN: npx is not on PATH. Hardhat plugin tests will be skipped."
}

# ----------------------------------------------------------------------------
# Step 4. Create timestamped report directory
# ----------------------------------------------------------------------------
$ReportRoot = [System.Environment]::GetEnvironmentVariable("PHASE8_REPORT_DIR", "Process")
if ([string]::IsNullOrWhiteSpace($ReportRoot)) { $ReportRoot = "reports\phase8\real-usage" }
$ReportRoot = $ReportRoot.Replace("/", "\")
if (-not [System.IO.Path]::IsPathRooted($ReportRoot)) {
    $ReportRoot = Join-Path $RepoRoot $ReportRoot
}
$Stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$ReportDirRun = Join-Path $ReportRoot $Stamp
New-Item -Path $ReportDirRun -ItemType Directory -Force | Out-Null
[System.Environment]::SetEnvironmentVariable("PHASE8_REPORT_DIR_RUN", $ReportDirRun, "Process")

Write-Host "Report dir: $ReportDirRun"
Write-Host ""

# ----------------------------------------------------------------------------
# Step 5. Helper - run a node script with logging
# ----------------------------------------------------------------------------
function Invoke-NodeScript {
    param(
        [string]$Label,
        [string]$Script,
        [string]$LogName,
        [string]$Phase
    )
    Write-Host "------------------------------------------------------------------------"
    Write-Host "[PHASE $Phase] $Label"
    Write-Host "------------------------------------------------------------------------"
    $LogPath = Join-Path $ReportDirRun $LogName
    $ScriptPath = Join-Path $ScriptDir $Script
    if (-not (Test-Path $ScriptPath)) {
        Write-Host "SKIP: $ScriptPath not found"
        "[SKIP] $Label - script not found: $ScriptPath" | Out-File -FilePath $LogPath -Encoding ascii
        return 2
    }
    & node $ScriptPath 2>&1 | Tee-Object -FilePath $LogPath -Append
    $code = $LASTEXITCODE
    Write-Host ""
    return $code
}

# ----------------------------------------------------------------------------
# Step 6. Run scenarios in order
# ----------------------------------------------------------------------------
$ResultsTable = @{}

$ResultsTable["create-fixtures"] = Invoke-NodeScript `
    -Label "Optional fixture creation (Mailbox + ContentRef)" `
    -Script "phase8-create-fixtures.js" `
    -LogName "create-fixtures.log" `
    -Phase "0"

# If fixtures.json was produced and exported new env vars, fold them into the
# current process environment so downstream scripts pick them up.
$FixturesJson = Join-Path $ReportDirRun "fixtures.json"
if (Test-Path $FixturesJson) {
    try {
        $fx = Get-Content $FixturesJson -Raw | ConvertFrom-Json
        if ($fx.envExport) {
            $fx.envExport.PSObject.Properties | ForEach-Object {
                [System.Environment]::SetEnvironmentVariable($_.Name, $_.Value, "Process")
                Write-Host ("  exported: {0}={1}" -f $_.Name, $_.Value)
            }
        }
    } catch {
        Write-Host "WARN: could not parse fixtures.json: $_"
    }
}

$ResultsTable["rpc-real-usage"] = Invoke-NodeScript `
    -Label "Raw JSON-RPC scenarios A through M" `
    -Script "phase8-rpc-real-usage.js" `
    -LogName "rpc-real-usage.log" `
    -Phase "1"

$ResultsTable["malformed-rpc"] = Invoke-NodeScript `
    -Label "Malformed input - liveness checks (Scenario N)" `
    -Script "phase8-rpc-malformed.js" `
    -LogName "malformed-rpc.log" `
    -Phase "2"

$EnableLoad = [System.Environment]::GetEnvironmentVariable("PHASE8_ENABLE_LOAD_TEST", "Process")
if ($EnableLoad -ne "false") {
    $ResultsTable["load-test"] = Invoke-NodeScript `
        -Label "Concurrent load test (Scenario O)" `
        -Script "phase8-rpc-load.js" `
        -LogName "load-test.log" `
        -Phase "3"
} else {
    Write-Host "[PHASE 3] Load test disabled (PHASE8_ENABLE_LOAD_TEST=false)"
    "[SKIP] Load test disabled by env" | Out-File -FilePath (Join-Path $ReportDirRun "load-test.log") -Encoding ascii
    $ResultsTable["load-test"] = 2
}

$EnableSdk = [System.Environment]::GetEnvironmentVariable("PHASE8_ENABLE_SDK", "Process")
if ($EnableSdk -ne "false") {
    $ResultsTable["sdk-test"] = Invoke-NodeScript `
        -Label "SDK real-usage parity (Scenario P)" `
        -Script "phase8-sdk-real-usage.js" `
        -LogName "sdk-test.log" `
        -Phase "4"
} else {
    Write-Host "[PHASE 4] SDK test disabled"
    "[SKIP] SDK test disabled by env" | Out-File -FilePath (Join-Path $ReportDirRun "sdk-test.log") -Encoding ascii
    $ResultsTable["sdk-test"] = 2
}

$EnableHardhat = [System.Environment]::GetEnvironmentVariable("PHASE8_ENABLE_HARDHAT_PLUGIN", "Process")
if ($EnableHardhat -ne "false") {
    $HardhatProj = Join-Path $ScriptDir "hardhat-test-project"
    if (Test-Path (Join-Path $HardhatProj "node_modules")) {
        $ResultsTable["hardhat-plugin"] = Invoke-NodeScript `
            -Label "Hardhat plugin nova:* tasks (Scenario Q)" `
            -Script "phase8-hardhat-plugin-real-usage.js" `
            -LogName "hardhat-plugin-test.log" `
            -Phase "5"
    } else {
        Write-Host "[PHASE 5] Hardhat plugin test SKIPPED - node_modules missing"
        Write-Host "          Run: cd scripts\phase8\hardhat-test-project && npm install"
        "[SKIP] hardhat-test-project not installed" | Out-File -FilePath (Join-Path $ReportDirRun "hardhat-plugin-test.log") -Encoding ascii
        $ResultsTable["hardhat-plugin"] = 2
    }
} else {
    Write-Host "[PHASE 5] Hardhat plugin test disabled"
    "[SKIP] Hardhat plugin test disabled by env" | Out-File -FilePath (Join-Path $ReportDirRun "hardhat-plugin-test.log") -Encoding ascii
    $ResultsTable["hardhat-plugin"] = 2
}

$EnableMultiRpc = [System.Environment]::GetEnvironmentVariable("PHASE8_ENABLE_MULTI_RPC", "Process")
if ($EnableMultiRpc -eq "true") {
    $ResultsTable["multirpc-consistency"] = Invoke-NodeScript `
        -Label "Multi-RPC consistency (Scenario S)" `
        -Script "phase8-multirpc-consistency.js" `
        -LogName "multirpc-consistency.log" `
        -Phase "6"
} else {
    Write-Host "[PHASE 6] Multi-RPC consistency disabled (PHASE8_ENABLE_MULTI_RPC != true)"
    "[SKIP] Multi-RPC disabled by env" | Out-File -FilePath (Join-Path $ReportDirRun "multirpc-consistency.log") -Encoding ascii
    $ResultsTable["multirpc-consistency"] = 2
}

$ResultsTable["standard-tooling-compat"] = Invoke-NodeScript `
    -Label "Standard tooling compatibility (Scenario R)" `
    -Script "phase8-standard-tooling-compat.js" `
    -LogName "standard-tooling-compat.log" `
    -Phase "7"

# ----------------------------------------------------------------------------
# Step 7. Aggregate summary
# ----------------------------------------------------------------------------
Write-Host "========================================================================"
Write-Host " Aggregating summary"
Write-Host "========================================================================"

$SummaryObj = [ordered]@{
    suite = "phase8-real-usage"
    rpcUrl = $RpcUrl
    chainId = $ChainId
    startedAt = (Get-Date).ToString("o")
    reportDir = $ReportDirRun
    scenarios = [ordered]@{}
    aggregateCounts = [ordered]@{ pass = 0; fail = 0; warn = 0; skip = 0 }
    highestSeverity = $null
    knownGaps = @(
        "BUG-1 nova_listProtocolObjects missing (HIGH)",
        "BUG-2 nova_getDeferredStats(blockNumber) missing (MEDIUM)",
        "BUG-3 nova_getMessages uses fromIndex not fromBlock (MEDIUM)",
        "BUG-4 SDK buildOpenChatSessionInput stale (HIGH)"
    )
}

$JsonNames = @(
    "create-fixtures.json",
    "rpc-real-usage.json",
    "malformed-rpc.json",
    "load-test.json",
    "sdk-test.json",
    "hardhat-plugin-test.json",
    "multirpc-consistency.json",
    "standard-tooling-compat.json"
)

$sevOrder = @{ "low" = 1; "medium" = 2; "high" = 3; "critical" = 4 }
$worstSev = $null

foreach ($name in $JsonNames) {
    $p = Join-Path $ReportDirRun $name
    if (-not (Test-Path $p)) {
        $SummaryObj.scenarios[$name] = [ordered]@{ present = $false }
        continue
    }
    try {
        $obj = Get-Content $p -Raw | ConvertFrom-Json
    } catch {
        $SummaryObj.scenarios[$name] = [ordered]@{ present = $true; parseError = $_.Exception.Message }
        continue
    }
    $entry = [ordered]@{
        present = $true
        suite = $obj.suite
        counts = $obj.counts
        highestSeverity = $obj.highestSeverity
    }
    $SummaryObj.scenarios[$name] = $entry
    if ($obj.counts) {
        if ($obj.counts.pass) { $SummaryObj.aggregateCounts.pass += [int]$obj.counts.pass }
        if ($obj.counts.fail) { $SummaryObj.aggregateCounts.fail += [int]$obj.counts.fail }
        if ($obj.counts.warn) { $SummaryObj.aggregateCounts.warn += [int]$obj.counts.warn }
        if ($obj.counts.skip) { $SummaryObj.aggregateCounts.skip += [int]$obj.counts.skip }
    }
    if ($obj.highestSeverity) {
        $sv = [string]$obj.highestSeverity
        if ($sevOrder.ContainsKey($sv)) {
            $cur = if ($worstSev -and $sevOrder.ContainsKey($worstSev)) { $sevOrder[$worstSev] } else { 0 }
            if ($sevOrder[$sv] -gt $cur) { $worstSev = $sv }
        }
    }
}
$SummaryObj.highestSeverity = $worstSev
$SummaryObj.endedAt = (Get-Date).ToString("o")

# Verdict logic
# PASS    = no failures at all (or only docs-known BUG-1/BUG-2 misses; tracked
#           specifically by the rpc-real-usage suite via its own severity)
# PARTIAL = failures exist but the highest severity is medium AND fails are
#           all attributable to documented audit gaps
# FAIL    = anything with high/critical severity failure
$verdict = "PASS"
if ($SummaryObj.aggregateCounts.fail -gt 0) {
    if ($worstSev -eq "critical" -or $worstSev -eq "high") {
        # Check if all high/critical fails are documented audit gaps
        # (BUG-1 = nova_listProtocolObjects, BUG-2 covered via medium).
        # Conservative default: treat any high failure as FAIL unless
        # rpc-real-usage was the only failing suite AND its highest sev was high
        # (which is the BUG-1 case).
        $rpc = $SummaryObj.scenarios["rpc-real-usage.json"]
        $onlyRpcFails = $true
        foreach ($k in $SummaryObj.scenarios.Keys) {
            if ($k -eq "rpc-real-usage.json") { continue }
            $entry = $SummaryObj.scenarios[$k]
            if ($entry.counts -and [int]$entry.counts.fail -gt 0) {
                $onlyRpcFails = $false
                break
            }
        }
        if ($onlyRpcFails -and $rpc -and $rpc.highestSeverity -eq "high") {
            $verdict = "PARTIAL"
        } else {
            $verdict = "FAIL"
        }
    } else {
        $verdict = "PARTIAL"
    }
}
$SummaryObj.verdict = $verdict

# Write summary.json
$SummaryJsonPath = Join-Path $ReportDirRun "summary.json"
$SummaryObj | ConvertTo-Json -Depth 12 | Out-File -FilePath $SummaryJsonPath -Encoding ascii
Write-Host "Wrote: $SummaryJsonPath"

# Write summary.md
$Md = New-Object System.Text.StringBuilder
[void]$Md.AppendLine("# Phase 8 Real-Usage Test Report")
[void]$Md.AppendLine("")
[void]$Md.AppendLine("- RPC URL: $RpcUrl")
[void]$Md.AppendLine("- Chain ID: $ChainId")
[void]$Md.AppendLine("- Started: $($SummaryObj.startedAt)")
[void]$Md.AppendLine("- Ended: $($SummaryObj.endedAt)")
[void]$Md.AppendLine("- Report dir: $ReportDirRun")
[void]$Md.AppendLine("- Verdict: **$verdict**")
[void]$Md.AppendLine("- Highest severity: $worstSev")
[void]$Md.AppendLine("")
[void]$Md.AppendLine("## Aggregate counts")
[void]$Md.AppendLine("")
[void]$Md.AppendLine("- pass: $($SummaryObj.aggregateCounts.pass)")
[void]$Md.AppendLine("- fail: $($SummaryObj.aggregateCounts.fail)")
[void]$Md.AppendLine("- warn: $($SummaryObj.aggregateCounts.warn)")
[void]$Md.AppendLine("- skip: $($SummaryObj.aggregateCounts.skip)")
[void]$Md.AppendLine("")
[void]$Md.AppendLine("## Per-scenario results")
[void]$Md.AppendLine("")
[void]$Md.AppendLine("| Scenario | Present | Pass | Fail | Warn | Skip | Highest sev |")
[void]$Md.AppendLine("|---|---|---|---|---|---|---|")
foreach ($k in $SummaryObj.scenarios.Keys) {
    $e = $SummaryObj.scenarios[$k]
    $present = if ($e.present) { "yes" } else { "no" }
    $p = if ($e.counts) { $e.counts.pass } else { "-" }
    $f = if ($e.counts) { $e.counts.fail } else { "-" }
    $w = if ($e.counts) { $e.counts.warn } else { "-" }
    $s = if ($e.counts) { $e.counts.skip } else { "-" }
    $sv = if ($e.highestSeverity) { $e.highestSeverity } else { "-" }
    [void]$Md.AppendLine("| $k | $present | $p | $f | $w | $s | $sv |")
}
[void]$Md.AppendLine("")
[void]$Md.AppendLine("## Documented audit gaps")
[void]$Md.AppendLine("")
foreach ($g in $SummaryObj.knownGaps) {
    [void]$Md.AppendLine("- $g")
}
[void]$Md.AppendLine("")
[void]$Md.AppendLine("See PHASE8_AUDIT.md (repo root) for full bug details.")
$SummaryMdPath = Join-Path $ReportDirRun "summary.md"
$Md.ToString() | Out-File -FilePath $SummaryMdPath -Encoding ascii
Write-Host "Wrote: $SummaryMdPath"

# ----------------------------------------------------------------------------
# Step 8. Final verdict + exit code
# ----------------------------------------------------------------------------
Write-Host ""
Write-Host "========================================================================"
Write-Host " VERDICT: $verdict"
Write-Host "========================================================================"
Write-Host " pass=$($SummaryObj.aggregateCounts.pass) fail=$($SummaryObj.aggregateCounts.fail) warn=$($SummaryObj.aggregateCounts.warn) skip=$($SummaryObj.aggregateCounts.skip)"
Write-Host " highest severity: $worstSev"
Write-Host " report: $ReportDirRun"
Write-Host "========================================================================"

if ($verdict -eq "FAIL") { exit 1 }
if ($verdict -eq "PARTIAL") { exit 1 }
exit 0
