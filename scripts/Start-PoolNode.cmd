@echo off
setlocal EnableExtensions
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"
pushd "%ROOT%"

set "BINARY=%ROOT%\\bin\\ethernova.exe"
if not exist "%BINARY%" set "BINARY=%ROOT%\\ethernova.exe"
set "GENESIS=%ROOT%\\genesis\\genesis-mainnet.json"
if not exist "%GENESIS%" set "GENESIS=%ROOT%\\genesis-mainnet.json"
set "DATADIR=%ROOT%\\data-mainnet"
set "LOGDIR=%ROOT%\\logs"
set "INITLOG=%LOGDIR%\init-pool.log"
set "INITERR=%LOGDIR%\init-pool.err.log"
set "NODELOG=%LOGDIR%\pool-node.log"
set "NODEERR=%LOGDIR%\pool-node.err.log"

if "%~1"=="" goto ASKETHER
set "ETHERBASE=%~1"
goto CHECKETHER

:ASKETHER
set "ETHERBASE="
echo Enter pool etherbase (0x...):
set /p ETHERBASE=>

:CHECKETHER
if "%ETHERBASE%"=="" (
    echo ERROR: Etherbase is required. Example: scripts\\Start-PoolNode.cmd 0x1234...
    goto END
)

if not exist "%BINARY%" (
    echo ERROR: %BINARY% not found.
    goto END
)
if not exist "%GENESIS%" (
    echo ERROR: %GENESIS% not found.
    goto END
)

if not exist "%LOGDIR%" mkdir "%LOGDIR%"
if not exist "%DATADIR%" mkdir "%DATADIR%"

set "INITNEEDED=yes"
if exist "%DATADIR%\geth\chaindata" set "INITNEEDED=no"
if exist "%DATADIR%\geth\LOCK" set "INITNEEDED=no"

echo Init needed: %INITNEEDED%
if "%INITNEEDED%"=="yes" (
    echo Initializing genesis (see %INITLOG% / %INITERR%)
    "%BINARY%" --datadir "%DATADIR%" init "%GENESIS%" 1>>"%INITLOG%" 2>>"%INITERR%"
    if errorlevel 1 (
        echo ERROR: init failed. Check %INITLOG% and %INITERR%
        goto END
    )
    echo Init done.
)

set "HTTPPORT=8545"
set "WSPORT=8546"
netstat -ano | findstr ":8545" >nul 2>&1
if %errorlevel%==0 (
    echo Port 8545 busy, falling back to 8547/8548.
    set "HTTPPORT=8547"
    set "WSPORT=8548"
)

echo Starting pool node on http://127.0.0.1:%HTTPPORT% (ws %WSPORT%)...
start "" /B "%BINARY%" --datadir "%DATADIR%" --networkid 121525 --port 30303 --http --http.addr 127.0.0.1 --http.port %HTTPPORT% --http.api eth,net,web3,txpool --ws --ws.addr 127.0.0.1 --ws.port %WSPORT% --ws.api eth,net,web3,txpool --mine --miner.etherbase %ETHERBASE% --miner.threads 1 --verbosity 3 1>>"%NODELOG%" 2>>"%NODEERR%"

echo Waiting for RPC...
timeout /t 8 >nul

powershell -ExecutionPolicy Bypass -File "%ROOT%\\scripts\\test-rpc.ps1" -Endpoint http://127.0.0.1:%HTTPPORT%
powershell -ExecutionPolicy Bypass -File "%ROOT%\\scripts\\verify-mainnet.ps1" -Endpoint http://127.0.0.1:%HTTPPORT%

echo.
echo Logs: %NODELOG% and %NODEERR%
echo Press Ctrl+C to stop the node (window stays open).
pause >nul

:END
popd
endlocal
