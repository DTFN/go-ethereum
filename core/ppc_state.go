package core

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txfilter"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"strconv"
)

var (
	bigGuy         = common.HexToAddress("0x63859f245ba7c3c407a603a6007490b217b3ec5d")
	mintGasAccount = common.HexToAddress("0x5555555555555555555555555555555555555555")
)

func PPCApplyTransactionWithFrom(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, from common.Address, usedGas *uint64, cfg vm.Config) (*types.Receipt, Message, uint64, error) {
	// can't be replaced by AsMessageWithPPCFrom
	// cause we want to get msg.Value
	msg, _ := tx.AsMessageWithFrom(from)
	mintFlag := false
	var mintGasNumber *big.Int

	//ignore value to forbid  eth-transfer
	if !bytes.Equal(from.Bytes(),bigGuy.Bytes()){
		msg,_ = tx.AsMessageWithPPCFrom(from)
	}
	if bytes.Equal(from.Bytes(), bigGuy.Bytes()) && bytes.Equal(msg.To().Bytes(), mintGasAccount.Bytes()) {
		msg, _ = tx.AsMessageWithPPCFrom(from)
		mintData, _ := strconv.ParseInt(string(msg.Data()), 10, 64)
		mintGasNumber = big.NewInt(mintData)
		mintFlag = true
	}

	r, u, e := ppcApplyTransactionMessage(config, bc, author, gp, statedb, header, tx, msg, usedGas, cfg, mintFlag, mintGasNumber)
	return r, msg, u, e
}

func ppcApplyTransactionMessage(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, msg types.Message, usedGas *uint64, cfg vm.Config, mintFlag bool, mintGasNumber *big.Int) (*types.Receipt, uint64, error) {
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err := PPCApplyMessage(vmenv, msg, gp, mintFlag, mintGasNumber)
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
func PPCApplyMessage(evm *vm.EVM, msg Message, gp *GasPool, mintFlag bool, mintGasNumber *big.Int) ([]byte, uint64, bool, error) {
	return NewStateTransition(evm, msg, gp).PPCTransitionDb(mintFlag, mintGasNumber)
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) PPCTransitionDb(mintFlag bool, mintGasNumber *big.Int) (ret []byte, usedGas uint64, failed bool, err error) {
	if err = st.preCheck(); err != nil {
		return
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
			vmerr := txfilter.IsBlocked(msg.From(), common.Address{}, st.state.GetBalance(msg.From()), msg.Data())
			if vmerr == nil {
				ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
			} else {
				st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			}
		} else {
			// Increment the nonce for the next transaction
			st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			isBetTx, vmerr := txfilter.DoFilter(msg.From(), *msg.To(), st.state.GetBalance(msg.From()), msg.Data(), st.evm.BlockNumber.Int64())
			if vmerr == nil && !isBetTx {
				ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
			}
		}
	} else {
		if contractCreation {
			vmerr = txfilter.IsBlocked(msg.From(), common.Address{}, st.state.GetBalance(msg.From()), msg.Data())
			if vmerr == nil {
				ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
			} else {
				st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			}
		} else {
			// Increment the nonce for the next transaction
			st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			isBetTx := false
			isBetTx, vmerr = txfilter.DoFilter(msg.From(), *msg.To(), st.state.GetBalance(msg.From()), msg.Data(), st.evm.BlockNumber.Int64())
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
	st.refundGas()
	st.state.AddBalance(bigGuy, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	//If mintFlag is true,mint gas to bigGuy
	if mintFlag {
		st.state.AddBalance(bigGuy,new(big.Int).Mul(mintGasNumber,big.NewInt(1000000000000000000)))
	}

	gasAmount := new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)
	fmt.Println(gasAmount)
	Gwei := new(big.Int).Div(gasAmount, new(big.Int).Mul(big.NewInt(1000),big.NewInt(1000000)))

	return ret, Gwei.Uint64(), vmerr != nil, err
}
