package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txfilter"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"reflect"
	"strconv"
	"unsafe"
)

var (
	bigGuy            = common.HexToAddress("0x63859f245ba7c3c407a603a6007490b217b3ec5d")
	mintGasAccount    = common.HexToAddress("0x5555555555555555555555555555555555555555")
	multiBetTxAccount = common.HexToAddress("0x6666666666666666666666666666666666666666")
)

func PPCApplyTransactionWithFrom(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, from common.Address, usedGas *uint64, cfg vm.Config) (*types.Receipt, Message, uint64, error) {
	msg, _ := tx.AsMessageWithFrom(from)
	mintFlag := false
	multiBetFlag := false
	var mintGasNumber *big.Int

	//ignore value to forbid  eth-transfer
	if !bytes.Equal(from.Bytes(), bigGuy.Bytes()) {
		msg, _ = tx.AsMessageWithPPCFrom(from)
	} else if bytes.Equal(from.Bytes(), bigGuy.Bytes()) && bytes.Equal(msg.To().Bytes(), mintGasAccount.Bytes()) {
		msg, _ = tx.AsMessageWithPPCFrom(from)
		mintData, _ := strconv.ParseInt(string(msg.Data()), 10, 64)
		mintGasNumber = big.NewInt(mintData)
		mintFlag = true
	} else if bytes.Equal(from.Bytes(), bigGuy.Bytes()) && bytes.Equal(msg.To().Bytes(), multiBetTxAccount.Bytes()) {
		encodedStr := BytesToString(msg.Data())
		testBytes, _ := hex.DecodeString(encodedStr)

		tx, _ = ppcDecodeTx(testBytes)

		var signer types.Signer = types.HomesteadSigner{}
		if tx.Protected() {
			signer = types.NewEIP155Signer(tx.ChainId())
		}
		var err error
		// Make sure the transaction is signed properly
		subFrom, err := types.Sender(signer, tx)
		if err != nil {
			return nil, nil, 0, err
		}

		msg, _ = tx.AsMessageWithFrom(subFrom)
		from = subFrom
		multiBetFlag = true
	} else if bytes.Equal(msg.To().Bytes(), txfilter.SendToLock.Bytes()) || bytes.Equal(msg.To().Bytes(), txfilter.SendToUnlock.Bytes()) {
		return nil, nil, 0, txfilter.ErrIllegalBetTx
	}

	r, u, e := ppcApplyTransactionMessage(config, bc, author, gp, statedb, header, tx, msg, usedGas, cfg, mintFlag, mintGasNumber, multiBetFlag)
	return r, msg, u, e
}

func ppcApplyTransactionMessage(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, msg types.Message, usedGas *uint64, cfg vm.Config, mintFlag bool, mintGasNumber *big.Int, multiBetFlag bool) (*types.Receipt, uint64, error) {
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err := PPCApplyMessage(vmenv, msg, gp, mintFlag, mintGasNumber, multiBetFlag)
	if err != nil {
		return nil, 0, err
	}
	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(header.Number) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, gas, err
}

// PPCApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// PPCApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func PPCApplyMessage(evm *vm.EVM, msg Message, gp *GasPool, mintFlag bool, mintGasNumber *big.Int, multiBetFlag bool) ([]byte, uint64, bool, error) {
	return NewStateTransition(evm, msg, gp).PPCTransitionDb(mintFlag, mintGasNumber, multiBetFlag)
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) PPCTransitionDb(mintFlag bool, mintGasNumber *big.Int, multiBetFlag bool) (ret []byte, usedGas uint64, failed bool, err error) {
	if multiBetFlag {
		if err = st.ppcPreCheck(); err != nil {
			return
		}
	} else {
		if err = st.preCheck(); err != nil {
			return
		}
	}
	msg := st.msg
	sender := vm.AccountRef(msg.From())
	homestead := st.evm.ChainConfig().IsHomestead(st.evm.BlockNumber)
	contractCreation := msg.To() == nil

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation, homestead)
	if err != nil {
		return nil, 0, false, err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, err
	}

	var (
		evm = st.evm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)

	if st.evm.BlockNumber.Int64() <= 3588000 {
		if contractCreation {
			vmerr := txfilter.PPCIsBlocked(msg.From(), common.Address{}, st.state.GetBalance(msg.From()), msg.Data())
			if vmerr == nil {
				ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
			} else {
				st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
				//wenbin add,support multi-tx nonce
				if multiBetFlag {
					st.state.SetNonce(bigGuy, st.state.GetNonce(bigGuy)+1)
				}
			}
		} else {
			// Increment the nonce for the next transaction
			st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			//wenbin add,support multi-tx nonce
			if multiBetFlag {
				st.state.SetNonce(bigGuy, st.state.GetNonce(bigGuy)+1)
			}

			isBetTx, vmerr := txfilter.PPCDoFilter(msg.From(), *msg.To(), st.state.GetBalance(msg.From()), msg.Data(), st.evm.BlockNumber.Int64())
			if vmerr == nil && !isBetTx {
				ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
			}
		}
	} else {
		if contractCreation {
			vmerr = txfilter.PPCIsBlocked(msg.From(), common.Address{}, st.state.GetBalance(msg.From()), msg.Data())
			if vmerr == nil {
				ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
			} else {
				st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)

				//wenbin add,support multi-tx nonce
				if multiBetFlag {
					st.state.SetNonce(bigGuy, st.state.GetNonce(bigGuy)+1)
				}
			}
		} else {
			// Increment the nonce for the next transaction
			st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)

			//wenbin add,support multi-tx nonce
			if multiBetFlag {
				st.state.SetNonce(bigGuy, st.state.GetNonce(bigGuy)+1)
			}

			isBetTx := false
			isBetTx, vmerr = txfilter.PPCDoFilter(msg.From(), *msg.To(), st.state.GetBalance(msg.From()), msg.Data(), st.evm.BlockNumber.Int64())
			if vmerr == nil && !isBetTx {
				ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
			}
		}
	}

	if vmerr != nil {
		log.Info("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, false, vmerr
		}
	}
	if multiBetFlag {
		st.ppcRefundGas()
	} else {
		st.refundGas()
	}

	st.state.AddBalance(bigGuy, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	//If mintFlag is true,mint gas to bigGuy
	if mintFlag {
		st.state.AddBalance(bigGuy, new(big.Int).Mul(mintGasNumber, big.NewInt(1000000000000000000)))
	}

	gasAmount := new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)
	fmt.Println(gasAmount)
	Gwei := new(big.Int).Div(gasAmount, new(big.Int).Mul(big.NewInt(1000), big.NewInt(1000000)))

	return ret, Gwei.Uint64(), vmerr != nil, err
}

func (st *StateTransition) ppcPreCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(st.msg.From())
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.ppcBuyGas()
}

func (st *StateTransition) ppcBuyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if st.state.GetBalance(st.msg.From()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(bigGuy, mgval)
	return nil
}

func (st *StateTransition) ppcRefundGas() {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(bigGuy, remaining)

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// rlp decode an etherum transaction
func ppcDecodeTx(txBytes []byte) (*types.Transaction, error) {
	tx := new(types.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}

func BytesToString(b []byte) string {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{bh.Data, bh.Len}
	return *(*string)(unsafe.Pointer(&sh))
}
