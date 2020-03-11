package txfilter

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
)

type AuthData struct {
	PermittedAddress   common.Address `json:"permitted_address"`
	StartBlockHeight   int64          `json:"start_block_height"`
	EndBlockHeight     int64          `json:"end_block_height"`
	OperationType      string         `json:"operation_type"`
	ApprovedTxDataHash []byte         `json:"approved_tx_data_hash"`
}

func UnMarshalPermitTxData(jsonByte []byte) (AuthData, error) {
	ppcdata := AuthData{}
	err := json.Unmarshal(jsonByte, &ppcdata)
	if err != nil {
		return ppcdata, err
	}
	return ppcdata, err
}
