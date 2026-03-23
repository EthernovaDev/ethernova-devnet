// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title NovaDEX (Simple Swap Pool)
 * @notice Minimal constant-product DEX for Ethernova Devnet testing.
 * Storage-heavy (multiple SSTORE per swap) - should trigger adaptive gas penalty.
 */
contract NovaDEX {
    address public tokenA;
    address public tokenB;
    uint256 public reserveA;
    uint256 public reserveB;
    uint256 public totalLiquidity;
    mapping(address => uint256) public liquidity;

    uint256 public swapCount;
    uint256 public totalVolumeA;
    uint256 public totalVolumeB;

    event AddLiquidity(address indexed provider, uint256 amountA, uint256 amountB, uint256 liquidity);
    event RemoveLiquidity(address indexed provider, uint256 amountA, uint256 amountB);
    event Swap(address indexed trader, address tokenIn, uint256 amountIn, uint256 amountOut);

    constructor(address _tokenA, address _tokenB) {
        tokenA = _tokenA;
        tokenB = _tokenB;
    }

    function addLiquidity(uint256 amountA, uint256 amountB) public returns (uint256) {
        // Transfer tokens in (simplified - assumes approval)
        require(IERC20(tokenA).transferFrom(msg.sender, address(this), amountA), "transfer A failed");
        require(IERC20(tokenB).transferFrom(msg.sender, address(this), amountB), "transfer B failed");

        uint256 liq;
        if (totalLiquidity == 0) {
            liq = sqrt(amountA * amountB);
        } else {
            liq = min(amountA * totalLiquidity / reserveA, amountB * totalLiquidity / reserveB);
        }

        liquidity[msg.sender] += liq;
        totalLiquidity += liq;
        reserveA += amountA;
        reserveB += amountB;

        emit AddLiquidity(msg.sender, amountA, amountB, liq);
        return liq;
    }

    function swapAForB(uint256 amountIn) public returns (uint256) {
        require(amountIn > 0 && reserveA > 0 && reserveB > 0, "invalid swap");
        require(IERC20(tokenA).transferFrom(msg.sender, address(this), amountIn), "transfer failed");

        uint256 amountOut = (amountIn * 997 * reserveB) / (reserveA * 1000 + amountIn * 997);
        require(amountOut > 0, "insufficient output");

        reserveA += amountIn;
        reserveB -= amountOut;
        swapCount++;
        totalVolumeA += amountIn;

        require(IERC20(tokenB).transfer(msg.sender, amountOut), "output transfer failed");
        emit Swap(msg.sender, tokenA, amountIn, amountOut);
        return amountOut;
    }

    function swapBForA(uint256 amountIn) public returns (uint256) {
        require(amountIn > 0 && reserveA > 0 && reserveB > 0, "invalid swap");
        require(IERC20(tokenB).transferFrom(msg.sender, address(this), amountIn), "transfer failed");

        uint256 amountOut = (amountIn * 997 * reserveA) / (reserveB * 1000 + amountIn * 997);
        require(amountOut > 0, "insufficient output");

        reserveB += amountIn;
        reserveA -= amountOut;
        swapCount++;
        totalVolumeB += amountIn;

        require(IERC20(tokenA).transfer(msg.sender, amountOut), "output transfer failed");
        emit Swap(msg.sender, tokenB, amountIn, amountOut);
        return amountOut;
    }

    function getPrice() public view returns (uint256) {
        if (reserveA == 0) return 0;
        return (reserveB * 1e18) / reserveA;
    }

    function sqrt(uint256 x) internal pure returns (uint256) {
        if (x == 0) return 0;
        uint256 z = (x + 1) / 2;
        uint256 y = x;
        while (z < y) { y = z; z = (x / z + z) / 2; }
        return y;
    }

    function min(uint256 a, uint256 b) internal pure returns (uint256) {
        return a < b ? a : b;
    }
}

interface IERC20 {
    function transfer(address to, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
}
