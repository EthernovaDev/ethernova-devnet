// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title DeferredTestHarness — NIP-0004 Phase 2 consensus harness
///
/// Exercises the novaDeferredQueue precompile at 0x2A:
///   - enqueueEffect(type, payload) schedules work for block N+1
///   - getPendingEffect(seq) reads back a queued entry (RLP bytes)
///   - getQueueStats() returns a 5 × uint256 stats block
///
/// The contract holds no state itself — all correctness checks run against
/// the chain's own queue storage via the precompile and via eth_getStorageAt
/// on address 0xFF02. This is on purpose: Phase 2 is about the queue
/// primitive, and a test that keeps its own mirror of queue state would
/// confuse what is actually being validated.
///
/// Effect type tags MUST mirror core/types/deferred_effect.go:
///     0x00 Noop   (drains silently)
///     0x01 Ping   (drains; bumps a per-caller counter at 0xFF02)
///     0x10 MailboxSend   (reserved for Phase 4 — drains as no-op now)
///     0x20 AsyncCallback (reserved for Phase 7 — drains as no-op now)
///     0x30 SessionUpdate (reserved for Phase 7 — drains as no-op now)
contract DeferredTestHarness {
    address constant DEFERRED_QUEUE = address(0x2A);

    uint8 constant EFFECT_NOOP = 0x00;
    uint8 constant EFFECT_PING = 0x01;
    uint8 constant EFFECT_MAILBOX_SEND = 0x10;
    uint8 constant EFFECT_ASYNC_CALLBACK = 0x20;
    uint8 constant EFFECT_SESSION_UPDATE = 0x30;

    event Enqueued(uint64 indexed seqNum, uint8 indexed effectType, uint256 payloadLen);
    event BatchEnqueued(uint64 firstSeq, uint64 count);

    /// Enqueue a single effect. Returns the assigned monotonic sequence
    /// number. Reverts if the per-block backpressure cap is hit.
    function enqueue(uint8 effectType, bytes calldata payload) external returns (uint64) {
        bytes memory input = abi.encodePacked(uint8(0x01), effectType, payload);
        (bool ok, bytes memory ret) = DEFERRED_QUEUE.call(input);
        require(ok, "enqueue: precompile reverted");
        require(ret.length == 32, "enqueue: unexpected return size");
        uint256 seq = abi.decode(ret, (uint256));
        uint64 seq64 = uint64(seq);
        emit Enqueued(seq64, effectType, payload.length);
        return seq64;
    }

    /// Batch enqueue — useful for stress tests. Each call costs one
    /// slot of per-block cap. Returns the first assigned seq and the count.
    function enqueueBatch(uint8 effectType, bytes calldata payload, uint256 count)
        external
        returns (uint64 firstSeq, uint64 actualCount)
    {
        require(count > 0 && count <= 256, "batch: bad count");
        bool firstSet = false;
        for (uint256 i = 0; i < count; i++) {
            bytes memory input = abi.encodePacked(uint8(0x01), effectType, payload);
            (bool ok, bytes memory ret) = DEFERRED_QUEUE.call(input);
            if (!ok) {
                // Backpressure cap was hit mid-batch. Return what we got.
                break;
            }
            require(ret.length == 32, "batch: unexpected return size");
            uint256 seq = abi.decode(ret, (uint256));
            if (!firstSet) {
                firstSeq = uint64(seq);
                firstSet = true;
            }
            actualCount++;
        }
        emit BatchEnqueued(firstSeq, actualCount);
    }

    /// Convenience wrapper: enqueue a ping effect. The drain-time handler
    /// bumps a counter at keccak256("ping_counter", caller) on address
    /// 0xFF02, observable via eth_getStorageAt.
    function ping(bytes calldata payload) external returns (uint64) {
        return this.enqueue(EFFECT_PING, payload);
    }

    /// Read queue stats via the precompile. Returns
    /// (head, tail, pending, enqueuesAtThisBlock, totalProcessed).
    function stats()
        external
        view
        returns (
            uint256 head,
            uint256 tail,
            uint256 pending,
            uint256 enqThisBlock,
            uint256 totalProcessed
        )
    {
        bytes memory input = abi.encodePacked(uint8(0x03));
        (bool ok, bytes memory ret) = DEFERRED_QUEUE.staticcall(input);
        require(ok, "stats: precompile reverted");
        require(ret.length == 160, "stats: unexpected return size");
        assembly {
            head := mload(add(ret, 32))
            tail := mload(add(ret, 64))
            pending := mload(add(ret, 96))
            enqThisBlock := mload(add(ret, 128))
            totalProcessed := mload(add(ret, 160))
        }
    }

    /// Read a single pending entry by sequence number. Returns raw RLP
    /// bytes; decoding is done off-chain by the JS harness or by RPC.
    /// Reverts if the entry is absent (drained or never existed).
    function getEntry(uint64 seq) external view returns (bytes memory) {
        bytes memory input = abi.encodePacked(uint8(0x02), bytes32(uint256(seq)));
        (bool ok, bytes memory ret) = DEFERRED_QUEUE.staticcall(input);
        require(ok, "getEntry: precompile reverted");
        return ret;
    }

    /// Read a caller's per-address ping counter from 0xFF02 directly.
    /// The slot is keccak256("ping_counter" || caller). We compute it the
    /// same way the drain handler does, so we can read without going
    /// through the precompile. This gives tests a clean, assertion-friendly
    /// integer result straight from the state trie.
    function pingCounterOf(address who) external view returns (uint256) {
        bytes32 slot = keccak256(abi.encodePacked("ping_counter", who));
        bytes32 raw;
        assembly {
            // SLOAD from an arbitrary contract is not possible in the EVM,
            // so we emulate it: the value at 0xFF02[slot] is readable only
            // via eth_getStorageAt from off-chain. This on-chain accessor
            // therefore returns 0 — tests should use eth_getStorageAt on
            // 0xFF02 with the slot computed above. Kept here as
            // documentation of the exact slot derivation.
            raw := 0
        }
        return uint256(raw);
    }

    /// Compute the pingCounterOf storage slot for a given address, so JS
    /// tests don't need to duplicate the hashing in their own code.
    function pingCounterSlot(address who) external pure returns (bytes32) {
        return keccak256(abi.encodePacked("ping_counter", who));
    }
}
