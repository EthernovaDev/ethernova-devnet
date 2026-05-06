@echo off
REM ============================================================
REM Ethernova Phase 5 - Brutal Test Suite (Windows runner)
REM ============================================================

echo.
echo ============================================================
echo   Ethernova Phase 5 - Brutal Test Suite
echo ============================================================
echo.

REM ---------- Check Node.js ----------
where node >nul 2>nul
if errorlevel 1 goto :no_node

for /f "delims=" %%v in ('node --version') do set NODEVER=%%v
echo Node.js: %NODEVER%

REM ---------- Check .env ----------
if not exist .env goto :no_env
goto :check_modules

:no_node
echo [ERROR] Node.js not found in PATH.
echo         Install from https://nodejs.org and re-run.
exit /b 2

:no_env
echo.
echo [WARN] .env file not found. Copying from .env.example...
copy /Y .env.example .env >nul
echo.
echo IMPORTANT: Edit .env and set PRIVATE_KEY before re-running.
echo            Press any key to open .env in notepad, or Ctrl+C to abort.
pause >nul
notepad .env
echo.
echo After editing .env, re-run "run.cmd"
exit /b 0

REM ---------- Install deps if needed ----------
:check_modules
if exist node_modules goto :check_compile

echo.
echo Installing npm dependencies, one-time...
call npm install
if errorlevel 1 goto :install_failed
goto :check_compile

:install_failed
echo [ERROR] npm install failed.
exit /b 2

REM ---------- Compile contracts if needed ----------
:check_compile
if exist artifacts\contracts\LifecycleHarness.sol\LifecycleHarness.json goto :run_tests

echo.
echo Compiling contracts...
call npx hardhat compile
if errorlevel 1 goto :compile_failed
goto :run_tests

:compile_failed
echo [ERROR] hardhat compile failed.
exit /b 2

REM ---------- Run tests ----------
:run_tests
echo.
node scripts\run-all.js
set EXITCODE=%errorlevel%

echo.
echo ============================================================
if %EXITCODE% EQU 0 goto :pass
echo   RESULT: FAILED, exit code %EXITCODE%
echo ============================================================
exit /b %EXITCODE%

:pass
echo   RESULT: ALL SUITES PASSED
echo ============================================================
exit /b 0
