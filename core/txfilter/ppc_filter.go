package txfilter

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"strings"
)

var (
	SendToMint         = common.HexToAddress("0x1111111111111111111111111111111111111111")
	SendToAuth         = common.HexToAddress("0x2222222222222222222222222222222222222222")
	RelayTxFromClient  = common.HexToAddress("0x3333333333333333333333333333333333333333")
	RelayTxFromRelayer = common.HexToAddress("0x4444444444444444444444444444444444444444")

	ErrAuthTableNotCreate = errors.New("AuthTable has not created yet")
)

var (
	PPChainAdmin common.Address
	Bigguy       common.Address
)

func IsAuthBlocked(from common.Address, txDataBytes []byte, height int64, sim bool) (err error) {
	var authTable *AuthTable
	var posTable *PosTable
	if sim {
		authTable = EthAuthTableCopy
		posTable = CurrentPosTable
	} else {
		authTable = EthAuthTable
		posTable = NextPosTable
	}
	if authTable == nil {
		return ErrAuthTableNotCreate
	}
	if !bytes.Equal(from.Bytes(), PPChainAdmin.Bytes()) {
		fmt.Printf("not admin %X sent an auth tx \n", from)
		return fmt.Errorf("not admin %X sent an auth tx \n", from)
	}
	var ppcdata AuthData
	ppcdata, err = UnMarshalAuthTxData(txDataBytes)
	if err != nil {
		fmt.Printf("admin %X sent an auth tx with illegal format \n", from)
		return fmt.Errorf("admin %X sent an auth tx with illegal format \n", from)
	}
	fmt.Printf("-----receive ppcdata. %v", ppcdata)
	if ppcdata.OperationType == "add" {
		authItem, exist := authTable.AuthItemMap[ppcdata.PermittedAddress]
		if exist {
			fmt.Printf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
			return fmt.Errorf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
		} else {
			_, exist := posTable.PosItemMap[ppcdata.PermittedAddress]
			if exist {
				fmt.Printf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
				return fmt.Errorf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
			}
			if len(ppcdata.TmAddress) == 0 {
				if AppVersion >= 5 {
					return fmt.Errorf("ppcdata.TmAddress is required after appversion 5 ")
				}
			} else {
				ppcdata.TmAddress = strings.ToUpper(ppcdata.TmAddress)
				for _, tmAddr := range authTable.ExtendAuthTable.SignerToTmAddressMap {
					if tmAddr == ppcdata.TmAddress {
						return fmt.Errorf("TmAddressToSignerMap already contains this addr %v ", ppcdata.TmAddress)
					}
				}
			}
			return
		}
	} else if ppcdata.OperationType == "remove" {
		if _, exist := authTable.AuthItemMap[ppcdata.PermittedAddress]; !exist {
			return fmt.Errorf("removePermitItem in auth check, permittedAddr %X not exists", ppcdata.PermittedAddress)
		}
		return
	} else if ppcdata.OperationType == "kickout" {
		if _, ok := posTable.PosItemMap[ppcdata.PermittedAddress]; !ok {
			fmt.Printf("admin %X wants to kickout %X, but it is not in the PosTable \n", from, ppcdata.PermittedAddress)
			return fmt.Errorf("admin %X wants to kickout %X, but it is not in the PosTable \n", from, ppcdata.PermittedAddress)
		}
		return
	}
	return fmt.Errorf("admin %X sent an unrecognized OperationType %v \n", from, ppcdata.OperationType)
}

