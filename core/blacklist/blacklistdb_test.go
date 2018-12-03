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
	locked = int64(^uint(0) >> 1) // max int64
	lockInfoKey = common.BytesToHash([]byte("LOCK_INFO"))
	lockInfoValue = common.BigToHash(big.NewInt(locked))
	fmt.Println(lockInfoValue)
}
