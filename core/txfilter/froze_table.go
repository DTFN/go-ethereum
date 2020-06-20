package txfilter

import "github.com/ethereum/go-ethereum/common"

type FrozeTable struct {
	FrozeItemMap map[common.Address]*FrozeItem `json:"froze_item_map"`
}

type FrozeItem struct {
	IsContractAddress bool  `json:"is_contract_address"`
	froze_height      int64 `json:"froze_height"`
}

func CreateFrozeTable() *FrozeTable {
	if EthFrozeTable != nil {
		panic("txfilter.EthPermitTable already exist")
	}
	EthFrozeTable = NewFrozeTable()
	return EthFrozeTable
}

func NewFrozeTable() *FrozeTable {
	return &FrozeTable{
		FrozeItemMap: make(map[common.Address]*FrozeItem),
	}
}
