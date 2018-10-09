package blacklist

import (
	"testing"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
)

func init() {
	fmt.Println("init")
}

func TestNewMemoryBlacklistDB(t *testing.T) {
	hashes := common.Hash{}
	fmt.Println(hashes)
	fmt.Println(hashes.Big())
	fmt.Println(hashes.Big().Int64())
}
