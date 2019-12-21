package txfilter

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	abciTypes "github.com/tendermint/tendermint/abci/types"
)

type PPCTxData struct {
	PubKey       abciTypes.PubKey `json:"pub_key"`
	Beneficiary  string           `json:"beneficiary"`
	BlsKeyString string           `json:"bls_key_string"`
}

type PPCTx struct {
	PPCTxDataStr     string `json:"ppc_tx_data_str"`
	StartBlockHeight uint64 `json:"start_block_height"`
	EndBlockHeight   uint64 `json:"end_block_height"`
}

type PPCSignature struct {
	Signatures    []string `json:"signatures"`
	SigMsgJsonStr string   `json:"sig_msg_json_str"`
}

func PPCUnMarshalTxData(jsonByte []byte) (*PPCTxData, error) {
	d := &PPCTxData{}
	err := json.Unmarshal(jsonByte, d)
	return d, err
}

func RecoverAddrFromSig(hash, sig []byte) (common.Address, error) {
	if len(sig) != 65 {
		return common.Address{}, fmt.Errorf("signature must be 65 bytes long")
	}
	if sig[64] != 27 && sig[64] != 28 {
		return common.Address{}, fmt.Errorf("invalid Ethereum signature (V is not 27 or 28)")
	}
	sig[64] -= 27 // Transform yellow paper V from 27/28 to 0/1

	rpk, err := crypto.Ecrecover(hash, sig)
	if err != nil {
		fmt.Println(err)
	}
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return recoveredAddr, nil
}
