package ethapi

import (
	"encoding/json"
)

type TxBatch struct {
	SubTxs []string         `json:"subTxs"`
}

func UnMarshalTxBatchData(jsonByte []byte) (TxBatch, error) {
	txBatchData := TxBatch{}
	err := json.Unmarshal(jsonByte, &txBatchData)
	if err != nil {
		return txBatchData, err
	}
	return txBatchData, err
}
