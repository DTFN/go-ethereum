package txfilter

import (
	"github.com/ethereum/go-ethereum/common"
	"fmt"
)

var (
	EthAuthTable *AuthTable
)

// TODO: merge this map into PosTable
type AuthTable struct {
	AuthItemMap        map[common.Address]*AuthItem `json:"auth_item_map"`
}

type AuthItem struct {
	ApprovedTxDataHash []byte `json:"approved_tx_data_hash"`
	PermitHeight       int64  `json:"auth_height"`
	StartHeight        int64  `json:"start_height"`
	EndHeight          int64  `json:"end_height"`
}

func CreateAuthTable() *AuthTable {
	if EthAuthTable != nil {
		panic("txfilter.EthPermitTable already exist")
	}
	EthAuthTable = NewAuthTable()
	return EthAuthTable
}

func NewAuthTable() *AuthTable {
	return &AuthTable{
		AuthItemMap:        make(map[common.Address]*AuthItem),
	}
}

func (permitTable *AuthTable) InsertAuthItem(permittedAddr common.Address, pi *AuthItem) error {
	fmt.Printf("insert pmi %v for permittedAddr %X", pi, permittedAddr)
	EthPosTable.ChangedFlagThisBlock = true
	if _, ok := permitTable.AuthItemMap[permittedAddr]; ok {
		return fmt.Errorf("InsertPermitItem, permittedAddr %X already exist", permittedAddr)
	}
	permitTable.AuthItemMap[permittedAddr] = pi
	return nil
}

func (permitTable *AuthTable) DeleteAuthItem(permittedAddr common.Address) error {
	fmt.Printf("delete pmi for permittedAddr %X", permittedAddr)
	if _, ok := permitTable.AuthItemMap[permittedAddr]; !ok {
		fmt.Printf("DeletePermitItem, permittedAddr %X does not exist \n", permittedAddr)
		return fmt.Errorf("DeletePermitItem, permittedAddr %X does not exist \n", permittedAddr)
	}
	EthPosTable.ChangedFlagThisBlock = true
	delete(permitTable.AuthItemMap, permittedAddr)
	return nil
}
