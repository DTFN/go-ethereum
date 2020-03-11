package txfilter

import (
	"github.com/ethereum/go-ethereum/common"
	"sync"
	"fmt"
)

var (
	EthPermitTable *PermitTable
)

type PermitTable struct {
	Mtx                  sync.RWMutex                   `json:"-"`
	ChangedFlagThisBlock bool                           `json:"-"`
	PermitItemMap        map[common.Address]*PermitItem `json:"permit_item_map"`
	Init                 bool                           `json:"-"`
}

type PermitItem struct {
	ApprovedTxDataHash []byte `json:"approved_tx_data_hash"`
	PermitHeight       int64  `json:"permit_height"`
	StartHeight        int64  `json:"start_height"`
	EndHeight          int64  `json:"end_height"`
}

func CreatePermitTable() *PermitTable {
	if EthPermitTable != nil {
		panic("txfilter.EthPermitTable already exist")
	}
	EthPermitTable = NewPermitTable()
	return EthPermitTable
}

func NewPermitTable() *PermitTable {
	return &PermitTable{
		ChangedFlagThisBlock: false,
		PermitItemMap:        make(map[common.Address]*PermitItem),
	}
}

func (permitTable *PermitTable) InsertPermitItem(permittedAddr common.Address, pi *PermitItem) error {
	fmt.Printf("insert pmi %v for permittedAddr %X", pi, permittedAddr)
	permitTable.ChangedFlagThisBlock = true
	if _, ok := permitTable.PermitItemMap[permittedAddr]; ok {
		return fmt.Errorf("InsertPermitItem, permittedAddr %X already exist", permittedAddr)
	}
	permitTable.PermitItemMap[permittedAddr] = pi
	return nil
}

func (permitTable *PermitTable) DeletePermitItem(permittedAddr common.Address) error {
	fmt.Printf("delete pmi for permittedAddr %X", permittedAddr)
	if _, ok := permitTable.PermitItemMap[permittedAddr]; !ok {
		fmt.Printf("DeletePermitItem, permittedAddr %X does not exist \n", permittedAddr)
		return fmt.Errorf("DeletePermitItem, permittedAddr %X does not exist \n", permittedAddr)
	}
	permitTable.ChangedFlagThisBlock = true
	delete(permitTable.PermitItemMap, permittedAddr)
	return nil
}
