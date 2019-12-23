package txfilter

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
)

type PPCTx struct {
	OperationAddress common.Address `json:"operation_address"`
	ApprovedTxData   TxData         `json:"approved_tx_data"`
	StartBlockHeight uint64         `json:"start_block_height"`
	EndBlockHeight   uint64         `json:"end_block_height"`
	OperationType    string         `json:"operation_type"`
}

func PPCUnMarshalTxData(jsonByte []byte) (*PPCTx, error) {
	d := &PPCTx{}
	err := json.Unmarshal(jsonByte, d)
	return d, err
}
