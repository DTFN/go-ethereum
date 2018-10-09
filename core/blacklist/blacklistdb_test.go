package blacklist

import (
	"testing"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

func init() {
	fmt.Println("init")
}

func TestNewMemoryBlacklistDB(t *testing.T) {
	hashes := common.Hash{}
	fmt.Println(hashes)
	fmt.Println(hashes.Big())
	fmt.Println(hashes.Big().Int64())
	u := int64(^uint(0) >> 1)
	fmt.Println(common.BigToHash(big.NewInt(u)).Big().Int64())
}
