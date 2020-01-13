package txfilter

import (
	"github.com/ethereum/go-ethereum/common"
	"sync"
)

type PPCCATable struct {
	Mtx                  sync.RWMutex                      `json:"-"`
	ChangedFlagThisBlock bool                              `json:"-"`
	PPCCATableItemMap    map[common.Address]PPCCATableItem `json:"ppc_ca_talbe_item_map"`
}

type PPCCachedTx struct {
	Mtx      sync.RWMutex
	CachedTx map[string]common.Address
}

type PPCCATableItem struct {
	ApprovedTxDataHash string `json:"approved_tx_data_hash"`
	StartHeight        uint64 `json:"start_height"`
	EndHeight          uint64 `json:"end_height"`
	Used               bool   `json:"used"`
}

func NewPPCCATable() PPCCATable {
	return PPCCATable{
		ChangedFlagThisBlock: false,
		PPCCATableItemMap:    make(map[common.Address]PPCCATableItem),
	}
}

func NewPPCCachedTx() *PPCCachedTx {
	return &PPCCachedTx{
		CachedTx: make(map[string]common.Address),
	}
}
