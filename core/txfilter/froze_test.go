package txfilter

import (
	"fmt"
	"testing"
)

var inputFroze = `{
"froze_address":"0x231dD21555C6D905ce4f2AafDBa0C01aF89Db0a0",
"is_contract_address":true,
"operation_type":"froze"}`

func TestUnMarshalFrozeTxData(t *testing.T) {
	data, _ := UnMarshalFrozeData([]byte(inputFroze))
	fmt.Println(data.FrozeAddress.String())
	fmt.Println(data.IsContractAddress)
	fmt.Println(data.OperationType)
}
