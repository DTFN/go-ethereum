package txfilter

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	tmTypes "github.com/tendermint/tendermint/types"
	"math/big"
	"github.com/ethereum/go-ethereum/crypto"
	"bytes"
)

var (
	SendToLock   = common.HexToAddress("0x7777777777777777777777777777777777777777")
	SendToUnlock = common.HexToAddress("0x8888888888888888888888888888888888888888")
	w            = make(map[common.Address]bool)

	ErrPosTableNotCreate = errors.New("PosTable has not created yet")
	ErrPosTableNotInit   = errors.New("PosTable has not init yet")

	UpgradeHeight int64
)

func init() {
	w[SendToLock] = true
	w[SendToUnlock] = true
	w[SendToAuth] = true
	w[SendToRelay] = true
	w[SendToMint] = true
}

func IsMintBlocked(from common.Address) (err error) {
	if from != Bigguy {
		fmt.Printf("Not big guy %X account tries to mint, block it \n", from)
		return fmt.Errorf("Not big guy %X account tries to mint, block it \n", from)
	}
	return nil
}

func IsBetBlocked(from common.Address, to *common.Address, balance *big.Int, txDataBytes []byte, height int64) (err error) {
	posItem, exist := EthPosTable.PosItemMap[from]
	if exist {
		if to != nil && IsUnlockTx(*to) {
			return EthPosTable.CanRemovePosItem()
		} else if to != nil && IsLockTx(*to) { //relock
			currentSlots := int64(0)
			if height < UpgradeHeight {
				tmpInt := big.NewInt(0)
				currentSlots = tmpInt.Div(balance, EthPosTable.Threshold).Int64()
			} else {
				return fmt.Errorf("signer %X is already in PosTable. and it is after ppchain upgrade, no need to relock", from)
				/*authItem, found := EthAuthTable.AuthItemMap[from]
				if !found {
					return fmt.Errorf("signer %X authItem not found in AuthTable", from)
				}
				if height < authItem.StartHeight {
					return fmt.Errorf("signer %X too early to join PosTable, current height %v, authItem startHeight %v ", from, height, authItem.StartHeight)
				}
				if height > authItem.EndHeight {
					return fmt.Errorf("signer %X too late to join PosTable, current height %v, authItem endHeight %v, expired ", from, height, authItem.StartHeight)
				}
				currentSlots = int64(10)*/
			}
			if posItem.Slots >= currentSlots {
				return fmt.Errorf("signer %X already bonded at height %d ,balance has not increased", from, posItem.Height)
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
				return fmt.Errorf("signer %X bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
			}
			if posItem.BlsKeyString != txData.BlsKeyString {
				return fmt.Errorf("signer %X bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, txData.BlsKeyString)
			}
			_, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
			if !exist {
				panic(fmt.Sprintf("tmAddress %v already be bonded by %X, but not found in TmAddressToSignerMap", tmAddress, from))
			}
			_, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
			if !exist {
				panic(fmt.Sprintf("blsKeyString %v already be bonded by %X, but not found in TmAddressToSignerMap", txData.BlsKeyString, from))
			}

			return nil
		} else {
			return fmt.Errorf("signer %X bonded at height %d ", from, posItem.Height)
		}
	} else {
		posItem, exist := EthPosTable.UnbondPosItemMap[from]
		if exist {
			if to != nil && IsUnlockTx(*to) {
				return fmt.Errorf("signer %X already unbonded at height %d", from, posItem.Height)
			} else if to != nil && IsLockTx(*to) {
				return fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			} else {
				return fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			}
		} else {
			if to != nil && IsUnlockTx(*to) {
				return fmt.Errorf("signer %X has not bonded ", from)
			} else if to != nil && IsLockTx(*to) { //first lock
				currentSlots := int64(0)
				if height < UpgradeHeight {
					tmpInt := big.NewInt(0)
					currentSlots = tmpInt.Div(balance, EthPosTable.Threshold).Int64()
				} else {
					authItem, found := EthAuthTable.AuthItemMap[from]
					if !found {
						return fmt.Errorf("signer %X authItem not found in AuthTable", from)
					}
					if height < authItem.StartHeight {
						return fmt.Errorf("signer %X too early to join PosTable, current height %v, authItem startHeight %v ", from, height, authItem.StartHeight)
					}
					if height > authItem.EndHeight {
						return fmt.Errorf("signer %X too late to join PosTable, current height %v, authItem endHeight %v, expired ", from, height, authItem.StartHeight)
					}
					if tmHash := crypto.Keccak256(txDataBytes); !bytes.Equal(tmHash, authItem.ApprovedTxDataHash) {
						fmt.Printf("signer %X tmData hash %X not match with authed hash %X \n", from, tmHash, authItem.ApprovedTxDataHash)
						return fmt.Errorf("signer %X tmData hash %X not match with authed hash %X", from, tmHash, authItem.ApprovedTxDataHash)
					}
					currentSlots = int64(10)
				}
				if 1 > currentSlots {
					fmt.Println("test1.1")
					fmt.Println(currentSlots)
					fmt.Printf("signer %X doesn't have one slot of money", from)
					return fmt.Errorf("signer %X doesn't have one slot of money", from)
				}
				txData, err := UnMarshalTxData(txDataBytes)
				if err != nil {
					return err
				}
				if len(txData.BlsKeyString) == 0 {
					return fmt.Errorf("len(txData.BlsKeyString)==0, wrong BlsKeyString? %v", txData.BlsKeyString)
				}
				pubKey, err := tmTypes.PB2TM.PubKey(txData.PubKey)
				if err != nil {
					return err
				}
				tmAddress := pubKey.Address().String()
				if len(tmAddress) == 0 {
					return fmt.Errorf("len(tmAddress)==0, wrong pubKey? %v", txData.PubKey)
				}
				signer, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
				if exist {
					return fmt.Errorf("tmAddress %v already be bonded by %X", tmAddress, signer)
				}
				signer, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
				if exist {
					return fmt.Errorf("blsKeyString %v already be bonded by %X", txData.BlsKeyString, signer)
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

func DoBetHandle(from common.Address, to *common.Address, balance *big.Int, txDataBytes []byte, height int64) (isBetTx bool, err error) {
	posItem, exist := EthPosTable.PosItemMap[from]
	if exist {
		if to != nil && IsUnlockTx(*to) {
			return true, EthPosTable.RemovePosItem(from, height, false)
		} else if to != nil && IsLockTx(*to) { //relock
			currentSlots := int64(0)
			if height < UpgradeHeight {
				tmpInt := big.NewInt(0)
				currentSlots = tmpInt.Div(balance, EthPosTable.Threshold).Int64()
			} else {
				return true, fmt.Errorf("signer %X is already in PosTable. and it is after ppchain upgrade, no need to relock", from)
				//currentSlots = int64(10)
			}
			if posItem.Slots >= currentSlots {
				fmt.Printf("signer %X already bonded at height %d , balance has not increased", from, posItem.Height)
				return true, fmt.Errorf("signer %X already bonded at height %d , balance has not increased", from, posItem.Height)
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
				fmt.Printf("signer %X bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
				return true, fmt.Errorf("signer %X bonded tmAddress %v not matched with current tmAddress %v ", from, posItem.TmAddress, tmAddress)
			}
			if posItem.BlsKeyString != txData.BlsKeyString {
				fmt.Printf("signer %X bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, txData.BlsKeyString)
				return true, fmt.Errorf("signer %X bonded BlsKeyString %v not matched with current BlsKeyString %v ", from, posItem.BlsKeyString, txData.BlsKeyString)
			}
			_, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
			if !exist {
				panic(fmt.Sprintf("tmAddress %v already be bonded by %X, but not found in TmAddressToSignerMap", tmAddress, from))
			}
			_, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
			if !exist {
				panic(fmt.Sprintf("blsKeyString %v already be bonded by %X, but not found in TmAddressToSignerMap", txData.BlsKeyString, from))
			}
			EthPosTable.UpdatePosItem(from, currentSlots)
			return true, nil
		} else {
			return false, fmt.Errorf("signer %X bonded at height %d ", from, posItem.Height)
		}
	} else {
		posItem, exist := EthPosTable.UnbondPosItemMap[from]
		if exist {
			if to != nil && IsUnlockTx(*to) {
				fmt.Printf("signer %X already unbonded at height %d", from, posItem.Height)
				return true, fmt.Errorf("signer %X already unbonded at height %d", from, posItem.Height)
			} else if to != nil && IsLockTx(*to) {
				fmt.Printf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
				return true, fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			} else {
				fmt.Printf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
				return false, fmt.Errorf("signer %X unbonded at height %d . will available at height %d", from, posItem.Height, (posItem.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks)
			}
		} else {
			if to != nil && IsUnlockTx(*to) {
				fmt.Printf("signer %X has not bonded ", from)
				return true, fmt.Errorf("signer %X has not bonded ", from)
			} else if to != nil && IsLockTx(*to) { //first lock
				txData, err := UnMarshalTxData(txDataBytes)
				if err != nil {
					return true, err
				}
				if len(txData.BlsKeyString) == 0 {
					return true, fmt.Errorf("len(txData.BlsKeyString)==0, wrong BlsKeyString? %v", txData.BlsKeyString)
				}
				pubKey, err := tmTypes.PB2TM.PubKey(txData.PubKey)
				if err != nil {
					return true, err
				}
				tmAddress := pubKey.Address().String()
				if len(tmAddress) == 0 {
					return true, fmt.Errorf("len(tmAddress)==0, wrong pubKey? %v", txData.PubKey)
				}
				signer, exist := EthPosTable.TmAddressToSignerMap[tmAddress]
				if exist {
					fmt.Printf("tmAddress %v already be bonded by %X", tmAddress, signer)
					return true, fmt.Errorf("tmAddress %v already be bonded by %X", tmAddress, signer)
				}
				signer, exist = EthPosTable.BlsKeyStringToSignerMap[txData.BlsKeyString]
				if exist {
					fmt.Printf("blsKeyString %v already be bonded by %X", txData.BlsKeyString, signer)
					return true, fmt.Errorf("blsKeyString %v already be bonded by %X", txData.BlsKeyString, signer)
				}
				currentSlots := int64(0)
				if height < UpgradeHeight {
					tmpInt := big.NewInt(0)
					currentSlots = tmpInt.Div(balance, EthPosTable.Threshold).Int64()
				} else {
					authItem, found := EthAuthTable.AuthItemMap[from]
					if !found {
						return true, fmt.Errorf("signer %X authItem not found in AuthTable", from)
					}
					if height < authItem.StartHeight {
						return true, fmt.Errorf("signer %X too early to join PosTable, current height %v, authItem startHeight %v ", from, height, authItem.StartHeight)
					}
					if height > authItem.EndHeight {
						return true, fmt.Errorf("signer %X too late to join PosTable, current height %v, authItem endHeight %v, expired ", from, height, authItem.StartHeight)
					}
					currentSlots = int64(10)
					delete(EthAuthTable.AuthItemMap, from)	//delete the auth item when it joins PosTable
				}
				if 1 > currentSlots {
					fmt.Println("test1.1")
					fmt.Println(currentSlots)
					fmt.Printf("signer %X doesn't have one slot of money", from)
					return true, fmt.Errorf("signer %X doesn't have one slot of money", from)
				}
				return true, EthPosTable.InsertPosItem(from, NewPosItem(height, currentSlots, txData.PubKey, tmAddress, txData.BlsKeyString, common.HexToAddress(txData.Beneficiary)))  //this should succeed, otherwise authItem has to rollback
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
	return bytes.Equal(to.Bytes(), SendToLock.Bytes())
}

func IsUnlockTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToUnlock.Bytes())
}
