Param(
    [string]$Endpoint = "http://127.0.0.1:8545"
)

$ErrorActionPreference = "Stop"
$ExpectedGenesisHash = "0xc3812eb81498965a3f9ff3e73d2f423934e6d440578d4f4fbb6623cc61c453d9"

function Call-Rpc([string]$method, [object[]]$params=@()) {
    $payload = @{
        jsonrpc = "2.0"
        id      = 1
        method  = $method
        params  = $params
    } | ConvertTo-Json -Compress
    try {
        return Invoke-RestMethod -Method Post -Uri $Endpoint -Body $payload -ContentType "application/json"
    } catch {
        return $null
    }
}

function Print-Result($name, $ok, $extra="") {
    if ($ok) {
        Write-Host "OK   $name $extra"
    } else {
        Write-Host "FAIL $name $extra" -ForegroundColor Red
    }
}

$chain = Call-Rpc "eth_chainId"
$okChain = $chain -and $chain.result
Print-Result "eth_chainId" $okChain "($($chain.result))"

$block0 = Call-Rpc "eth_getBlockByNumber" @("0x0",$false)
$blockHash = $null
if ($block0 -and $block0.result) { $blockHash = $block0.result.hash }
$okBlock = $false
if ($blockHash) {
    $okBlock = ($blockHash.ToLower() -eq $ExpectedGenesisHash.ToLower())
}
Print-Result "genesis hash" $okBlock "hash=$blockHash"

$work = Call-Rpc "eth_getWork"
$okWork = $work -and $work.result -and $work.result.Count -ge 3
$workHint = if ($okWork) { "" } else { "(start mining or enable getWork)" }
Print-Result "eth_getWork" $okWork $workHint
