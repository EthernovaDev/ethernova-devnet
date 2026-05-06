@echo off
setlocal

REM ============================================================
REM Ethernova Phase 5 - Brutal Test Suite (Windows runner)
REM Starts from TEST 03 and skips TEST 01/02.
REM ============================================================

echo.
echo ============================================================
echo   Ethernova Phase 5 - Brutal Test Suite
echo ============================================================
echo.

REM ---------- Check Node.js ----------
where node >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Node.js not found in PATH.
  echo         Install Node.js and re-run this file.
  exit /b 2
)

for /f "delims=" %%v in ('node --version') do set "NODEVER=%%v"
echo Node.js: %NODEVER%

REM ---------- Check .env ----------
if not exist .env (
  echo.
  echo [WARN] .env file not found.
  if exist .env.example (
    echo Copying from .env.example...
    copy /Y .env.example .env >nul
    echo.
    echo IMPORTANT: Edit .env and set PRIVATE_KEY before re-running.
    echo Press any key to open .env in notepad, or Ctrl+C to abort.
    pause >nul
    notepad .env
    echo.
    echo After editing .env, re-run "run.cmd"
    exit /b 0
  ) else (
    echo [ERROR] .env.example not found. Create .env manually first.
    exit /b 2
  )
)

REM ---------- Install deps if needed ----------
if not exist node_modules (
  echo.
  echo Installing npm dependencies, one-time...
  call npm install
  if errorlevel 1 (
    echo [ERROR] npm install failed.
    exit /b 2
  )
)

REM ---------- Compile contracts if needed ----------
if not exist artifacts\contracts\LifecycleHarness.sol\LifecycleHarness.json (
  echo.
  echo Compiling contracts...
  call npx hardhat compile
  if errorlevel 1 (
    echo [ERROR] hardhat compile failed.
    exit /b 2
  )
)

REM ---------- Run tests ----------
echo.
echo Starting from TEST 03. TEST 01 and TEST 02 are skipped because they already passed.
echo Runner will continue even if a suite fails.
echo Missing local ethernova_* RPC methods will fall back to Devrpc when possible.

set "START_TEST=03"
set "SKIP_TESTS=01,02"
set "CONTINUE_ON_FAIL=1"
set "CUSTOM_RPC_FALLBACK=1"

node scripts\run-all.js --start 03 --skip 01,02
set "EXITCODE=%ERRORLEVEL%"

echo.
echo ============================================================
if not "%EXITCODE%"=="0" (
  echo   RESULT: FAILED, exit code %EXITCODE%
  echo ============================================================
  exit /b %EXITCODE%
)

echo   RESULT: ALL SUITES PASSED
echo ============================================================
exit /b 0
