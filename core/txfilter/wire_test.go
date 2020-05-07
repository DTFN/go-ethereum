package txfilter

import (
	"testing"
	"fmt"
)

var input = `{
 "pub_key":{
"type":"ed25519","data":"EkrH/BnWtQs03ep15Dk+CYM+p9FztAEWajHOH/tvlpY="},
"beneficiary":"0x231dD21555C6D905ce4f2AafDBa0C01aF89Db0a0",
"bls_key_string":"{\"type\":\"Secp256k1\",\"address\":\"62F9FBE1E0BBFCEFDA8F79F1709E6B636D93BAB4\",\"value\":\"26419f5842b553dbc5e727a58369f721e4fc5d9d10a676502ed742f0657dc1618f839c9a81f4c436dc75804e9cbd6ef891295c01c2c6fc894d3c0174a843162c865\"}"
}`

func TestUnMarshalTxData(t *testing.T) {
	data, _ := UnMarshalTxData([]byte(input))
	fmt.Println(data.PubKey)
	fmt.Println(data.Beneficiary)
	fmt.Println(data.BlsKeyString)
}
