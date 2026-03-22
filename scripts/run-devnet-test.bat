@echo off
setlocal
powershell -ExecutionPolicy Bypass -File "%~dp0run-devnet-test.ps1"
exit /b %ERRORLEVEL%
