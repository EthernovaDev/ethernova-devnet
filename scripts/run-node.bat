@echo off
setlocal
pushd "%~dp0"
if exist "%~dp0scripts\\run-mainnet-node.bat" (
  call "%~dp0scripts\\run-mainnet-node.bat" %*
  exit /b %ERRORLEVEL%
)
echo ERROR: scripts\\run-mainnet-node.bat not found.
pause
exit /b 1
