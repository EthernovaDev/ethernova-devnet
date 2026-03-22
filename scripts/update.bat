@echo off
setlocal
pushd "%~dp0"
if exist "%~dp0update.ps1" (
  powershell -ExecutionPolicy Bypass -File "%~dp0update.ps1" %*
  exit /b %ERRORLEVEL%
)
if exist "%~dp0scripts\\update.ps1" (
  powershell -ExecutionPolicy Bypass -File "%~dp0scripts\\update.ps1" %*
  exit /b %ERRORLEVEL%
)
echo ERROR: update.ps1 not found.
pause
exit /b 1