func DoAuthHandle(from common.Address, txDataBytes []byte, height int64, sim bool) (err error) {
	var authTable *AuthTable
	var posTable *PosTable
	if sim {
		authTable = EthAuthTableCopy.Copy()
		posTable = CurrentPosTable.Copy()
	} else {
		authTable = EthAuthTable
		posTable = NextPosTable
	}
	if authTable == nil {
		return ErrAuthTableNotCreate
	}
	if !bytes.Equal(from.Bytes(), PPChainAdmin.Bytes()) {
		fmt.Printf("not admin %X sent an auth tx \n", from)
		return fmt.Errorf("not admin %X sent an auth tx \n", from)
	}
	var ppcdata AuthData
	ppcdata, err = UnMarshalAuthTxData(txDataBytes)
	if err != nil {
		fmt.Printf("admin %X sent an auth tx with illegal format \n", from)
		return fmt.Errorf("admin %X sent an auth tx with illegal format \n", from)
	}
	fmt.Printf("-----handle ppcdata. %v", ppcdata)
	if ppcdata.OperationType == "add" {
		authItem, exist := authTable.AuthItemMap[ppcdata.PermittedAddress]
		if exist {
			fmt.Printf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
			return fmt.Errorf("addr %X already permitted at height %d , auth range [%v,%v] auth hash %X", ppcdata.PermittedAddress, authItem.PermitHeight, authItem.StartHeight, authItem.EndHeight, authItem.ApprovedTxDataHash)
		}
		_, exist = posTable.PosItemMap[ppcdata.PermittedAddress]
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
		if AppVersion >= 5 {
			if len(ppcdata.TmAddress) == 0 {
				return fmt.Errorf("ppcdata.TmAddress is required after appversion 5 ")
			}
			ppcdata.TmAddress = strings.ToUpper(ppcdata.TmAddress)
			for _, tmAddr := range authTable.ExtendAuthTable.SignerToTmAddressMap {
				if tmAddr == ppcdata.TmAddress {
					return fmt.Errorf("TmAddressToSignerMap already contains this addr %v ", ppcdata.TmAddress)
				}
			}
			authTable.ThisBlockChangedMap[ppcdata.TmAddress] = true
			if err = authTable.InsertAuthItemWithTmAddr(ppcdata.PermittedAddress, authItem, ppcdata.TmAddress); err != nil {
				panic(err) //we have checked this before, this should not happen
			}
		} else {
			if err = authTable.InsertAuthItem(ppcdata.PermittedAddress, authItem); err != nil {
				panic(err) //we have checked this before, this should not happen
			}
		}
		return
	} else if ppcdata.OperationType == "remove" {
		_, exist := authTable.AuthItemMap[ppcdata.PermittedAddress]
		if !exist {
			fmt.Printf("addr %X has not been permitted ", ppcdata.PermittedAddress)
			return fmt.Errorf("addr %X has not been permitted ", ppcdata.PermittedAddress)
		}
		if AppVersion >= 5 {
			tmAddress, found := authTable.ExtendAuthTable.SignerToTmAddressMap[ppcdata.PermittedAddress]
			if !found {
				fmt.Printf("SignerToTmAddressMap does not find %v ! ", ppcdata.PermittedAddress)
				if len(ppcdata.TmAddress) == 0 {
					return fmt.Errorf("SignerToTmAddressMap does not find %v ! and ppcdata.TmAddress is empty", ppcdata.PermittedAddress)
				}
				tmAddress = strings.ToUpper(ppcdata.TmAddress)
			}
			authTable.ThisBlockChangedMap[tmAddress] = false
		}
		if err = authTable.DeleteAuthItem(ppcdata.PermittedAddress); err != nil {
			panic(err)
		}

		return
	} else if ppcdata.OperationType == "kickout" {
		if err := posTable.RemovePosItem(ppcdata.PermittedAddress, height, false); err != nil {
			return err
		}
		return
	}
	return fmt.Errorf("admin %X sent an unrecognized OperationType %v \n", from, ppcdata.OperationType)
}

func IsMintTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToMint.Bytes())
}

func IsAuthTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToAuth.Bytes())
}

func IsRelayTxFromClient(to common.Address) bool {
	return bytes.Equal(to.Bytes(), RelayTxFromClient.Bytes())
}

func IsRelayTxFromRelayer(to common.Address) bool {
	return bytes.Equal(to.Bytes(), RelayTxFromRelayer.Bytes())
}
