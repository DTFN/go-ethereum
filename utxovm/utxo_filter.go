package utxovm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var (
	UTXOToFlag = common.HexToAddress("0x1111111111111111111111111111111111111000")
)

func UTXOTxProc(msg types.Message, statedb *state.StateDB, config *params.ChainConfig,
	header *types.Header) *types.Receipt {
	statedb.SetNonce(msg.From(), msg.Nonce()+1)
	root := statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	receipt := types.NewReceipt(root, false, 0)
	return receipt
}
