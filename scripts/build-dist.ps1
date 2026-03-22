param(
    [string]$Version = "",
    [switch]$WindowsOnly,
    [switch]$LinuxOnly
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageScript = Join-Path $ScriptDir "package-release.ps1"

if (-not (Test-Path $PackageScript)) {
    throw "package-release.ps1 not found."
}

if ($Version) {
    & $PackageScript -Version $Version -WindowsOnly:$WindowsOnly -LinuxOnly:$LinuxOnly
} else {
    & $PackageScript -WindowsOnly:$WindowsOnly -LinuxOnly:$LinuxOnly
}
