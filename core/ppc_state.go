package core

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"unsafe"
)

func PPCApplyTransactionWithFrom(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, from common.Address, usedGas *uint64, cfg vm.Config) (*types.Receipt, Message, uint64, error) {
	mintFlag := false
	multiRelayerFlag := false
	kickoutFlag := false
	noErrorInDataFlag := true

	var msg types.Message
	var mintGasNumber *big.Int
	var relayerAccount common.Address
	var originHash common.Hash
	ppcCATable := txfilter.NewPPCCATable()

	if bytes.Equal(from.Bytes(), txfilter.Bigguy.Bytes()) {
		msg, _ = tx.AsMessageWithFrom(from)
	} else {
		//ignore tx.data.amout to forbid eth-transfer-tx
		msg, _ = tx.AsMessageWithPPCFrom(from)
	}

	ppcTableBytes := statedb.GetCode(txfilter.PPCCATableAccount)
	if len(ppcTableBytes) != 0 {
		json.Unmarshal(ppcTableBytes, &ppcCATable)
	} else {
		ppcTableBytes, _ = json.Marshal(ppcCATable)
	}
	statedb.SetCode(txfilter.PPCCATableAccount, ppcTableBytes)

	if msg.To() == nil {
	} else {
		//bigguy may as relayyer and send bet-tx and deploy-contract
		//This is a bet-tx maybe sent by anyone
		if bytes.Equal(msg.To().Bytes(), txfilter.SendToLock.Bytes()) {
			//msg.From must equals Permissoned_address
			value, isExisted := ppcCATable.PPCCATableItemMap[msg.From()]
			if isExisted {
				if !value.Used && (header.Number.Uint64() >= value.StartHeight) &&
					(header.Number.Uint64() <= value.EndHeight) {
					//verify whether txdata equals approved txdata
					txData, err := txfilter.UnMarshalTxData(msg.Data())
					if err != nil {
						noErrorInDataFlag = false
						msg, _ = tx.AsMessageWithErrorData(from)
						log.Info("inillegal format txdata")
					} else {
						txDataStr, _ := json.Marshal(txData)
						data := []byte(txDataStr)
						hashData := md5.Sum(data)
						hashStr := hex.EncodeToString(hashData[:])
						if hashStr != value.ApprovedTxDataHash {
							noErrorInDataFlag = false
							msg, _ = tx.AsMessageWithErrorData(from)
							log.Info("txData doesn't match approved txData")
						}
					}
				} else {
					noErrorInDataFlag = false
					msg, _ = tx.AsMessageWithErrorData(from)
					log.Info("illeagal tx")
				}
			} else {
				noErrorInDataFlag = false
				msg, _ = tx.AsMessageWithErrorData(from)
				log.Info("illeagal tx")
			}
		}
		//This is a relay-tx maybe sent by anyone
		if bytes.Equal(msg.To().Bytes(), txfilter.RelayAccount.Bytes()) {
			txfilter.PPCTXCached.Mtx.Lock()
			defer txfilter.PPCTXCached.Mtx.Unlock()

			relayerAccount = msg.From()
			originHash = tx.Hash()

			var err error
			var subFrom common.Address

			tx, _ = PPCDecodeTx(msg.Data())
			data := msg.Data()
			hashData := md5.Sum(data)
			hashStr := hex.EncodeToString(hashData[:])
			_, ok := txfilter.PPCTXCached.CachedTx[hashStr]
			if ok {
				subFrom = txfilter.PPCTXCached.CachedTx[hashStr]
			} else {
				var signer types.Signer = types.HomesteadSigner{}
				if tx.Protected() {
					signer = types.NewEIP155Signer(tx.ChainId())
				}
				// Make sure the transaction is signed properly
				subFrom, err = types.Sender(signer, tx)
			}

			if err != nil {
				noErrorInDataFlag = false
				msg, _ = tx.AsMessageWithErrorData(from)
				log.Info(err.Error())
			} else {
				msg, _ = tx.AsMessageWithFrom(subFrom)
				from = subFrom
				multiRelayerFlag = true
			}
			//finally we will remove the data
			delete(txfilter.PPCTXCached.CachedTx, hashStr)
		}
		//This is an approved-tx sent by bigguy
		if bytes.Equal(msg.From().Bytes(), txfilter.Bigguy.Bytes()) && bytes.Equal(msg.To().Bytes(), txfilter.PPCCATableAccount.Bytes()) {
			//Init ppcCaTable
			msg, _ = tx.AsMessageWithPPCFrom(from)
			msgData := string(msg.Data())
			//manage PPCCATable by bigguy
			ppcTxData, _ := txfilter.PPCUnMarshalTxData([]byte(msgData))
			switch ppcTxData.OperationType {
			case "add":
				{
					ppcCATable.ChangedFlagThisBlock = true
					var ppcCATableItem txfilter.PPCCATableItem
					ppcCATableItem.Used = false

					txDataStr, _ := json.Marshal(ppcTxData.ApprovedTxData)
					data := []byte(txDataStr)
					hashData := md5.Sum(data)

					ppcCATableItem.ApprovedTxDataHash = hex.EncodeToString(hashData[:])
					ppcCATableItem.StartHeight = ppcTxData.StartBlockHeight
					ppcCATableItem.EndHeight = ppcTxData.EndBlockHeight

					ppcCATable.PPCCATableItemMap[ppcTxData.PermissonedAddress] = ppcCATableItem
				}
			case "remove":
				{
					ppcCATable.ChangedFlagThisBlock = true
					delete(ppcCATable.PPCCATableItemMap, ppcTxData.PermissonedAddress)
				}
			case "kickout":
				{
					//directly remove user in pos_table by bigguy
					ppcCATable.ChangedFlagThisBlock = true
					kickoutFlag = true
					delete(ppcCATable.PPCCATableItemMap, ppcTxData.PermissonedAddress)
					msg, _ = tx.AsMessageWithKickoutFrom(ppcTxData.PermissonedAddress, txfilter.SendToUnlock)
				}
			}
		}
		//This is a mint-tx sent by bigguy
		if bytes.Equal(from.Bytes(), txfilter.Bigguy.Bytes()) && bytes.Equal(msg.To().Bytes(), txfilter.MintGasAccount.Bytes()) {
			msg, _ = tx.AsMessageWithPPCFrom(from)
			mintData := string(msg.Data())
			mintGasNumber, mintFlag = new(big.Int).SetString(mintData, 10)
		}
	}

	r, u, e := ppcApplyTransactionMessage(originHash, config, bc, author, gp, statedb, header, tx, msg, usedGas, cfg, mintFlag, mintGasNumber, multiRelayerFlag, &relayerAccount, &ppcCATable, kickoutFlag, noErrorInDataFlag)

	txfilter.PPCCATableCopy = &ppcCATable
	//persist ppcCaTable
	if ppcCATable.ChangedFlagThisBlock {
		curBytes, _ := json.Marshal(ppcCATable)
		statedb.SetCode(txfilter.PPCCATableAccount, curBytes)
	}

	return r, msg, u, e
}

