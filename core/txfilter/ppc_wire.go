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

type ClientTxData struct {
	RelayerAddress     string `json:"relayerAddress"`
	EncodeData      string `json:"encodeData"`
	ContractAddress string `json:"contractAddress"`
	RelayerSignedMessage string `json:"relayerSignedMessage"`
}

func ClientUnMarshalTxData(jsonByte []byte) (*ClientTxData, error) {
	d := &ClientTxData{}
	err := json.Unmarshal(jsonByte, d)
	return d, err
}

type RelayerSignedData struct {
	Nonce uint64 `json:"nonce"`
	ClientAddress string `json:"clientAddress"`
	EncodeData string `json:"encodeData"`
}

func RelayUnMarshalSignedTxData(jsonByte []byte) (*RelayerSignedData, error) {
	d := &RelayerSignedData{}
	err := json.Unmarshal(jsonByte, d)
	return d, err
}