// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title ContentRefTest — NIP-0004 Phase 3 Solidity harness
/// @notice Exercises the novaContentRegistry precompile at address 0x2B.
///         The precompile was originally drafted at 0x2A in NIP-0004, but
///         0x2A is already occupied by the Phase 2 novaDeferredQueue in
///         this codebase. The final Phase 3 spec (shipped with this commit)
///         documents 0x2B as the canonical address.
///
///         Selectors (first byte of input):
///           0x01 createContentRef
///           0x02 getContentRef
///           0x03 isValid
///           0x04 listContentRefsByOwner
///           0x05 getContentRefCount
contract ContentRefTest {
    address constant CONTENT_REGISTRY = address(0x2B);

    event Created(bytes32 indexed id, bytes32 contentHash, uint256 size);
    event Checked(bytes32 indexed id, bool valid);

    /// @notice Create a ContentRef via the precompile.
    /// @param contentHash keccak256 commitment of the referenced off-chain blob
    /// @param size        declared size of the off-chain blob, in bytes
    /// @param contentType MIME-style type bytes (<= 64 bytes)
    /// @param proof       availability proof bytes (<= 256 bytes)
    /// @param rentPrepay  rent prepay in wei, must be >= MinRentPrepayWei
    /// @param expiryBlock hard block-number expiry, 0 = never (rent-only)
    /// @return id the 32-byte ContentRef ID
    function createContentRef(
        bytes32 contentHash,
        uint256 size,
        bytes calldata contentType,
        bytes calldata proof,
        uint256 rentPrepay,
        uint256 expiryBlock
    ) external returns (bytes32 id) {
        // Head layout (bytes): [1][32][32][32][32][32][32]
        //   selector, contentHash, size, ctLen, proofLen, rentPrepay, expiryBlock
        // Tail: contentType bytes, then proof bytes.
        bytes memory input = abi.encodePacked(
            bytes1(0x01),
            contentHash,
            size,
            uint256(contentType.length),
            uint256(proof.length),
            rentPrepay,
            expiryBlock,
            contentType,
            proof
        );

        (bool ok, bytes memory ret) = CONTENT_REGISTRY.call(input);
        require(ok, "createContentRef: precompile call failed");
        require(ret.length == 32, "createContentRef: bad return length");

        id = bytes32(ret);
        emit Created(id, contentHash, size);
    }

    /// @notice Get a ContentRef. Returns the raw RLP body concatenated with
    ///         a single 0x01/0x00 validity flag byte.
    function getContentRef(bytes32 id) external view returns (bytes memory) {
        bytes memory input = abi.encodePacked(bytes1(0x02), id);
        (bool ok, bytes memory ret) = CONTENT_REGISTRY.staticcall(input);
        require(ok, "getContentRef: precompile call failed");
        return ret;
    }

    /// @notice Returns true if the ContentRef is live and has enough rent
    ///         for at least one more epoch.
    function isValid(bytes32 id) external view returns (bool) {
        bytes memory input = abi.encodePacked(bytes1(0x03), id);
        (bool ok, bytes memory ret) = CONTENT_REGISTRY.staticcall(input);
        require(ok, "isValid: precompile call failed");
        require(ret.length == 32, "isValid: bad return length");
        return uint256(bytes32(ret)) != 0;
    }

    /// @notice Non-view wrapper for isValid — useful for tests that want
    ///         the result in a receipt log rather than a return value.
    function isValidTx(bytes32 id) external returns (bool ok_) {
        ok_ = this.isValid(id);
        emit Checked(id, ok_);
    }

    /// @notice List ContentRef IDs owned by an address.
    /// @return count number of IDs returned, and the IDs themselves packed.
    function listByOwner(
        address owner,
        uint256 offset,
        uint256 limit
    ) external view returns (uint256 count, bytes32[] memory ids) {
        // listContentRefsByOwner accepts 20-byte owner + 32-byte offset +
        // 32-byte limit (= 84 bytes after the selector). Solidity gives us
        // a 20-byte address natively, which is exactly what we want.
        bytes memory input = abi.encodePacked(bytes1(0x04), owner, offset, limit);
        (bool ok, bytes memory ret) = CONTENT_REGISTRY.staticcall(input);
        require(ok, "listByOwner: precompile call failed");
        require(ret.length >= 32, "listByOwner: bad return length");

        assembly {
            count := mload(add(ret, 32))
        }
        require(ret.length == 32 + count * 32, "listByOwner: length mismatch");

        ids = new bytes32[](count);
        for (uint256 i = 0; i < count; i++) {
            bytes32 idv;
            uint256 off = 32 + 32 + i * 32; // +32 for length-prefix of `ret`, +32 for count word
            assembly {
                idv := mload(add(ret, off))
            }
            ids[i] = idv;
        }
    }

    /// @notice Global live ContentRef count.
    function getCount() external view returns (uint256) {
        bytes memory input = abi.encodePacked(bytes1(0x05));
        (bool ok, bytes memory ret) = CONTENT_REGISTRY.staticcall(input);
        require(ok, "getCount: precompile call failed");
        require(ret.length == 32, "getCount: bad return length");
        return uint256(bytes32(ret));
    }

    /// @notice Convenience: create many ContentRefs in one transaction.
    ///         Useful for populating the registry in stress tests.
    function createBatch(
        bytes32[] calldata contentHashes,
        uint256[] calldata sizes,
        uint256 rentPrepayEach,
        uint256 expiryBlock
    ) external returns (bytes32[] memory ids) {
        require(contentHashes.length == sizes.length, "length mismatch");
        ids = new bytes32[](contentHashes.length);
        bytes memory emptyCT = new bytes(0);
        bytes memory emptyProof = new bytes(0);
        for (uint256 i = 0; i < contentHashes.length; i++) {
            ids[i] = this.createContentRef(
                contentHashes[i],
                sizes[i],
                emptyCT,
                emptyProof,
                rentPrepayEach,
                expiryBlock
            );
        }
    }
}
