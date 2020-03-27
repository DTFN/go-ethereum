package txfilter

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
)

var (
	EthAuthTable *AuthTable
)

// TODO: merge this map into PosTable
type AuthTable struct {
	AuthItemMap map[common.Address]*AuthItem `json:"auth_item_map"`

	//This is used to record all the auth-tx in a block
	//and will be cleared in the endBlock
	//needn't be writen into merkle-trie
	ThisBlockChangedMap map[common.Address]*AuthTmItem `json:"-"`
}

type AuthItem struct {
	ApprovedTxDataHash []byte `json:"approved_tx_data_hash"`
	PermitHeight       int64  `json:"auth_height"`
	StartHeight        int64  `json:"start_height"`
	EndHeight          int64  `json:"end_height"`
}

type AuthTmItem struct {
	ApprovedTxData TxData
	Type           string
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
		AuthItemMap:         make(map[common.Address]*AuthItem),
		ThisBlockChangedMap: make(map[common.Address]*AuthTmItem),
	}
}

func (permitTable *AuthTable) InsertAuthItem(permittedAddr common.Address, pi *AuthItem) error {
	fmt.Printf("insert pmi %v for permittedAddr %X", pi, permittedAddr)
	EthPosTable.ChangedFlagThisBlock = true
	if _, ok := permitTable.AuthItemMap[permittedAddr]; ok {
		return fmt.Errorf("InsertAuthItem, permittedAddr %X already exist", permittedAddr)
	}
	permitTable.AuthItemMap[permittedAddr] = pi
	return nil
}

func (permitTable *AuthTable) DeleteAuthItem(permittedAddr common.Address) error {
	fmt.Printf("delete pmi for permittedAddr %X", permittedAddr)
	if _, ok := permitTable.AuthItemMap[permittedAddr]; !ok {
		fmt.Printf("DeleteAuthItem, permittedAddr %X does not exist \n", permittedAddr)
		return fmt.Errorf("DeletePermitItem, permittedAddr %X does not exist \n", permittedAddr)
	}
	EthPosTable.ChangedFlagThisBlock = true
	delete(permitTable.AuthItemMap, permittedAddr)
	return nil
}
