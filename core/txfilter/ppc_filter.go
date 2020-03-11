package txfilter

import (
	"errors"
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"fmt"
	"math/big"
)

var (
	SendToMint  = common.HexToAddress("0x1111111111111111111111111111111111111111")
	SendToAuth  = common.HexToAddress("0x2222222222222222222222222222222222222222")
	SendToRelay = common.HexToAddress("0x3333333333333333333333333333333333333333")

	ErrPermitTableNotCreate = errors.New("PermitTable has not created yet")
)

var (
	PPChainAdmin common.Address
	Bigguy       common.Address
)

//signed data: rlp(subTransaction fields + from)
func DeriveRelayer(from common.Address, txDataBytes []byte) (subTx *types.Transaction, relayer common.Address, err error) {
	subTx, err = types.DecodeTx(txDataBytes)
	if err != nil {
		return subTx, common.Address{}, fmt.Errorf("ppc tx data is not a subTx structure. %v", err)
	}
	var signer types.Signer = types.HomesteadSigner{}
	if subTx.Protected() {
		signer = types.NewEIP155Signer(subTx.ChainId())
	}
	relayer, err = signer.RelaySender(subTx, from)
	return
}

func CheckRelayerTx(tx, subTx *types.Transaction) (err error) {
	if tx.Nonce() != subTx.Nonce() {
		fmt.Printf("tx nonce %v not equal to sub tx nonce %v", tx.Nonce(), subTx.Nonce())
		return fmt.Errorf("tx nonce %v not equal to sub tx nonce %v", tx.Nonce(), subTx.Nonce())
	}
	if tx.Value() != subTx.Value() {
		fmt.Printf("tx value %v not equal to sub tx value %v", tx.Value(), subTx.Value())
		return fmt.Errorf("tx nonce %v not equal to sub tx nonce %v", tx.Value(), subTx.Value())
	}
	if tx.GasPrice() != subTx.GasPrice() {
		fmt.Printf("tx price %v not equal to sub tx price %v", tx.GasPrice(), subTx.GasPrice())
		return fmt.Errorf("tx price %v not equal to sub tx price %v", tx.GasPrice(), subTx.GasPrice())
	}
	if tx.Gas() != subTx.Gas() {
		fmt.Printf("tx gasLimit %v not equal to sub tx gasLimit %v", tx.Gas(), subTx.Gas())
		return fmt.Errorf("tx gasLimit %v not equal to sub tx gasLimit %v", tx.Gas(), subTx.Gas())
	}
}

func IsAuthBlocked(from common.Address, txDataBytes []byte, height int64) (err error) {
	var ppcdata AuthData
	ppcdata, err = UnMarshalPermitTxData(txDataBytes)
	if err != nil {
		fmt.Printf("admin %X sent an auth tx with illegal format \n", from)
		return fmt.Errorf("admin %X sent an auth tx with illegal format \n", from)
	}
	if EthPermitTable == nil {
		return ErrPermitTableNotCreate
	}
	EthPermitTable.Mtx.RLock()
	if ppcdata.OperationType == "add" {
		permitItem, exist := EthPermitTable.PermitItemMap[ppcdata.PermittedAddress]
		if exist {
			EthPermitTable.Mtx.RUnlock()
			fmt.Printf("addr %X already permitted at height %d , permit range [%v,%v]", ppcdata.PermittedAddress, permitItem.PermitHeight, permitItem.StartHeight, permitItem.EndHeight)
			return fmt.Errorf("addr %X already permitted at height %d , permit range [%v,%v]", ppcdata.PermittedAddress, permitItem.PermitHeight, permitItem.StartHeight, permitItem.EndHeight)
		} else {
			if _, exist := EthPermitTable.PermitItemMap[ppcdata.PermittedAddress]; exist {
				return fmt.Errorf("insertPermitItem in auth check, permittedAddr %X already exist", ppcdata.PermittedAddress)
			}
			EthPermitTable.Mtx.RUnlock()
			return
		}
	} else if ppcdata.OperationType == "remove" {
		if _, exist := EthPermitTable.PermitItemMap[ppcdata.PermittedAddress]; !exist {
			return fmt.Errorf("removePermitItem in auth check, permittedAddr %X not exists", ppcdata.PermittedAddress)
		}
		EthPermitTable.Mtx.RUnlock()
		return
	} else if ppcdata.OperationType == "kickout" {
		EthPermitTable.Mtx.RUnlock()
		EthPosTable.Mtx.RLock()
		defer EthPosTable.Mtx.RUnlock()
		if _, ok := EthPosTable.PosItemMap[ppcdata.PermittedAddress]; !ok {
			fmt.Printf("admin %X wants to kickout %X, but it is not in the PosTable \n", from, ppcdata.PermittedAddress)
			return fmt.Errorf("admin %X wants to kickout %X, but it is not in the PosTable \n", from, ppcdata.PermittedAddress)
		}
		return EthPosTable.RemovePosItem(ppcdata.PermittedAddress, height, false)
	}
	return fmt.Errorf("admin %X sent an unrecognized OperationType %v \n", from, ppcdata.OperationType)
}

func DoAuthHandle(from, to common.Address, balance *big.Int, txDataBytes []byte, height int64) (err error) {
	var ppcdata AuthData
	ppcdata, err = UnMarshalPermitTxData(txDataBytes)
	if err != nil {
		fmt.Printf("admin %X sent an auth tx with illegal format \n", from)
		return fmt.Errorf("admin %X sent an auth tx with illegal format \n", from)
	}
	if EthPermitTable == nil {
		return ErrPermitTableNotCreate
	}
	EthPermitTable.Mtx.Lock()
	if ppcdata.OperationType == "add" {
		permitItem, exist := EthPermitTable.PermitItemMap[ppcdata.PermittedAddress]
		if exist {
			EthPermitTable.Mtx.Unlock()
			fmt.Printf("addr %X already permitted at height %d , permit range [%v,%v]", ppcdata.PermittedAddress, permitItem.PermitHeight, permitItem.StartHeight, permitItem.EndHeight)
			return fmt.Errorf("addr %X already permitted at height %d , permit range [%v,%v]", ppcdata.PermittedAddress, permitItem.PermitHeight, permitItem.StartHeight, permitItem.EndHeight)
		} else {
			permitItem = &PermitItem{
				ApprovedTxDataHash: ppcdata.ApprovedTxDataHash,
				StartHeight:        ppcdata.StartBlockHeight,
				EndHeight:          ppcdata.EndBlockHeight,
				PermitHeight:       height,
			}
			err = EthPermitTable.InsertPermitItem(ppcdata.PermittedAddress, permitItem)
			EthPermitTable.Mtx.Unlock()
			return
		}
	} else if ppcdata.OperationType == "remove" {
		err = EthPermitTable.DeletePermitItem(ppcdata.PermittedAddress)
		EthPermitTable.Mtx.Unlock()
		return
	} else if ppcdata.OperationType == "kickout" {
		EthPermitTable.Mtx.Unlock()
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
