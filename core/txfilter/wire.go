package txfilter

import (
	"encoding/json"
	abciTypes "github.com/tendermint/tendermint/abci/types"
)

type TxData struct {
	PubKey	abciTypes.PubKey	`json:"pubKey"`
	Beneficiary  string	`json:"beneficiary"`
	BlsKeyString string	`json:"bls_key_string"`
}

func UnMarshalTxData(jsonByte []byte) (*TxData, error) {
	d := &TxData{}
	err:=json.Unmarshal(jsonByte, d)
	return d, err
}