func ppcApplyTransactionMessage(originHash common.Hash, config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, msg types.Message, usedGas *uint64, cfg vm.Config, mintFlag bool, mintGasNumber *big.Int, multiRelayerFlag bool, relayerAccount *common.Address, ppcCATable *txfilter.PPCCATable, kickoutFlag bool, noErrorInDataFlag bool) (*types.Receipt, uint64, error) {
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err, doFilterFlag := PPCApplyMessage(vmenv, msg, gp, mintFlag, mintGasNumber, multiRelayerFlag, relayerAccount, kickoutFlag, noErrorInDataFlag)
	if err != nil {
		return nil, 0, err
	}
	failed = failed && noErrorInDataFlag
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
	if multiRelayerFlag {
		receipt.TxHash = originHash
	} else {
		receipt.TxHash = tx.Hash()
	}
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	if multiRelayerFlag {
		receipt.Logs = statedb.GetLogs(originHash)
	} else {
		receipt.Logs = statedb.GetLogs(tx.Hash())
	}
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	if doFilterFlag {
		ppcCATable.ChangedFlagThisBlock = true
		if bytes.Equal(msg.To().Bytes(), txfilter.SendToLock.Bytes()) {
			ppcCATableItem := ppcCATable.PPCCATableItemMap[msg.From()]
			ppcCATableItem.Used = true
			ppcCATable.PPCCATableItemMap[msg.From()] = ppcCATableItem
		}
	}

	return receipt, gas, err
}

// PPCApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// PPCApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func PPCApplyMessage(evm *vm.EVM, msg Message, gp *GasPool, mintFlag bool, mintGasNumber *big.Int, multiRelayerFlag bool, relayerAccount *common.Address, kickoutFlag bool, noErrorInDataFlag bool) ([]byte, uint64, bool, error, bool) {
	return NewStateTransition(evm, msg, gp).PPCTransitionDb(mintFlag, mintGasNumber, multiRelayerFlag, relayerAccount, kickoutFlag, noErrorInDataFlag)
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) PPCTransitionDb(mintFlag bool, mintGasNumber *big.Int, multiRelayerFlag bool, relayerAccount *common.Address, kickoutFlag bool, noErrorInDataFlag bool) (ret []byte, usedGas uint64, failed bool, err error, DofilterFlag bool) {
	dofilterFlag := false
	if multiRelayerFlag {
		if err = st.ppcPreCheck(relayerAccount); err != nil {
			return
		}
	} else if kickoutFlag {
		if err = st.ppcKickoutPreCheck(); err != nil {
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
		return nil, 0, false, err, dofilterFlag
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, err, dofilterFlag
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
				if multiRelayerFlag {
					st.state.SetNonce(*relayerAccount, st.state.GetNonce(*relayerAccount)+1)
				}
			}
		} else {
			// Increment the nonce for the next transaction
			if !kickoutFlag {
				st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			} else {
				st.state.SetNonce(txfilter.Bigguy, st.state.GetNonce(txfilter.Bigguy)+1)
			}
			//wenbin add,support multi-tx nonce
			if multiRelayerFlag {
				st.state.SetNonce(*relayerAccount, st.state.GetNonce(*relayerAccount)+1)
			}

			isBetTx, vmerr := txfilter.PPCDoFilter(msg.From(), *msg.To(), st.state.GetBalance(msg.From()), msg.Data(), st.evm.BlockNumber.Int64())
			if vmerr == nil && !isBetTx {
				ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
			} else if vmerr == nil && isBetTx {
				dofilterFlag = true
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
				if multiRelayerFlag {
					st.state.SetNonce(*relayerAccount, st.state.GetNonce(*relayerAccount)+1)
				}
			}
		} else {
			// Increment the nonce for the next transaction
			if !kickoutFlag {
				st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
			} else {
				st.state.SetNonce(txfilter.Bigguy, st.state.GetNonce(txfilter.Bigguy)+1)
			}

			//wenbin add,support multi-tx nonce
			if multiRelayerFlag {
				st.state.SetNonce(*relayerAccount, st.state.GetNonce(*relayerAccount)+1)
			}

			isBetTx := false
			isBetTx, vmerr = txfilter.PPCDoFilter(msg.From(), *msg.To(), st.state.GetBalance(msg.From()), msg.Data(), st.evm.BlockNumber.Int64())
			if vmerr == nil && !isBetTx {
				ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
			} else if vmerr == nil && isBetTx {
				dofilterFlag = true
			}
		}
	}

	if vmerr != nil {
		log.Info("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, false, vmerr, dofilterFlag
		}
	}
	if multiRelayerFlag {
		st.ppcRefundGas(relayerAccount)
	} else {
		st.refundGas()
	}

	st.state.AddBalance(txfilter.Bigguy, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	//If mintFlag is true,mint gas to Bigguy
	if mintFlag {
		st.state.AddBalance(txfilter.Bigguy, mintGasNumber)
	}

	gasAmount := new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)
	Gwei := new(big.Int).Div(gasAmount, new(big.Int).Mul(big.NewInt(1000), big.NewInt(1000000)))

	return ret, Gwei.Uint64(), vmerr != nil, err, dofilterFlag
}

