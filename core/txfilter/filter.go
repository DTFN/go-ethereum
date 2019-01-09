package txfilter

import (
	"github.com/ethereum/go-ethereum/common"
	tmTypes "github.com/tendermint/tendermint/types"
	"strings"
	"fmt"
	"math/big"
)

var (
	sendToLock   = common.HexToAddress("0x7777777777777777777777777777777777777777")
	sendToUnlock = common.HexToAddress("0x8888888888888888888888888888888888888888")
	w            = make(map[common.Address]bool)
)

func init() {
	w[sendToLock] = true
	w[sendToUnlock] = true
}

func IsBlocked(from, to common.Address, balance *big.Int, txDataBytes []byte) (err error) {
	EthPosTable.Mtx.RLock()
	defer EthPosTable.Mtx.RUnlock()
	if !EthPosTable.InitFlag {
		return fmt.Errorf("PosTable has not init yet")
	}
	posItem, exist := EthPosTable.PosItemMap[from]
	if exist {
		if IsUnlockTx(to) {
			return nil
		} else if IsLockTx(to) {
			tmpInt := big.NewInt(0)
			currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
			if posItem.Slots >= currentSlots {
				return fmt.Errorf("signer %v already bonded at height %d ", from, posItem.Height)
			}

			txData, err := UnMarshalTxData(txDataBytes)
			if err != nil {
				return err
			}
			pubKey, err := tmTypes.PB2TM.PubKey(txData.PubKey)
			if err != nil {
				return err
			}
			tmAddress := pubKey.Address().String()
			if posItem.TmAddress != tmAddress {
				return fmt.Errorf("signer %v bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
			}
			if posItem.BlsKeyString != txData.BlsKeyString {
				return fmt.Errorf("signer %v bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, txData.BlsKeyString)
			}
			_, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
			if !exist {
				panic(fmt.Sprintf("tmAddress %v already be bonded by %v, but not found in TmAddressToSignerMap", tmAddress, from))
			}
			_, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
			if !exist {
				panic(fmt.Sprintf("blsKeyString %v already be bonded by %v, but not found in TmAddressToSignerMap", txData.BlsKeyString, from))
			}

			return nil
		} else {
			return fmt.Errorf("signer %v bonded at height %d ", from, posItem.Height)
		}
	} else {
		posItem, exist := EthPosTable.UnbondPosItemMap[from]
		if exist {
			if IsUnlockTx(to) {
				return fmt.Errorf("signer %v already unbonded at height %d", from, posItem.Height)
			} else if IsLockTx(to) {
				return fmt.Errorf("signer %v unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			} else {
				return fmt.Errorf("signer %v unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			}
		} else {
			if IsUnlockTx(to) {
				return fmt.Errorf("signer %v has not bonded ", from)
			} else if IsLockTx(to) {
				txData, err := UnMarshalTxData(txDataBytes)
				if err != nil {
					return err
				}
				pubKey, err := tmTypes.PB2TM.PubKey(txData.PubKey)
				if err != nil {
					return err
				}
				tmAddress := pubKey.Address().String()
				signer, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
				if exist {
					return fmt.Errorf("tmAddress %v already be bonded by %v", tmAddress, signer)
				}
				signer, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
				if exist {
					return fmt.Errorf("blsKeyString %v already be bonded by %v", txData.BlsKeyString, signer)
				}
			}
		}
	}

	_, isSpecificAddress := w[from]
	if isSpecificAddress {
		return fmt.Errorf("Specific Account %v should be blocked ", from)
	}
	return nil
}

func DoFilter(from, to common.Address, balance *big.Int, txDataBytes []byte, height int64) (isBetTx bool, err error) {
	EthPosTable.Mtx.RLock()
	defer EthPosTable.Mtx.RUnlock()
	if !EthPosTable.InitFlag {
		return false, fmt.Errorf("PosTable has not init yet")
	}
	posItem, exist := EthPosTable.PosItemMap[from]
	if exist {
		if IsUnlockTx(to) {
			EthPosTable.RemovePosItem(from, height)
			return true, nil
		} else if IsLockTx(to) { //relock
			tmpInt := big.NewInt(0)
			currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
			if posItem.Slots >= currentSlots {
				return true, fmt.Errorf("signer %v already bonded at height %d ", from, posItem.Height)
			}

			txData, err := UnMarshalTxData(txDataBytes)
			if err != nil {
				return true, err
			}
			pubKey, err := tmTypes.PB2TM.PubKey(txData.PubKey)
			if err != nil {
				return true, err
			}
			tmAddress := pubKey.Address().String()
			if posItem.TmAddress != tmAddress {
				return true, fmt.Errorf("signer %v bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
			}
			if posItem.BlsKeyString != txData.BlsKeyString {
				return true, fmt.Errorf("signer %v bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, txData.BlsKeyString)
			}
			_, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
			if !exist {
				panic(fmt.Sprintf("tmAddress %v already be bonded by %v, but not found in TmAddressToSignerMap", tmAddress, from))
			}
			_, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
			if !exist {
				panic(fmt.Sprintf("blsKeyString %v already be bonded by %v, but not found in TmAddressToSignerMap", txData.BlsKeyString, from))
			}
			posItem.Slots = currentSlots
			EthPosTable.UpsertPosItem(from, posItem)
			return true, nil
		} else {
			return false, fmt.Errorf("signer %v bonded at height %d ", from, posItem.Height)
		}
	} else {
		posItem, exist := EthPosTable.UnbondPosItemMap[from]
		if exist {
			if IsUnlockTx(to) {
				return true, fmt.Errorf("signer %v already unbonded at height %d", from, posItem.Height)
			} else if IsLockTx(to) {
				return true, fmt.Errorf("signer %v unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			} else {
				return false, fmt.Errorf("signer %v unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			}
		} else {
			if IsUnlockTx(to) {
				return true, fmt.Errorf("signer %v has not bonded ", from)
			} else if IsLockTx(to) { //first lock
				txData, err := UnMarshalTxData(txDataBytes)
				if err != nil {
					return true, err
				}
				pubKey, err := tmTypes.PB2TM.PubKey(txData.PubKey)
				if err != nil {
					return true, err
				}
				tmAddress := pubKey.Address().String()
				signer, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
				if exist {
					return true, fmt.Errorf("tmAddress %v already be bonded by %v", tmAddress, signer)
				}
				signer, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
				if exist {
					return true, fmt.Errorf("blsKeyString %v already be bonded by %v", txData.BlsKeyString, signer)
				}
				tmpInt := big.NewInt(0)
				currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
				EthPosTable.UpsertPosItem(from, NewPosItem(height, currentSlots, txData.PubKey, tmAddress, txData.BlsKeyString, common.HexToAddress(txData.Beneficiary)))
				return true, nil
			}
		}
	}

	_, isSpecificAddress := w[from]
	if isSpecificAddress {
		return false, fmt.Errorf("Specific Account %v should be blocked ", from)
	}
	return false, nil
}

func IsBetTx(to common.Address) bool {
	return IsLockTx(to) || IsUnlockTx(to)
}

func IsLockTx(to common.Address) bool {
	return strings.EqualFold(to.String(), sendToLock.String())
}

func IsUnlockTx(to common.Address) bool {
	return strings.EqualFold(to.String(), sendToUnlock.String())
}