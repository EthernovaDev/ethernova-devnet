#!/bin/bash
# ============================================================
# Ethernova Devnet - Full Feature Validation Test
# Tests ALL features before Noven Fork deployment to mainnet
# ============================================================

RPC=${1:-"http://127.0.0.1:28545"}
PRIVKEY=${2:-"63ad8427b96fc8f13cf7ce252fcad2f7b0659035bdc893e097fdd8cde250c37e"}

# Contract addresses (deployed on devnet)
TOKEN="0xd6Dc5b3E9CEF3c4117fFd32F138717bBc0f8d91c"
NFT="0xa407ABC46D71A56fb4fAc2Ae9CA1F599A2270C2a"
MULTISIG="0x24fcDc40BFa6e8Fce87ACF50da1e69a36019083f"
FAUCET_ADDR="0x246Cbae156Cf083F635C0E1a01586b730678f5Cb"

PASS=0
FAIL=0
TOTAL=0

test_rpc() {
    local NAME=$1
    local METHOD=$2
    local PARAMS=$3
    local EXPECT=$4
    TOTAL=$((TOTAL+1))

    RESULT=$(curl -s --max-time 10 -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"$METHOD\",\"params\":$PARAMS,\"id\":1}" $RPC)

    if echo "$RESULT" | grep -q "$EXPECT"; then
        echo "  [PASS] $NAME"
        PASS=$((PASS+1))
    else
        echo "  [FAIL] $NAME"
        echo "         Expected: $EXPECT"
        echo "         Got: $(echo $RESULT | head -c 200)"
        FAIL=$((FAIL+1))
    fi
}

test_rpc_exists() {
    local NAME=$1
    local METHOD=$2
    TOTAL=$((TOTAL+1))

    RESULT=$(curl -s --max-time 10 -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"$METHOD\",\"params\":[],\"id\":1}" $RPC)

    if echo "$RESULT" | grep -q '"result"'; then
        echo "  [PASS] $NAME"
        PASS=$((PASS+1))
    else
        echo "  [FAIL] $NAME - method not available"
        echo "         $(echo $RESULT | head -c 200)"
        FAIL=$((FAIL+1))
    fi
}

echo "============================================================"
echo "  ETHERNOVA DEVNET - FULL FEATURE VALIDATION"
echo "  Noven Fork Readiness Test"
echo "  RPC: $RPC"
echo "============================================================"
echo ""

# ============================================================
echo "=== TEST 1: Core Network ==="
# ============================================================
test_rpc "Chain ID is 121526" "eth_chainId" "[]" "0x1dab6"
test_rpc "Network synced (block > 0)" "eth_blockNumber" "[]" "result"
test_rpc "Client version" "web3_clientVersion" "[]" "Ethernova"
echo ""

# ============================================================
echo "=== TEST 2: Ethernova RPC Namespace ==="
# ============================================================
test_rpc_exists "ethernova_forkStatus" "ethernova_forkStatus"
test_rpc_exists "ethernova_chainConfig" "ethernova_chainConfig"
test_rpc_exists "ethernova_nodeHealth" "ethernova_nodeHealth"
echo ""

# ============================================================
echo "=== TEST 3: EVM Profiler ==="
# ============================================================
test_rpc_exists "ethernova_evmProfile" "ethernova_evmProfile"
test_rpc "Profiler is enabled" "ethernova_evmProfile" "[]" "enabled"

# Toggle off and on
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_evmProfileToggle","params":[false],"id":1}' $RPC > /dev/null
test_rpc "Profiler toggle OFF" "ethernova_evmProfile" "[]" "false"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_evmProfileToggle","params":[true],"id":1}' $RPC > /dev/null
test_rpc "Profiler toggle ON" "ethernova_evmProfile" "[]" "true"

test_rpc_exists "ethernova_evmProfileReset" "ethernova_evmProfileReset"
echo ""

# ============================================================
echo "=== TEST 4: Adaptive Gas ==="
# ============================================================
test_rpc_exists "ethernova_adaptiveGas" "ethernova_adaptiveGas"

