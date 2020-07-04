package txfilter

import "github.com/ethereum/go-ethereum/common"

var (
	EthFrozeTable     *FrozeTable
	EthFrozeTableCopy *FrozeTable
)

type FrozeTable struct {
	FrozeItemMap map[common.Address]*FrozeItem `json:"froze_item_map"`

	WaitForDeleteMap map[common.Address]*FrozeItem `json:"wait_for_delete_map"`

	ThisBlockChangedFlag bool `json:"this_block_changed_flag"`
}

type FrozeItem struct {
	IsContractAddress bool  `json:"is_contract_address"`
	Froze_height      int64 `json:"froze_height"`
}

func (frozeItem *FrozeItem) Copy() *FrozeItem {
	return &FrozeItem{
		IsContractAddress: frozeItem.IsContractAddress,
		Froze_height:      frozeItem.Froze_height,
	}
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
		FrozeItemMap:         make(map[common.Address]*FrozeItem),
		WaitForDeleteMap:     make(map[common.Address]*FrozeItem),
		ThisBlockChangedFlag: false,
	}
}

func (frozeTable *FrozeTable) Copy() *FrozeTable {
	copyFrozeTable := NewFrozeTable()
	//we just need copy FrozeItemMap, ThisBlockChanedMap and WaitForDeleteMap will reset every block.
	for addr, frozeItem := range frozeTable.FrozeItemMap {
		copyFrozeTable.FrozeItemMap[addr] = frozeItem.Copy()
	}
	return copyFrozeTable
}
