package txfilter

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	tmTypes "github.com/tendermint/tendermint/types"
	"math/big"
)

func PPCIsBlocked(from, to common.Address, balance *big.Int, txDataBytes []byte) (err error) {
	if EthPosTable == nil {
		return ErrPosTableNotCreate
	}
	EthPosTable.Mtx.RLock()
	defer EthPosTable.Mtx.RUnlock()
	if !EthPosTable.InitFlag {
		return ErrPosTableNotInit
	}
	posItem, exist := EthPosTable.PosItemMap[from]
	if exist {
		if IsUnlockTx(to) {
			return EthPosTable.CanRemovePosItem()
		} else if IsLockTx(to) {
			//wenbin add, ignore balance judge
			//tmpInt := big.NewInt(0)
			//currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
			currentSlots := int64(10)
			if posItem.Slots >= currentSlots {
				return fmt.Errorf("signer %X already bonded at height %d ,balance has not increased", from, posItem.Height)
			}

			ppcTxData, err := PPCUnMarshalTxData(txDataBytes)
			if err != nil {
				return err
			}
			pubKey, err := tmTypes.PB2TM.PubKey(ppcTxData.PubKey)
			if err != nil {
				return err
			}
			tmAddress := pubKey.Address().String()
			if posItem.TmAddress != tmAddress {
				return fmt.Errorf("signer %X bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
			}
			if posItem.BlsKeyString != ppcTxData.BlsKeyString {
				return fmt.Errorf("signer %X bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, ppcTxData.BlsKeyString)
			}
			_, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
			if !exist {
				panic(fmt.Sprintf("tmAddress %v already be bonded by %X, but not found in TmAddressToSignerMap", tmAddress, from))
			}
			_, exist = EthPosTable.BlsKeyStringToSignerMap[ppcTxData.BlsKeyString]
			if !exist {
				panic(fmt.Sprintf("blsKeyString %v already be bonded by %X, but not found in TmAddressToSignerMap", ppcTxData.BlsKeyString, from))
			}

			return nil
		} else {
			return fmt.Errorf("signer %X bonded at height %d ", from, posItem.Height)
		}
	} else {
		posItem, exist := EthPosTable.UnbondPosItemMap[from]
		if exist {
			if IsUnlockTx(to) {
				return fmt.Errorf("signer %X already unbonded at height %d", from, posItem.Height)
			} else if IsLockTx(to) {
				return fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			} else {
				return fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			}
		} else {
			if IsUnlockTx(to) {
				return fmt.Errorf("signer %X has not bonded ", from)
			} else if IsLockTx(to) { //first lock
				//wenbin add, ignore balance judge
				//tmpInt := big.NewInt(0)
				//currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
				currentSlots := int64(10)
				if 1 > currentSlots {
					fmt.Printf("signer %X doesn't have one slot of money", from)
					return fmt.Errorf("signer %X doesn't have one slot of money", from)
				}

				ppcTxData, err := PPCUnMarshalTxData(txDataBytes)
				if err != nil {
					return err
				}
				if len(ppcTxData.BlsKeyString) == 0 {
					return fmt.Errorf("len(ppcTxData.BlsKeyString)==0, wrong BlsKeyString? %v", ppcTxData.BlsKeyString)
				}
				pubKey, err := tmTypes.PB2TM.PubKey(ppcTxData.PubKey)
				if err != nil {
					return err
				}
				tmAddress := pubKey.Address().String()
				if len(tmAddress) == 0 {
					return fmt.Errorf("len(tmAddress)==0, wrong pubKey? %v", ppcTxData.PubKey)
				}
				signer, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
				if exist {
					return fmt.Errorf("tmAddress %v already be bonded by %X", tmAddress, signer)
				}
				signer, exist = EthPosTable.BlsKeyStringToSignerMap[ppcTxData.BlsKeyString]
				if exist {
					return fmt.Errorf("blsKeyString %v already be bonded by %X", ppcTxData.BlsKeyString, signer)
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

func PPCDoFilter(from, to common.Address, balance *big.Int, txDataBytes []byte, height int64) (isBetTx bool, err error) {
	if EthPosTable == nil { //should not happen
		fmt.Printf("PosTable has not created yet")
		return false, ErrPosTableNotCreate
	}
	EthPosTable.Mtx.Lock()
	defer EthPosTable.Mtx.Unlock()
	if !EthPosTable.InitFlag { //should not happen
		fmt.Printf("PosTable has not init yet")
		return false, ErrPosTableNotInit
	}
	posItem, exist := EthPosTable.PosItemMap[from]
	if exist {
		if IsUnlockTx(to) {
			return true, EthPosTable.RemovePosItem(from, height, false)
		} else if IsLockTx(to) { //relock
			//wenbin add, ignore balance judge
			//tmpInt := big.NewInt(0)
			//currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
			currentSlots := int64(10)
			if posItem.Slots >= currentSlots {
				fmt.Printf("signer %X already bonded at height %d , balance has not increased", from, posItem.Height)
				return true, fmt.Errorf("signer %X already bonded at height %d , balance has not increased", from, posItem.Height)
			}

			ppcTxData, err := PPCUnMarshalTxData(txDataBytes)
			if err != nil {
				return true, err
			}
			pubKey, err := tmTypes.PB2TM.PubKey(ppcTxData.PubKey)
			if err != nil {
				return true, err
			}
			tmAddress := pubKey.Address().String()
			if posItem.TmAddress != tmAddress {
				fmt.Printf("signer %X bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
				return true, fmt.Errorf("signer %X bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
			}
			if posItem.BlsKeyString != ppcTxData.BlsKeyString {
				fmt.Printf("signer %X bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, ppcTxData.BlsKeyString)
				return true, fmt.Errorf("signer %X bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, ppcTxData.BlsKeyString)
			}
			_, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
			if !exist {
				panic(fmt.Sprintf("tmAddress %v already be bonded by %X, but not found in TmAddressToSignerMap", tmAddress, from))
			}
			_, exist = EthPosTable.BlsKeyStringToSignerMap[ppcTxData.BlsKeyString]
			if !exist {
				panic(fmt.Sprintf("blsKeyString %v already be bonded by %X, but not found in TmAddressToSignerMap", ppcTxData.BlsKeyString, from))
			}
			posItem.Slots = currentSlots
			EthPosTable.UpsertPosItem(from, posItem)
			return true, nil
		} else {
			return false, fmt.Errorf("signer %X bonded at height %d ", from, posItem.Height)
		}
	} else {
		posItem, exist := EthPosTable.UnbondPosItemMap[from]
		if exist {
			if IsUnlockTx(to) {
				fmt.Printf("signer %X already unbonded at height %d", from, posItem.Height)
				return true, fmt.Errorf("signer %X already unbonded at height %d", from, posItem.Height)
			} else if IsLockTx(to) {
				fmt.Printf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
				return true, fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			} else {
				fmt.Printf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
				return false, fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			}
		} else {
			if IsUnlockTx(to) {
				fmt.Printf("signer %X has not bonded ", from)
				return true, fmt.Errorf("signer %X has not bonded ", from)
			} else if IsLockTx(to) { //first lock
				ppcTxData, err := PPCUnMarshalTxData(txDataBytes)
				if err != nil {
					return true, err
				}
				if len(ppcTxData.BlsKeyString) == 0 {
					return true, fmt.Errorf("len(ppcTxData.BlsKeyString)==0, wrong BlsKeyString? %v", ppcTxData.BlsKeyString)
				}
				pubKey, err := tmTypes.PB2TM.PubKey(ppcTxData.PubKey)
				if err != nil {
					return true, err
				}
				tmAddress := pubKey.Address().String()
				if len(tmAddress) == 0 {
					return true, fmt.Errorf("len(tmAddress)==0, wrong pubKey? %v", ppcTxData.PubKey)
				}
				signer, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
				if exist {
					fmt.Printf("tmAddress %v already be bonded by %X", tmAddress, signer)
					return true, fmt.Errorf("tmAddress %v already be bonded by %X", tmAddress, signer)
				}
				signer, exist = EthPosTable.BlsKeyStringToSignerMap[ppcTxData.BlsKeyString]
				if exist {
					fmt.Printf("blsKeyString %v already be bonded by %X", ppcTxData.BlsKeyString, signer)
					return true, fmt.Errorf("blsKeyString %v already be bonded by %X", ppcTxData.BlsKeyString, signer)
				}

				//wenbin add, ignore balance judge
				//tmpInt := big.NewInt(0)
				//currentSlots := tmpInt.Div(balance, EthPosTable.Threshold).Int64()
				currentSlots := int64(10)
				if 1 > currentSlots {
					fmt.Printf("signer %X doesn't have one slot of money", from)
					return true, fmt.Errorf("signer %X doesn't have one slot of money", from)
				}
				return true, EthPosTable.UpsertPosItem(from, NewPosItem(height, currentSlots, ppcTxData.PubKey, tmAddress, ppcTxData.BlsKeyString, common.HexToAddress(ppcTxData.Beneficiary)))
			}
		}
	}

	_, isSpecificAddress := w[from]
	if isSpecificAddress {
		return false, fmt.Errorf("Specific Account %v should be blocked ", from)
	}
	return false, nil
}
