package txfilter

import (
	"errors"
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"fmt"
)

var (
	SendToMint  = common.HexToAddress("0x1111111111111111111111111111111111111111")
	SendToAuth  = common.HexToAddress("0x2222222222222222222222222222222222222222")
	SendToRelay = common.HexToAddress("0x3333333333333333333333333333333333333333")

	ErrAuthTableNotCreate = errors.New("AuthTable has not created yet")
)

var (
	PPChainAdmin common.Address
	Bigguy       common.Address
)

func IsAuthBlocked(from common.Address, txDataBytes []byte, height int64) (err error) {
	var ppcdata AuthData
	ppcdata, err = UnMarshalAuthTxData(txDataBytes)
	if err != nil {
		fmt.Printf("admin %X sent an auth tx with illegal format \n", from)
		return fmt.Errorf("admin %X sent an auth tx with illegal format \n", from)
	}
	if EthPosTable == nil {
		return ErrPosTableNotCreate
	}
	if EthAuthTable == nil {
		return ErrAuthTableNotCreate
	}
	EthPosTable.Mtx.RLock()
	defer EthPosTable.Mtx.RUnlock()
	if !EthPosTable.InitFlag {
		return ErrPosTableNotInit
	}
	fmt.Printf("=======receive ppcdata. %v", ppcdata)
	if ppcdata.OperationType == "add" {
		authItem, exist := EthAuthTable.AuthItemMap[ppcdata.PermittedAddress]
		if exist {
			fmt.Printf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
			return fmt.Errorf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
		} else {
			_, exist := EthPosTable.PosItemMap[ppcdata.PermittedAddress]
			if exist {
				fmt.Printf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
				return fmt.Errorf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
			}
			return
		}
	} else if ppcdata.OperationType == "remove" {
		if _, exist := EthAuthTable.AuthItemMap[ppcdata.PermittedAddress]; !exist {
			return fmt.Errorf("removePermitItem in auth check, permittedAddr %X not exists", ppcdata.PermittedAddress)
		}
		return
	} else if ppcdata.OperationType == "kickout" {
		if _, ok := EthPosTable.PosItemMap[ppcdata.PermittedAddress]; !ok {
			fmt.Printf("admin %X wants to kickout %X, but it is not in the PosTable \n", from, ppcdata.PermittedAddress)
			return fmt.Errorf("admin %X wants to kickout %X, but it is not in the PosTable \n", from, ppcdata.PermittedAddress)
		}
		EthAuthTable.DeleteAuthItem(ppcdata.PermittedAddress) //it possibly has auth item
		return EthPosTable.RemovePosItem(ppcdata.PermittedAddress, height, false)
	}
	return fmt.Errorf("admin %X sent an unrecognized OperationType %v \n", from, ppcdata.OperationType)
}

func DoAuthHandle(from common.Address, txDataBytes []byte, height int64) (err error) {
	var ppcdata AuthData
	ppcdata, err = UnMarshalAuthTxData(txDataBytes)
	if err != nil {
		fmt.Printf("admin %X sent an auth tx with illegal format \n", from)
		return fmt.Errorf("admin %X sent an auth tx with illegal format \n", from)
	}
	if EthPosTable == nil {
		return ErrPosTableNotCreate
	}
	if EthAuthTable == nil {
		return ErrAuthTableNotCreate
	}
	EthPosTable.Mtx.Lock()
	defer EthPosTable.Mtx.Unlock()
	if !EthPosTable.InitFlag {
		return ErrPosTableNotInit
	}
	fmt.Printf("=======ppcdata. %v", ppcdata)
	if ppcdata.OperationType == "add" {
		authItem, exist := EthAuthTable.AuthItemMap[ppcdata.PermittedAddress]
		if exist {
			fmt.Printf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
			return fmt.Errorf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
		} else {
			_, exist := EthPosTable.PosItemMap[ppcdata.PermittedAddress]
			if exist {
				fmt.Printf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
				return fmt.Errorf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
			}
			authItem = &AuthItem{
				ApprovedTxDataHash: ppcdata.ApprovedTxDataHash,
				StartHeight:        ppcdata.StartBlockHeight,
				EndHeight:          ppcdata.EndBlockHeight,
				PermitHeight:       height,
			}
			err = EthAuthTable.InsertAuthItem(ppcdata.PermittedAddress, authItem)
			fmt.Printf("=======after InsertPermitItem. %v", EthAuthTable)
			return
		}
	} else if ppcdata.OperationType == "remove" {
		err = EthAuthTable.DeleteAuthItem(ppcdata.PermittedAddress)
		return
	} else if ppcdata.OperationType == "kickout" {
		EthPosTable.Mtx.Lock()
		defer EthPosTable.Mtx.Unlock()
		return EthPosTable.RemovePosItem(ppcdata.PermittedAddress, height, false)
	}
	return fmt.Errorf("admin %X sent an unrecognized OperationType %v \n", from, ppcdata.OperationType)
}

func IsMintTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToMint.Bytes())
}

func IsAuthTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToAuth.Bytes())
}

func IsRelayTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToRelay.Bytes())
}
