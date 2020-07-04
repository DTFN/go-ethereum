package txfilter

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
)

type FrozeData struct {
	FrozeAddress      common.Address `json:"froze_address"`
	IsContractAddress bool           `json:"is_contract_address"`
	OperationType     string         `json:"operation_type"` // "froze" or "thaw"
}

func UnMarshalFrozeData(jsonByte []byte) (FrozeData, error) {
	frozeData := FrozeData{}
	err := json.Unmarshal(jsonByte, &frozeData)
	if err != nil {
		return frozeData, err
	}
	return frozeData, err
}
