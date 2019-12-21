package txfilter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	abciTypes "github.com/tendermint/tendermint/abci/types"
	"testing"
)

func TestDecodePPCWire(t *testing.T) {
	// step 1. client construct ppctxdata
	PubKey1 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000001"),
	}
	fmt.Println(PubKey1.String())

	Address1 := common.HexToAddress("0x0000000000000000000000000000000000000001")
	BlsKeyString1 := "fake blsKeyString1"

	ppcTxData := PPCTxData{Beneficiary: Address1.String(), PubKey: PubKey1, BlsKeyString: BlsKeyString1}
	ppcTxDataBytes, _ := json.Marshal(ppcTxData)

	fmt.Println(string(ppcTxDataBytes))

	//step2: client construct ppctx
	ppcTx := PPCTx{
		PPCTxDataStr:     string(ppcTxDataBytes),
		StartBlockHeight: 10000,
		EndBlockHeight:   100000,
	}
	ppcTxBytes, _ := json.Marshal(ppcTx)
	fmt.Println(string(ppcTxBytes))

	//Step3: client got the signatures,construct PPCSignature
	ppcSign := PPCSignature{
		Signatures:    []string{},
		SigMsgJsonStr: string(ppcTxBytes),
	}

	signature := "8aff64671554886e0c136228518c5ef07d95b459f888757ea035c565531d12fa5465284a48274175c721ec9d448838e2146a4d6c510d535eb0b5f7358e488a671c"
	ppcSign.Signatures = append(ppcSign.Signatures, signature)

	//Step4 : node try to verify signautre
	data := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(string(ppcTxBytes)), string(ppcTxBytes))
	hash := crypto.Keccak256([]byte(data))
	sig, _ := hex.DecodeString(ppcSign.Signatures[0])

	recoveredAddr, _ := RecoverAddrFromSig(hash, sig)
	require.Equal(t, recoveredAddr.String(), "0x796E349A1252b43E358Aa65e4c19a52E1375F9Eb")

	//step5: unmarshal PPCTx
	var ppcTxUnmarshal PPCTx
	json.Unmarshal(ppcTxBytes, &ppcTxUnmarshal)
	require.Equal(t, ppcTxUnmarshal.StartBlockHeight, uint64(10000))
	require.Equal(t, ppcTxUnmarshal.EndBlockHeight, uint64(100000))

	//step6: unmarshal ppcTxData
	var ppcTxDataUnmarshal PPCTxData
	json.Unmarshal([]byte(ppcTxUnmarshal.PPCTxDataStr), &ppcTxDataUnmarshal)
	require.Equal(t, ppcTxDataUnmarshal.BlsKeyString, BlsKeyString1)
	require.Equal(t, ppcTxDataUnmarshal.PubKey.String(), PubKey1.String())
	require.Equal(t, ppcTxDataUnmarshal.Beneficiary, Address1.String())
}