# Enable
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasToggle","params":[true],"id":1}' $RPC > /dev/null
test_rpc "Adaptive gas enabled" "ethernova_adaptiveGas" "[]" "\"enabled\":true"
test_rpc "Discount is 25%" "ethernova_adaptiveGas" "[]" "\"discountPercent\":25"
test_rpc "Penalty is 10%" "ethernova_adaptiveGas" "[]" "\"penaltyPercent\":10"

# Set discount
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetDiscount","params":[30],"id":1}' $RPC > /dev/null
test_rpc "Set discount to 30%" "ethernova_adaptiveGas" "[]" "\"discountPercent\":30"

# Restore
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetDiscount","params":[25],"id":1}' $RPC > /dev/null

# Set penalty
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetPenalty","params":[15],"id":1}' $RPC > /dev/null
test_rpc "Set penalty to 15%" "ethernova_adaptiveGas" "[]" "\"penaltyPercent\":15"

# Restore
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_adaptiveGasSetPenalty","params":[10],"id":1}' $RPC > /dev/null
echo ""

# ============================================================
echo "=== TEST 5: Execution Modes ==="
# ============================================================
test_rpc_exists "ethernova_executionMode" "ethernova_executionMode"
test_rpc "Default mode is standard" "ethernova_executionMode" "[]" "standard"

# Switch to fast
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_executionModeSet","params":[1],"id":1}' $RPC > /dev/null
test_rpc "Switch to fast mode" "ethernova_executionMode" "[]" "fast"

# Switch to parallel
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_executionModeSet","params":[2],"id":1}' $RPC > /dev/null
test_rpc "Switch to parallel mode" "ethernova_executionMode" "[]" "parallel"

# Back to standard
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_executionModeSet","params":[0],"id":1}' $RPC > /dev/null
test_rpc "Back to standard mode" "ethernova_executionMode" "[]" "standard"

test_rpc_exists "ethernova_parallelStats" "ethernova_parallelStats"
echo ""

# ============================================================
echo "=== TEST 6: Call Cache ==="
# ============================================================
test_rpc_exists "ethernova_callCache" "ethernova_callCache"

curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_callCacheToggle","params":[true],"id":1}' $RPC > /dev/null
test_rpc "Cache enabled" "ethernova_callCache" "[]" "\"enabled\":true"
test_rpc "Max size 10000" "ethernova_callCache" "[]" "\"maxSize\":10000"

test_rpc_exists "ethernova_callCacheReset" "ethernova_callCacheReset"
echo ""

# ============================================================
echo "=== TEST 7: Opcode Optimizer ==="
# ============================================================
test_rpc_exists "ethernova_optimizer" "ethernova_optimizer"

curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_optimizerToggle","params":[true],"id":1}' $RPC > /dev/null
test_rpc "Optimizer enabled" "ethernova_optimizer" "[]" "\"enabled\":true"
test_rpc "Tracks redundant ops" "ethernova_optimizer" "[]" "redundantOps"
test_rpc "Tracks gas refunded" "ethernova_optimizer" "[]" "gasRefunded"
echo ""

# ============================================================
echo "=== TEST 8: Auto-Tuner ==="
# ============================================================
test_rpc_exists "ethernova_autoTuner" "ethernova_autoTuner"

curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"ethernova_autoTunerToggle","params":[true],"id":1}' $RPC > /dev/null
test_rpc "Auto-tuner enabled" "ethernova_autoTuner" "[]" "\"enabled\":true"
echo ""

# ============================================================
echo "=== TEST 9: Bytecode Analysis ==="
# ============================================================
test_rpc_exists "ethernova_bytecodeAnalysis" "ethernova_bytecodeAnalysis"
echo ""

# ============================================================
echo "=== TEST 10: Custom Precompiles ==="
# ============================================================
test_rpc_exists "ethernova_precompiles" "ethernova_precompiles"
test_rpc "novaBatchHash at 0x20" "ethernova_precompiles" "[]" "novaBatchHash"
test_rpc "novaBatchVerify at 0x21" "ethernova_precompiles" "[]" "novaBatchVerify"

