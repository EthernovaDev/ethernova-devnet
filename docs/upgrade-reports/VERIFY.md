# Verification Checklist

Replace <RPC_URL>, <FROM>, <TO>, <DATA>, <TX_HASH> as needed.

## RPC Health

curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"web3_clientVersion","params":[]}' <RPC_URL>
curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' <RPC_URL>
curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' <RPC_URL>

## Gas Estimation (CALL{value} forwarder)

curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_estimateGas","params":[{"from":"<FROM>","to":"<TO>","value":"0x1","data":"<DATA>"}]}' <RPC_URL>

## Receipt Status (post 110500 and 118200)

curl -s -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_getTransactionReceipt","params":["<TX_HASH>"]}' <RPC_URL>