func (st *StateTransition) ppcKickoutPreCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(txfilter.Bigguy)
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

func (st *StateTransition) ppcPreCheck(relayerAccount *common.Address) error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(st.msg.From())
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.ppcBuyGas(relayerAccount)
}

func (st *StateTransition) ppcBuyGas(relayerAccount *common.Address) error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if st.state.GetBalance(st.msg.From()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(*relayerAccount, mgval)
	return nil
}

func (st *StateTransition) ppcRefundGas(relayerAccount *common.Address) {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(*relayerAccount, remaining)

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

func BytesToString(b []byte) string {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{bh.Data, bh.Len}
	return *(*string)(unsafe.Pointer(&sh))
}

// rlp decode an etherum transaction
func PPCDecodeTx(txBytes []byte) (*types.Transaction, error) {
	tx := new(types.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}

func PPCIllegalRelayFrom(from, to common.Address, balance *big.Int, txDataBytes []byte, statedb *state.StateDB) (bool, error) {
	if txfilter.PPCTXCached == nil {
		txfilter.PPCTXCached = txfilter.NewPPCCachedTx()
	}
	txfilter.PPCTXCached.Mtx.Lock()
	defer txfilter.PPCTXCached.Mtx.Unlock()

	//relayTx is a flag whether the tx is relay
	relayTx := false
	var err error
	var subFrom common.Address
	tx, _ := PPCDecodeTx(txDataBytes)
	var hashStr string

	if txfilter.IsRelayAccount(to) {
		relayTx = true

		data := txDataBytes
		hashData := md5.Sum(data)
		hashStr = hex.EncodeToString(hashData[:])
		_, ok := txfilter.PPCTXCached.CachedTx[hashStr]
		if ok {
			subFrom = txfilter.PPCTXCached.CachedTx[hashStr]
		} else {

			var signer types.Signer = types.HomesteadSigner{}
			if tx.Protected() {
				signer = types.NewEIP155Signer(tx.ChainId())
			}
			// Make sure the transaction is signed properly
			subFrom, err = types.Sender(signer, tx)

			if err != nil {
				delete(txfilter.PPCTXCached.CachedTx, hashStr)
				return relayTx, err
			}
		}
		//allow bigger nonce come in
		nonce := statedb.GetNonce(subFrom)
		if nonce > tx.Nonce() {
			//If hashStr existed in cachedTx, removed
			//This may called by recheckTx
			delete(txfilter.PPCTXCached.CachedTx, hashStr)
			return relayTx, ErrNonceTooLow
		} else {
			// verified success
			txfilter.PPCTXCached.CachedTx[hashStr] = subFrom
		}

	}
	return relayTx, nil
}

func PPCIllegalForm(from, to common.Address, balance *big.Int, txDataBytes []byte, currHeight uint64, statedb *state.StateDB) (err error) {
	//verify whether the ppc-approved data is valid
	if txfilter.IsBigGuy(from) && txfilter.IsPPCCATableAccount(to) {
		_, err := txfilter.PPCUnMarshalTxData(txDataBytes)
		if err != nil {
			return err
		}
	} else if txfilter.IsLockTx(to) {
		value, isExisted := txfilter.PPCCATableCopy.PPCCATableItemMap[from]
		if isExisted {
			if !value.Used && (currHeight >= value.StartHeight) &&
				(currHeight <= value.EndHeight) {
				//approved bet tx,go ahead
				txData, err := txfilter.UnMarshalTxData(txDataBytes)
				if err != nil {
					return errors.New("inillegal format txdata")
				}
				txDataStr, _ := json.Marshal(txData)
				data := []byte(txDataStr)
				hashData := md5.Sum(data)
				hashStr := hex.EncodeToString(hashData[:])
				if hashStr != value.ApprovedTxDataHash {
					return errors.New("txData doesn't match approved txData")
				}

			} else {
				return errors.New("unmatched height for bet tx")
			}
		} else {
			return errors.New("bet tx doesn't exist in the ppccatable")
		}
	}
	return nil
}
