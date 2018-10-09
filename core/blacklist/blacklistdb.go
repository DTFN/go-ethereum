package blacklist

import (
	"github.com/ethereum/go-ethereum/common"
	"strings"
	"math/big"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"errors"
)

var (
	blacklistDBEntryExpiration int64 = 20
	sendToLock                       = "0x7777777777777777777777777777777777777777"
	sendToUnlock                     = "0x8888888888888888888888888888888888888888"
	w                                = make(map[string]bool)
	locked                           = int64(^uint(0) >> 1) // max int64
	lockInfoKey                      = common.BytesToHash([]byte("LOCK_INFO"))
	lockInfoValue                    = common.BigToHash(big.NewInt(locked))
)

func init() {
	w[sendToLock] = true
	w[sendToUnlock] = true
}

func Lock(db *state.StateDB, address common.Address) {
	db.SetState(address, lockInfoKey, lockInfoValue)
}

func Unlock(db *state.StateDB, address common.Address, height *big.Int) {
	if db.GetState(address, lockInfoKey) == lockInfoValue {
		db.SetState(address, lockInfoKey, common.BigToHash(height))
	}
}

func Validate(evm *vm.EVM, from common.Address, to *common.Address) error {
	h := evm.StateDB.GetState(from, lockInfoKey).Big().Int64()
	if h != 0 {
		l := (h == locked) || (h+blacklistDBEntryExpiration >= evm.BlockNumber.Int64())
		forbiddenSendee := to == nil || !w[to.Hex()]
		if l && forbiddenSendee {
			return errors.New("locked")
		}
	}
	return nil
}

func IsLockTx(to string) bool {
	return strings.EqualFold(to, sendToLock)
}

func IsUnlockTx(to string) bool {
	return strings.EqualFold(to, sendToUnlock)
}
