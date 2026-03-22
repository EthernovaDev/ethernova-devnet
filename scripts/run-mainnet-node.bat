@echo off
setlocal
pushd "%~dp0"
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"
set "LOG_FILE=%ROOT%\\node.log"

powershell -ExecutionPolicy Bypass -File "%~dp0run-mainnet-node.ps1" %*
set "EXITCODE=%ERRORLEVEL%"

if not "%EXITCODE%"=="0" (
  echo.
  echo ERROR: run-mainnet-node failed with exit code %EXITCODE%.
  if exist "%LOG_FILE%" (
    echo Last 50 lines from %LOG_FILE%:
    powershell -NoProfile -Command "Get-Content -Path '%LOG_FILE%' -Tail 50"
  ) else (
    echo Log file not found: %LOG_FILE%
  )
  echo.
  pause
)

popd
exit /b %EXITCODE%
