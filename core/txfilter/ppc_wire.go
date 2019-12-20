package txfilter

import (
	"encoding/json"
	abciTypes "github.com/tendermint/tendermint/abci/types"
)

type PPCTxData struct {
	PubKey       abciTypes.PubKey `json:"pub_key"`
	Beneficiary  string           `json:"beneficiary"`
	BlsKeyString string           `json:"bls_key_string"`
}

func PPCUnMarshalTxData(jsonByte []byte) (*PPCTxData, error) {
	d := &PPCTxData{}
	err := json.Unmarshal(jsonByte, d)
	return d, err
}

