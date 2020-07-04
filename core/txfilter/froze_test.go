package txfilter

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
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

func TestUnmarshalFrozeTable(t *testing.T) {
	datastr := "{\"froze_item_map\":{},\"wait_for_delete_map\":{}}"
	EthFrozeTable = &FrozeTable{}
	err := json.Unmarshal([]byte(datastr), EthFrozeTable)
	fmt.Println(EthFrozeTable)
	isExisted := IsFrozed(common.HexToAddress("0x5555555555555555555555555555555555555553"))
	fmt.Println(isExisted)

	if err != nil{
		fmt.Println(err)
		t.Fail()
	}
}

func TestUnmarshalFrozeTable2(t *testing.T) {
	datastr := "{\"froze_item_map\":{},\"wait_for_delete_map\":{},\"this_block_changed_flag\":true}"
	EthFrozeTable = &FrozeTable{}
	err := json.Unmarshal([]byte(datastr), EthFrozeTable)
	fmt.Println(EthFrozeTable)
	isExisted := IsFrozed(common.HexToAddress("0x5555555555555555555555555555555555555553"))
	fmt.Println(isExisted)

	if err != nil{
		fmt.Println(err)
		t.Fail()
	}
}
