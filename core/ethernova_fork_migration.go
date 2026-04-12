package core

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/ethernova"
	"github.com/ethereum/go-ethereum/params/types/coregeth"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
)

func ethernovaPatchConfigIfNeeded(cfg ctypes.ChainConfigurator, head uint64) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	chainID := cfg.GetChainID()
	if chainID == nil || chainID.Uint64() != ethernova.NewChainID {
		return false, nil
	}

	updated := false
	var errs []string

	forkBlock := ethernova.EVMCompatibilityForkBlock
	missing, mismatched, err := EthernovaForkStatus(cfg, forkBlock)
	if err != nil {
		errs = append(errs, err.Error())
	} else if len(mismatched) > 0 {
		errs = append(errs, fmt.Sprintf("unexpected fork block values (%s); expected %d", strings.Join(mismatched, ", "), forkBlock))
	} else if missing {
		if head >= forkBlock {
			errs = append(errs, fmt.Sprintf("UPGRADE REQUIRED: missing Constantinople/Petersburg/Istanbul fork blocks; head=%d fork=%d", head, forkBlock))
		} else {
			updatedForks, err := ethernovaApplyForks(cfg, forkBlock)
			if err != nil {
				errs = append(errs, err.Error())
			} else if updatedForks {
				log.Warn("Ethernova chain config upgraded in-place", "fork_block", forkBlock, "head", head, "feature", "evm-compat")
				updated = true
			}
		}
	}

	eip658Block := ethernova.EIP658ForkBlock
	missing, mismatched, err = EthernovaEIP658Status(cfg, eip658Block)
	if err != nil {
		errs = append(errs, err.Error())
	} else if len(mismatched) > 0 {
		errs = append(errs, fmt.Sprintf("unexpected EIP-658 fork block values (%s); expected %d", strings.Join(mismatched, ", "), eip658Block))
	} else if missing {
		if head >= eip658Block {
			errs = append(errs, fmt.Sprintf("UPGRADE REQUIRED: missing EIP-658 fork block; head=%d fork=%d", head, eip658Block))
		} else {
			updated658, err := ethernovaApplyEIP658(cfg, eip658Block)
			if err != nil {
				errs = append(errs, err.Error())
			} else if updated658 {
				log.Warn("Ethernova chain config upgraded in-place", "fork_block", eip658Block, "head", head, "feature", "eip658")
				updated = true
			}
		}
	}

	megaBlock := ethernova.MegaForkBlock
	missingFields, mismatched, err := EthernovaMegaForkStatus(cfg, megaBlock)
	if err != nil {
		errs = append(errs, err.Error())
	} else if len(mismatched) > 0 {
		errs = append(errs, fmt.Sprintf("unexpected mega fork values (%s); expected %d", strings.Join(mismatched, ", "), megaBlock))
	} else if len(missingFields) > 0 {
		if head >= megaBlock {
			errs = append(errs, fmt.Sprintf("UPGRADE REQUIRED: missing mega fork fields (%s); head=%d fork=%d", strings.Join(missingFields, ", "), head, megaBlock))
		} else {
			updatedMega, err := ethernovaApplyMegaFork(cfg, megaBlock)
			if err != nil {
				errs = append(errs, err.Error())
			} else if updatedMega {
				log.Warn("Ethernova chain config upgraded in-place", "fork_block", megaBlock, "head", head, "feature", "mega-fork")
				updated = true
			}
		}
	}

	if len(errs) > 0 {
		return updated, fmt.Errorf("ethernova config errors: %s", strings.Join(errs, "; "))
	}
	return updated, nil
}

func EthernovaForkStatus(cfg ctypes.ChainConfigurator, forkBlock uint64) (missing bool, mismatched []string, err error) {
	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
	if !ok {
		return false, nil, fmt.Errorf("unsupported chain config type for ethernova: %T", cfg)
	}
	checkBig := func(name string, val *big.Int) {
		if val == nil {
			missing = true
			return
		}
		if val.Uint64() != forkBlock {
			mismatched = append(mismatched, fmt.Sprintf("%s=%d", name, val.Uint64()))
		}
	}
	checkBig("constantinopleBlock", cg.ConstantinopleBlock)
	checkBig("petersburgBlock", cg.PetersburgBlock)
	checkBig("istanbulBlock", cg.IstanbulBlock)

	check := func(name string, val *uint64) {
		if val == nil {
			return
		}
		if *val != forkBlock {
			mismatched = append(mismatched, fmt.Sprintf("%s=%d", name, *val))
		}
	}
	check("eip145", cfg.GetEIP145Transition())
	check("eip1014", cfg.GetEIP1014Transition())
	check("eip1052", cfg.GetEIP1052Transition())
	check("eip1283", cfg.GetEIP1283Transition())
	check("petersburg", cfg.GetEIP1283DisableTransition())
	check("eip152", cfg.GetEIP152Transition())
	check("eip1108", cfg.GetEIP1108Transition())
	check("eip1344", cfg.GetEIP1344Transition())
	check("eip1884", cfg.GetEIP1884Transition())
	check("eip2028", cfg.GetEIP2028Transition())
	check("eip2200", cfg.GetEIP2200Transition())
	return missing, mismatched, nil
}

