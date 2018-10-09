package blacklist

import (
	"github.com/ethereum/go-ethereum/common"
	"strings"
	"math/big"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core"
	"errors"
)

var (
	blacklistDBEntryExpiration int64           = 4
	sendToLock                                 = "0x7777777777777777777777777777777777777777"
	sendToUnlock                               = "0x8888888888888888888888888888888888888888"
	w                          map[string]bool = make(map[string]bool)
	lockInfoKey                                = common.BytesToHash([]byte("LOCK_INFO"))
	lockInfoValue                              = common.BigToHash(big.NewInt(-1))
)

func init() {
	w[sendToLock] = true
	w[sendToUnlock] = true
}

func Lock(db *state.StateDB, address common.Address) {
	db.SetState(address, lockInfoKey, lockInfoValue)
}

func Unlock(db *state.StateDB, address common.Address, height *big.Int) {
	db.SetState(address, lockInfoKey, common.BigToHash(height))
}

func Validate(evm *vm.EVM, msg core.Message) error {
	h := evm.StateDB.GetState(msg.From(), lockInfoKey).Big().Int64()
	locked := h == -1 || h+blacklistDBEntryExpiration >= blacklistDBEntryExpiration
	forbiddenSendee := msg.To() == nil || !w[msg.To().Hex()]
	if locked && forbiddenSendee {
		return errors.New("locked")
	}
	return nil
}

func IsLockTx(to string) bool {
	return strings.EqualFold(to, sendToLock)
}

func IsUnlockTx(to string) bool {
	return strings.EqualFold(to, sendToUnlock)
}
