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
			if len(ppcdata.TmAddress) != 0 {
				ppcdata.TmAddress = strings.ToUpper(ppcdata.TmAddress)
				if _, ok := authTable.RevertAuthTable.TmAddressToSignerMap[ppcdata.TmAddress]; ok {
					return fmt.Errorf("tmAddr %X already exists in AuthTable", ppcdata.TmAddress)
				}
			} else {
				for _, signer := range authTable.RevertAuthTable.TmAddressToSignerMap {
					if signer == ppcdata.PermittedAddress {
						return fmt.Errorf("signer %X has authed some tmAddr, please send! ", ppcdata.TmAddress)
					}
				}
			}
			return
		}
	} else if ppcdata.OperationType == "remove" {
		if len(ppcdata.TmAddress) != 0 {
			ppcdata.TmAddress = strings.ToUpper(ppcdata.TmAddress)
			if signer, exist := authTable.RevertAuthTable.TmAddressToSignerMap[ppcdata.TmAddress]; exist {
				if signer != ppcdata.PermittedAddress {
					return fmt.Errorf("ppcdata.PermittedAddress %X did not auth this tmAddr %v", ppcdata.PermittedAddress, ppcdata.TmAddress)
				}
			} else {
				return fmt.Errorf("ppcdata.TmAddress %X does not exist ", ppcdata.TmAddress)
			}
		} else {
			for _, signer := range authTable.RevertAuthTable.TmAddressToSignerMap {
				if signer == ppcdata.PermittedAddress {
					return fmt.Errorf("ppcdata.PermittedAddress %X has authed some tmAddr, please send! ", ppcdata.PermittedAddress)
				}
			}
		}
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
		} else {
			_, exist := posTable.PosItemMap[ppcdata.PermittedAddress]
			if exist {
				fmt.Printf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
				return fmt.Errorf("addr %X is already in PosTable, no need to auth it ", ppcdata.PermittedAddress)
			}
			if AppVersion >= 5 {
				if len(ppcdata.TmAddress) == 0 {
					return fmt.Errorf("ppcdata.TmAddress is required after appversion 5 ")
				}
				ppcdata.TmAddress = strings.ToUpper(ppcdata.TmAddress)
				if err = authTable.InsertTmAddrSignerPair(ppcdata.TmAddress, ppcdata.PermittedAddress); err != nil {
					return fmt.Errorf("tmAddr %X already exists in AuthTable", ppcdata.TmAddress)
				}
				authTable.ThisBlockChangedMap[ppcdata.TmAddress] = true
			}

			authItem = &AuthItem{
				ApprovedTxDataHash: ppcdata.ApprovedTxDataHash,
				StartHeight:        ppcdata.StartBlockHeight,
				EndHeight:          ppcdata.EndBlockHeight,
				PermitHeight:       height,
			}
			if err = authTable.InsertAuthItem(ppcdata.PermittedAddress, authItem); err != nil {
				panic(err) //we have checked this before, this should not happen
			}

			return
		}
	} else if ppcdata.OperationType == "remove" {
		if AppVersion >= 5 {
			if len(ppcdata.TmAddress) == 0 {
				return fmt.Errorf("ppcdata.TmAddress is required after appversion 5 ")
			}
			ppcdata.TmAddress = strings.ToUpper(ppcdata.TmAddress)
			if err = authTable.DeleteTmAddrSignerPair(ppcdata.TmAddress, from); err != nil {
				return err
			}
			authTable.ThisBlockChangedMap[ppcdata.TmAddress] = true
		}

		if err = authTable.DeleteAuthItem(ppcdata.PermittedAddress); err != nil {
			panic(err) //AuthItemMap should be consistent with RevertAuthItemMap
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
