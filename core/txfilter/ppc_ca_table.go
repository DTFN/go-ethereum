package txfilter

import (
	"github.com/ethereum/go-ethereum/common"
)

type PPCCATable struct {
	ChangedFlagThisBlock bool                              `json:"-"`
	PPCCATableItemMap    map[common.Address]PPCCATableItem `json:"ppc_ca_talbe_item_map"`
}

type PPCCATableItem struct {
	ApprovedTxData TxData `json:"approved_tx_data"`
	StartHeight    uint64 `json:"start_height"`
	EndHeight      uint64 `json:"end_height"`
	Used           bool   `json:"used"`
}

func NewPPCCATable() PPCCATable {
	return PPCCATable{
		ChangedFlagThisBlock: false,
		PPCCATableItemMap:    make(map[common.Address]PPCCATableItem),
	}
}
