package txfilter

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
)

/*
	Frooze filter is used to filter some frooze account,include user-address
	and contract-address.
	we maintain a frooze list, like auth-table account.

	It include the follow method:
	1. frooze account
	2. thaw account
	3. filter all the frooze account
*/

var (
	AccountAdmin common.Address
	SendToFroze  = common.HexToAddress("0x5555555555555555555555555555555555555555")
)

// tx.To equls 0x555...55, we should try to check whether it is legal
// If appversion < 6, we needn't to verify it.
func IsFrozeBlocked(from common.Address, to *common.Address) (err error) {
	if IsFrozed(from) {
		fmt.Printf("Sender %X have been in frozen state.\n", from)
		return fmt.Errorf("Sender %X have been in frozen state.\n", from)
	}
	if to != nil {
		if IsFrozed(*to) {
			fmt.Printf("Receiver %X was in frozen state.\n", from)
			return fmt.Errorf("Receiver %X was in frozen state.\n", from)
		}
	}
	return nil
}

func ValidateFrozeTx(from common.Address, txDataBytes []byte) (err error) {
	if !bytes.Equal(from.Bytes(), AccountAdmin.Bytes()) {
		fmt.Printf("not account admin %X sent an froze tx \n", from)
		return fmt.Errorf("not account admin %X sent an froze tx \n", from)
	}

	frozeData, err := UnMarshalFrozeData(txDataBytes)
	if frozeData.OperationType == "froze" {
		if IsFrozed(frozeData.FrozeAddress) {
			fmt.Printf("account %X have already been frozen.\n", frozeData.FrozeAddress)
			return fmt.Errorf("account %X have already been frozen.\n", frozeData.FrozeAddress)
		}
	} else if frozeData.OperationType == "thaw" {
		if !IsFrozed(frozeData.FrozeAddress) {
			fmt.Printf("account %X have not been frozen.\n", frozeData.FrozeAddress)
			return fmt.Errorf("account %X have not been frozen.\n", frozeData.FrozeAddress)
		}
	} else {
		fmt.Printf("unrecognized operation type:%v \n", frozeData.OperationType)
		return fmt.Errorf("unrecognized operation type:%v \n", frozeData.OperationType)
	}
	return nil
}

//We should check it again , for the state may have changed in the prior tx of same block.
//If appversion < 6, we needn't run `DoFrozeHandle`
func DoFrozeHandle(from common.Address, to common.Address, txDataBytes []byte, height int64) (isFrozeTx bool, err error) {
	if !IsFrozeTx(to) {
		return false, nil
	}

	if !bytes.Equal(from.Bytes(), AccountAdmin.Bytes()) {
		fmt.Printf("not account admin %X sent an froze tx \n", from)
		return true, fmt.Errorf("not account admin %X sent an froze tx \n", from)
	}

	frozeData, err := UnMarshalFrozeData(txDataBytes)
	if frozeData.OperationType == "froze" {
		if IsFrozed(frozeData.FrozeAddress) {
			fmt.Printf("account %X have already been frozen.\n", frozeData.FrozeAddress)
			return true, fmt.Errorf("account %X have already been frozen.\n", frozeData.FrozeAddress)
		} else {
			frozeItem := FrozeItem{
				IsContractAddress: frozeData.IsContractAddress,
				Froze_height:      height,
			}
			EthFrozeTable.FrozeItemMap[frozeData.FrozeAddress] = &frozeItem
			EthFrozeTable.ThisBlockChangedFlag = true
		}
	} else if frozeData.OperationType == "thaw" {
		if !IsFrozed(frozeData.FrozeAddress) {
			fmt.Printf("account %X have not been frozen.\n", frozeData.FrozeAddress)
			return true, fmt.Errorf("account %X have not been frozen.\n", frozeData.FrozeAddress)
		} else {
			EthFrozeTable.ThisBlockChangedFlag = true
			EthFrozeTable.WaitForDeleteMap[frozeData.FrozeAddress] = EthFrozeTable.FrozeItemMap[frozeData.FrozeAddress]
		}
	} else {
		fmt.Printf("unrecognized operation type:%v \n", frozeData.OperationType)
		return true, fmt.Errorf("unrecognized operation type:%v \n", frozeData.OperationType)
	}

	return true, nil
}

//If account is frozed , all the relative txs should be blocked
func IsFrozed(address common.Address) bool {
	_, isExisted := EthFrozeTable.FrozeItemMap[address]
	return isExisted
}

func IsAccountAdmin(from common.Address) bool {
	return bytes.Equal(from.Bytes(), SendToFroze.Bytes())
}

func IsFrozeTx(to common.Address) bool {
	return bytes.Equal(to.Bytes(), SendToFroze.Bytes())
}
