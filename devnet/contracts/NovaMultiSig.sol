// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title NovaMultiSig
 * @notice Multi-signature wallet for Ethernova Devnet testing.
 * Moderate storage usage (mappings for confirmations) - neutral to slight penalty.
 */
contract NovaMultiSig {
    address[] public owners;
    uint256 public required;
    uint256 public transactionCount;

    struct Transaction {
        address to;
        uint256 value;
        bytes data;
        bool executed;
        uint256 confirmations;
    }

    mapping(uint256 => Transaction) public transactions;
    mapping(uint256 => mapping(address => bool)) public confirmations;
    mapping(address => bool) public isOwner;

    event Submit(uint256 indexed txId, address indexed owner, address to, uint256 value);
    event Confirm(uint256 indexed txId, address indexed owner);
    event Revoke(uint256 indexed txId, address indexed owner);
    event Execute(uint256 indexed txId);
    event Deposit(address indexed sender, uint256 value);

    modifier onlyOwner() {
        require(isOwner[msg.sender], "not owner");
        _;
    }

    constructor(address[] memory _owners, uint256 _required) {
        require(_owners.length > 0, "no owners");
        require(_required > 0 && _required <= _owners.length, "invalid required");
        for (uint256 i = 0; i < _owners.length; i++) {
            require(_owners[i] != address(0), "zero address");
            require(!isOwner[_owners[i]], "duplicate");
            isOwner[_owners[i]] = true;
            owners.push(_owners[i]);
        }
        required = _required;
    }

    receive() external payable {
        emit Deposit(msg.sender, msg.value);
    }

    function submit(address to, uint256 value, bytes memory data) public onlyOwner returns (uint256) {
        uint256 txId = transactionCount++;
        transactions[txId] = Transaction({
            to: to,
            value: value,
            data: data,
            executed: false,
            confirmations: 0
        });
        emit Submit(txId, msg.sender, to, value);
        return txId;
    }

    function confirm(uint256 txId) public onlyOwner {
        require(!transactions[txId].executed, "already executed");
        require(!confirmations[txId][msg.sender], "already confirmed");
        confirmations[txId][msg.sender] = true;
        transactions[txId].confirmations++;
        emit Confirm(txId, msg.sender);
    }

    function execute(uint256 txId) public onlyOwner {
        Transaction storage t = transactions[txId];
        require(!t.executed, "already executed");
        require(t.confirmations >= required, "not enough confirmations");
        t.executed = true;
        (bool ok, ) = t.to.call{value: t.value}(t.data);
        require(ok, "tx failed");
        emit Execute(txId);
    }

    function revoke(uint256 txId) public onlyOwner {
        require(!transactions[txId].executed, "already executed");
        require(confirmations[txId][msg.sender], "not confirmed");
        confirmations[txId][msg.sender] = false;
        transactions[txId].confirmations--;
        emit Revoke(txId, msg.sender);
    }

    function getOwners() public view returns (address[] memory) {
        return owners;
    }
}
