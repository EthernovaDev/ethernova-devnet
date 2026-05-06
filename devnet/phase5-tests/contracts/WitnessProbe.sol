// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title WitnessProbe — exercise 0x2F precompile via real STATICCALL.
/// @notice EOA → contract.staticCall() forces real EVM-level STATICCALL
///         frame on the inner call to 0x2F. eth_call from EOA aja gak
///         cukup buat exercise EIP-214 path — itu cuma read-only frame
///         dari JSON-RPC, bukan EVM STATICCALL flag.
contract WitnessProbe {
    address public constant STATE_WITNESS_PRECOMPILE = 0x000000000000000000000000000000000000002F;

    /// Selector 0x01 verifyStateWitness via STATICCALL — should succeed.
    function probeVerifyStatic(bytes calldata payload) external view returns (bool ok, bytes memory ret) {
        (ok, ret) = STATE_WITNESS_PRECOMPILE.staticcall(abi.encodePacked(uint8(0x01), payload));
    }

    /// Selector 0x02 restoreState via STATICCALL — MUST FAIL (EIP-214).
    function probeRestoreStatic(bytes calldata payload) external view returns (bool ok, bytes memory ret) {
        (ok, ret) = STATE_WITNESS_PRECOMPILE.staticcall(abi.encodePacked(uint8(0x02), payload));
    }

    /// Selector 0x03 getCurrentTier via STATICCALL — should succeed.
    function probeGetTierStatic(address target) external view returns (bool ok, bytes memory ret) {
        bytes memory input = abi.encodePacked(uint8(0x03), bytes12(0), target);
        (ok, ret) = STATE_WITNESS_PRECOMPILE.staticcall(input);
    }

    /// Selector 0x02 via regular CALL — should succeed kalau proof valid.
    function callRestore(bytes calldata payload) external returns (bool ok, bytes memory ret) {
        (ok, ret) = STATE_WITNESS_PRECOMPILE.call(abi.encodePacked(uint8(0x02), payload));
    }

    function callGetTier(address target) external returns (bool ok, bytes memory ret) {
        bytes memory input = abi.encodePacked(uint8(0x03), bytes12(0), target);
        (ok, ret) = STATE_WITNESS_PRECOMPILE.call(input);
    }
}