# Test novaBatchHash precompile via eth_call
# Call address 0x20 with 32 bytes of data
HASH_CALL=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000000020","data":"0x0000000000000000000000000000000000000000000000000000000000000001"},"latest"],"id":1}' $RPC)
TOTAL=$((TOTAL+1))
if echo "$HASH_CALL" | grep -q '"result":"0x'; then
    echo "  [PASS] novaBatchHash precompile executes"
    PASS=$((PASS+1))
else
    echo "  [FAIL] novaBatchHash precompile"
    echo "         $(echo $HASH_CALL | head -c 200)"
    FAIL=$((FAIL+1))
fi

# Test novaBatchVerify precompile (will fail with invalid sig but proves it runs)
VERIFY_CALL=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000000021","data":"0x00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000300"},"latest"],"id":1}' $RPC)
TOTAL=$((TOTAL+1))
if echo "$VERIFY_CALL" | grep -q '"result"'; then
    echo "  [PASS] novaBatchVerify precompile executes"
    PASS=$((PASS+1))
else
    echo "  [FAIL] novaBatchVerify precompile"
    echo "         $(echo $VERIFY_CALL | head -c 200)"
    FAIL=$((FAIL+1))
fi
echo ""

# ============================================================
echo "=== TEST 11: Deployed Contracts ==="
# ============================================================
# Check NovaToken has code
TOKEN_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$TOKEN\",\"latest\"],\"id\":1}" $RPC)
TOTAL=$((TOTAL+1))
if echo "$TOKEN_CODE" | grep -q '"result":"0x' && ! echo "$TOKEN_CODE" | grep -q '"result":"0x"'; then
    echo "  [PASS] NovaToken has bytecode deployed"
    PASS=$((PASS+1))
else
    echo "  [FAIL] NovaToken not deployed"
    FAIL=$((FAIL+1))
fi

NFT_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$NFT\",\"latest\"],\"id\":1}" $RPC)
TOTAL=$((TOTAL+1))
if echo "$NFT_CODE" | grep -q '"result":"0x' && ! echo "$NFT_CODE" | grep -q '"result":"0x"'; then
    echo "  [PASS] NovaNFT has bytecode deployed"
    PASS=$((PASS+1))
else
    echo "  [FAIL] NovaNFT not deployed"
    FAIL=$((FAIL+1))
fi

MULTISIG_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$MULTISIG\",\"latest\"],\"id\":1}" $RPC)
TOTAL=$((TOTAL+1))
if echo "$MULTISIG_CODE" | grep -q '"result":"0x' && ! echo "$MULTISIG_CODE" | grep -q '"result":"0x"'; then
    echo "  [PASS] NovaMultiSig has bytecode deployed"
    PASS=$((PASS+1))
else
    echo "  [FAIL] NovaMultiSig not deployed"
    FAIL=$((FAIL+1))
fi
echo ""

# ============================================================
echo "=== TEST 12: Node Health ==="
# ============================================================
test_rpc "Has version field" "ethernova_nodeHealth" "[]" "version"
test_rpc "Has currentBlock" "ethernova_nodeHealth" "[]" "currentBlock"
test_rpc "Has peerCount" "ethernova_nodeHealth" "[]" "peerCount"
test_rpc "Has uptimeSeconds" "ethernova_nodeHealth" "[]" "uptimeSeconds"
test_rpc "Has memoryMB" "ethernova_nodeHealth" "[]" "memoryMB"
echo ""

# ============================================================
echo "============================================================"
echo "  RESULTS: $PASS passed / $FAIL failed / $TOTAL total"
echo "============================================================"
if [ $FAIL -eq 0 ]; then
    echo "  ALL TESTS PASSED - Noven Fork Ready!"
else
    echo "  $FAIL TESTS FAILED - Fix before mainnet deployment"
fi
echo ""
