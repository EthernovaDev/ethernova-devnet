// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title TestProfiler - Contract to generate diverse opcode usage for profiling
contract TestProfiler {
    uint256 public counter;
    uint256 public totalGasUsed;
    mapping(address => uint256) public balances;
    uint256[] public values;

    event CounterUpdated(uint256 newValue, uint256 gasUsed);
    event StorageWrite(address indexed user, uint256 value);

    // Heavy arithmetic - uses ADD, MUL, DIV, MOD, EXP
    function heavyMath(uint256 iterations) external returns (uint256 result) {
        uint256 startGas = gasleft();
        result = 1;
        for (uint256 i = 1; i <= iterations; i++) {
            result = (result * i + i) % (type(uint256).max / 2);
            result = result / (i + 1) + result % (i + 1);
        }
        counter += 1;
        uint256 used = startGas - gasleft();
        totalGasUsed += used;
        emit CounterUpdated(counter, used);
    }

    // Storage heavy - uses SSTORE, SLOAD
    function storageHeavy(uint256 iterations) external {
        for (uint256 i = 0; i < iterations; i++) {
            balances[msg.sender] += 1;
            values.push(i);
        }
        emit StorageWrite(msg.sender, iterations);
    }

    // Memory heavy - uses MSTORE, MLOAD, MSIZE
    function memoryHeavy(uint256 size) external pure returns (bytes32) {
        bytes memory data = new bytes(size);
        for (uint256 i = 0; i < size && i < 1000; i++) {
            data[i] = bytes1(uint8(i % 256));
        }
        return keccak256(data);
    }

    // Call chain - uses CALL, STATICCALL
    function callChain(uint256 depth) external returns (uint256) {
        if (depth == 0) return counter;
        counter += 1;
        return this.callChain(depth - 1);
    }

    // Predictable pure computation (candidate for optimization)
    function pureCompute(uint256 a, uint256 b) external pure returns (uint256) {
        uint256 result = a;
        for (uint256 i = 0; i < 100; i++) {
            result = (result ^ b) + i;
            result = (result >> 1) | (result << 255);
        }
        return result;
    }

    // Simple counter increment (baseline)
    function increment() external {
        counter += 1;
    }

    // Read-only (no gas optimization needed)
    function getState() external view returns (uint256, uint256, uint256) {
        return (counter, totalGasUsed, values.length);
    }
}
