// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title NovaNFT (ERC-721 Simplified)
 * @notice Minimal ERC-721 for Ethernova Devnet testing.
 * Mixed operations (storage writes for ownership + events) - neutral gas pattern.
 */
contract NovaNFT {
    string public constant name = "Nova NFT";
    string public constant symbol = "NNFT";

    uint256 public totalSupply;
    mapping(uint256 => address) public ownerOf;
    mapping(address => uint256) public balanceOf;
    mapping(uint256 => address) public getApproved;
    mapping(uint256 => string) public tokenURI;

    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
    event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId);

    function mint(address to, string memory uri) public returns (uint256) {
        totalSupply++;
        uint256 tokenId = totalSupply;
        ownerOf[tokenId] = to;
        balanceOf[to]++;
        tokenURI[tokenId] = uri;
        emit Transfer(address(0), to, tokenId);
        return tokenId;
    }

    function transferFrom(address from, address to, uint256 tokenId) public {
        require(ownerOf[tokenId] == from, "not owner");
        require(msg.sender == from || msg.sender == getApproved[tokenId], "not authorized");
        balanceOf[from]--;
        balanceOf[to]++;
        ownerOf[tokenId] = to;
        getApproved[tokenId] = address(0);
        emit Transfer(from, to, tokenId);
    }

    function approve(address to, uint256 tokenId) public {
        require(ownerOf[tokenId] == msg.sender, "not owner");
        getApproved[tokenId] = to;
        emit Approval(msg.sender, to, tokenId);
    }
}
