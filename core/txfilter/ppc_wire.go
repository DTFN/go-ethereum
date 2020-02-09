package txfilter

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
)

type PPCTx struct {
	PermissonedAddress common.Address `json:"permissoned_address"`
	ApprovedTxData     TxData         `json:"approved_tx_data"`
	StartBlockHeight   uint64         `json:"start_block_height"`
	EndBlockHeight     uint64         `json:"end_block_height"`
	OperationType      string         `json:"operation_type"`
}

func PPCUnMarshalTxData(jsonByte []byte) (PPCTx, error) {
	var ppcdata PPCTx
	err := json.Unmarshal(jsonByte, &ppcdata)
	if err != nil {
		return ppcdata, err
	}
	return ppcdata, err
}

type RelayTxData struct {
	RelayerAddress     string `json:"relayerAddress"`
	EncodeData      string `json:"encodeData"`
	ContractAddress string `json:"contractAddress"`
}

func RelayUnMarshalTxData(jsonByte []byte) (*RelayTxData, error) {
	d := &RelayTxData{}
	err := json.Unmarshal(jsonByte, d)
	return d, err
}