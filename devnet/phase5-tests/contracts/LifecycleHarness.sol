// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title LifecycleHarness — Phase 5 test target with deterministic storage.
/// @notice 5 storage slots untuk multi-slot tier transition test.
///         Tidak ada randomness, tidak ada time-based logic.
contract LifecycleHarness {
    uint256 public slot0;
    uint256 public slot1;
    uint256 public slot2;
    uint256 public slot3;
    uint256 public slot4;
    uint256 public touches;

    address public immutable owner;
    uint256 public immutable createdAt;

    constructor() {
        owner = msg.sender;
        createdAt = block.number;
    }

    function set(uint8 idx, uint256 v) external {
        if (idx == 0) slot0 = v;
        else if (idx == 1) slot1 = v;
        else if (idx == 2) slot2 = v;
        else if (idx == 3) slot3 = v;
        else if (idx == 4) slot4 = v;
        else revert("idx OOR");
        touches++;
    }

    function read(uint8 idx) external view returns (uint256) {
        if (idx == 0) return slot0;
        if (idx == 1) return slot1;
        if (idx == 2) return slot2;
        if (idx == 3) return slot3;
        if (idx == 4) return slot4;
        revert("idx OOR");
    }

    function readAll() external view returns (uint256, uint256, uint256, uint256, uint256) {
        return (slot0, slot1, slot2, slot3, slot4);
    }

    /// Multi-SLOAD untuk gas profiling per-tier.
    function multiRead(uint256 count) external view returns (uint256 sum) {
        for (uint256 i = 0; i < count; i++) {
            sum += slot0;
        }
    }

    /// Touch all slots — promote contract back to Active.
    function touchAll() external {
        slot0 = slot0 + 1;
        slot1 = slot1 + 1;
        slot2 = slot2 + 1;
        slot3 = slot3 + 1;
        slot4 = slot4 + 1;
        touches++;
    }
}