func EthernovaEIP658Status(cfg ctypes.ChainConfigurator, forkBlock uint64) (missing bool, mismatched []string, err error) {
	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
	if !ok {
		return false, nil, fmt.Errorf("unsupported chain config type for ethernova: %T", cfg)
	}
	if cg.EIP658FBlock == nil {
		return true, nil, nil
	}
	if cg.EIP658FBlock.Uint64() != forkBlock {
		mismatched = append(mismatched, fmt.Sprintf("eip658FBlock=%d", cg.EIP658FBlock.Uint64()))
	}
	return false, mismatched, nil
}

func ethernovaApplyForks(cfg ctypes.ChainConfigurator, forkBlock uint64) (bool, error) {
	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
	if !ok {
		return false, fmt.Errorf("unsupported chain config type for ethernova: %T", cfg)
	}
	updated := false
	if cg.ConstantinopleBlock == nil {
		cg.ConstantinopleBlock = new(big.Int).SetUint64(forkBlock)
		updated = true
	}
	if cg.PetersburgBlock == nil {
		cg.PetersburgBlock = new(big.Int).SetUint64(forkBlock)
		updated = true
	}
	if cg.IstanbulBlock == nil {
		cg.IstanbulBlock = new(big.Int).SetUint64(forkBlock)
		updated = true
	}
	return updated, nil
}

func ethernovaApplyEIP658(cfg ctypes.ChainConfigurator, forkBlock uint64) (bool, error) {
	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
	if !ok {
		return false, fmt.Errorf("unsupported chain config type for ethernova: %T", cfg)
	}
	if cg.EIP658FBlock != nil {
		return false, nil
	}
	cg.EIP658FBlock = new(big.Int).SetUint64(forkBlock)
	return true, nil
}

func EthernovaMegaForkStatus(cfg ctypes.ChainConfigurator, forkBlock uint64) (missing []string, mismatched []string, err error) {
	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported chain config type for ethernova: %T", cfg)
	}
	checkBig := func(name string, val *big.Int) {
		if val == nil {
			missing = append(missing, name)
			return
		}
		if val.Uint64() != forkBlock {
			mismatched = append(mismatched, fmt.Sprintf("%s=%d", name, val.Uint64()))
		}
	}
	checkBig("eip2FBlock", cg.EIP2FBlock)
	checkBig("eip7FBlock", cg.EIP7FBlock)
	checkBig("eip150Block", cg.EIP150Block)
	checkBig("eip160FBlock", cg.EIP160FBlock)
	checkBig("eip161FBlock", cg.EIP161FBlock)
	checkBig("eip170FBlock", cg.EIP170FBlock)
	checkBig("eip100FBlock", cg.EIP100FBlock)
	checkBig("eip140FBlock", cg.EIP140FBlock)
	checkBig("eip198FBlock", cg.EIP198FBlock)
	checkBig("eip211FBlock", cg.EIP211FBlock)
	checkBig("eip212FBlock", cg.EIP212FBlock)
	checkBig("eip213FBlock", cg.EIP213FBlock)
	checkBig("eip214FBlock", cg.EIP214FBlock)
	checkBig("eip1706FBlock", cg.EIP1706FBlock)
	return missing, mismatched, nil
}

func ethernovaApplyMegaFork(cfg ctypes.ChainConfigurator, forkBlock uint64) (bool, error) {
	cg, ok := cfg.(*coregeth.CoreGethChainConfig)
	if !ok {
		return false, fmt.Errorf("unsupported chain config type for ethernova: %T", cfg)
	}
	updated := false
	setIfNil := func(val **big.Int) {
		if *val == nil {
			*val = new(big.Int).SetUint64(forkBlock)
			updated = true
		}
	}
	setIfNil(&cg.EIP2FBlock)
	setIfNil(&cg.EIP7FBlock)
	setIfNil(&cg.EIP150Block)
	setIfNil(&cg.EIP160FBlock)
	setIfNil(&cg.EIP161FBlock)
	setIfNil(&cg.EIP170FBlock)
	setIfNil(&cg.EIP100FBlock)
	setIfNil(&cg.EIP140FBlock)
	setIfNil(&cg.EIP198FBlock)
	setIfNil(&cg.EIP211FBlock)
	setIfNil(&cg.EIP212FBlock)
	setIfNil(&cg.EIP213FBlock)
	setIfNil(&cg.EIP214FBlock)
	setIfNil(&cg.EIP1706FBlock)
	return updated, nil
}
