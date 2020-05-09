// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/core/txfilter"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 0
	// rmTxChanSize is the size of channel listening to RemovedTransactionEvent.
	rmTxChanSize = 10

	cachedTxSize = 10000
)

var (
	ErrAlreadyKnown = errors.New("already known")
	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")

	ErrInvalidRelaySignature = errors.New("invalid relayer signature for relaytx")

	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")

	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = errors.New("transaction underpriced")

	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	// ErrBlockedSender is returned if the sender is some specific address or already bet for validators
	ErrBlockedSender = errors.New("locked sender")
)

var (
	evictionInterval    = time.Minute     // Time interval to check for evictable transactions
	statsReportInterval = 8 * time.Second // Time interval to report transaction pool stats
)

var (
	// Metrics for the pending pool
	pendingDiscardCounter   = metrics.NewRegisteredCounter("txpool/pending/discard", nil)
	pendingReplaceCounter   = metrics.NewRegisteredCounter("txpool/pending/replace", nil)
	pendingRateLimitCounter = metrics.NewRegisteredCounter("txpool/pending/ratelimit", nil) // Dropped due to rate limiting
	pendingNofundsCounter   = metrics.NewRegisteredCounter("txpool/pending/nofunds", nil)   // Dropped due to out-of-funds

	// Metrics for the queued pool
	queuedDiscardCounter   = metrics.NewRegisteredCounter("txpool/queued/discard", nil)
	queuedReplaceCounter   = metrics.NewRegisteredCounter("txpool/queued/replace", nil)
	queuedRateLimitCounter = metrics.NewRegisteredCounter("txpool/queued/ratelimit", nil) // Dropped due to rate limiting
	queuedNofundsCounter   = metrics.NewRegisteredCounter("txpool/queued/nofunds", nil)   // Dropped due to out-of-funds

	// General tx metrics
	invalidTxCounter     = metrics.NewRegisteredCounter("txpool/invalid", nil)
	underpricedTxCounter = metrics.NewRegisteredCounter("txpool/underpriced", nil)
)

// TxStatus is the current status of a transaction as seen by the pool.
type TxStatus uint

const (
	TxStatusUnknown  TxStatus = iota
	TxStatusQueued
	TxStatusPending
	TxStatusIncluded
)

// blockChain provides the state of blockchain and current gas limit to do
// some pre checks in tx pool and event subscribers.
type blockChain interface {
	CurrentBlock() *types.Block
	PendingBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)

	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
}

// TxPoolConfig are the configuration parameters of the transaction pool.
type TxPoolConfig struct {
	Locals    []common.Address // Addresses that should be treated by default as local
	NoLocals  bool             // Whether local transaction handling should be disabled
	Journal   string           // Journal of local transactions to survive node restarts
	Rejournal time.Duration    // Time interval to regenerate the local transaction journal

	PriceLimit uint64 // Minimum gas price to enforce for acceptance into the pool
	PriceBump  uint64 // Minimum price bump percentage to replace an already existing transaction (nonce)

	AccountSlots uint64 // Minimum number of executable transaction slots guaranteed per account
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	AccountQueue uint64 // Maximum number of non-executable transaction slots permitted per account
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts

	Lifetime time.Duration // Maximum amount of time non-executable transaction are queued

	LRUCacheSize          int
	MempoolSize           uint64 //not zero to indicate open flow control
	FlowLimitThreshold    uint64
	MaxFlowLimitSleepTime time.Duration
}

// DefaultTxPoolConfig contains the default configurations for the transaction
// pool.
var DefaultTxPoolConfig = TxPoolConfig{
	Journal:   "transactions.rlp",
	Rejournal: time.Hour,

	PriceLimit: 1,
	PriceBump:  10,

	AccountSlots: 32,
	GlobalSlots:  409600,
	AccountQueue: 128,
	GlobalQueue:  409600,

	Lifetime: 3 * time.Hour,

	MempoolSize:  0,
	LRUCacheSize: 256,
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *TxPoolConfig) sanitize() TxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		log.Warn("Sanitizing invalid txpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	if conf.PriceLimit < 1 {
		log.Warn("Sanitizing invalid txpool price limit", "provided", conf.PriceLimit, "updated", DefaultTxPoolConfig.PriceLimit)
		conf.PriceLimit = DefaultTxPoolConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		log.Warn("Sanitizing invalid txpool price bump", "provided", conf.PriceBump, "updated", DefaultTxPoolConfig.PriceBump)
		conf.PriceBump = DefaultTxPoolConfig.PriceBump
	}
	return conf
}

type TxCallback struct {
	tx     *types.Transaction
	local  bool
	result chan error
}

// TxPool contains all currently known transactions. Transactions
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are included in the blockchain.
//
// The pool separates processable transactions (which can be applied to the
// current state) and future transactions. Transactions move between those
// two states over time as they are received and processed.
type TxPool struct {
	config       TxPoolConfig
	chainconfig  *params.ChainConfig
	chain        blockChain
	gasPrice     *big.Int
	txFeed       event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan ChainHeadEvent
	chainHeadSub event.Subscription
	signer       types.Signer
	mu           sync.RWMutex

	currentState  *state.StateDB      // Current state in the blockchain head
	pendingState  *state.ManagedState // Pending state tracking virtual nonces
	currentMaxGas uint64              // Current gas limit for transaction caps

	locals  *accountSet // Set of local transaction to exempt from eviction rules
	journal *txJournal  // Journal of local transaction to back up to disk

	pending map[common.Address]*txList         // All currently processable transactions
	queue   map[common.Address]*txList         // Queued but non-processable transactions
	beats   map[common.Address]time.Time       // Last heartbeat from each known account
	all     map[common.Hash]*types.Transaction // All transactions to allow lookups
	priced  *txPricedList                      // All transactions sorted by price

	blockArrive       bool //true when loop() wants to get lock
	appConsumer       bool
	pendingTxPreEvent map[common.Hash]*TxPreEvent
	relayTxInfo       map[common.Hash]*RelayInfo
	flowLimit         bool
	hasCachedTxs      bool
	cachedTxs         chan TxCallback
	mempoolTxsLen     int

	wg sync.WaitGroup // for shutdown sync

	homestead bool
}

type RelayInfo struct {
	RelayFrom common.Address
	SubTx     *types.Transaction
}

// NewTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewTxPool(config TxPoolConfig, chainconfig *params.ChainConfig, chain blockChain) *TxPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// Create the transaction pool with its initial settings
	pool := &TxPool{
		config:            config,
		chainconfig:       chainconfig,
		chain:             chain,
		signer:            types.NewEIP155Signer(chainconfig.ChainID),
		pending:           make(map[common.Address]*txList),
		queue:             make(map[common.Address]*txList),
		beats:             make(map[common.Address]time.Time),
		all:               make(map[common.Hash]*types.Transaction),
		chainHeadCh:       make(chan ChainHeadEvent, chainHeadChanSize),
		cachedTxs:         make(chan TxCallback, cachedTxSize),
		pendingTxPreEvent: make(map[common.Hash]*TxPreEvent),
		relayTxInfo:       make(map[common.Hash]*RelayInfo),
		gasPrice:          new(big.Int).SetUint64(config.PriceLimit),
		flowLimit:         false,
		appConsumer:       false,
		hasCachedTxs:      false,
	}
	pool.locals = newAccountSet(pool.signer)
	pool.priced = newTxPricedList(&pool.all)
	pool.reset(nil, chain.CurrentBlock().Header())

	// If local transactions and journaling is enabled, load from disk
	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(config.Journal)

		if err := pool.journal.load(pool.AddLocalsInit); err != nil { //don't call addTx because PosTable has not init yet
			log.Warn("Failed to load transaction journal", "err", err)
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			log.Warn("Failed to rotate transaction journal", "err", err)
		}
	}
	// Subscribe events from blockchain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

func (pool *TxPool) GetTxpoolChainHeadSize() int {
	return len(pool.chainHeadCh)
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *TxPool) loop() {
	defer pool.wg.Done()

	// Start the stats reporting and transaction eviction tickers
	var prevPending, prevQueued, prevStales int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// Track the previous head headers for transaction reorgs
	head := pool.chain.CurrentBlock()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			if ev.Block != nil {
				pool.blockArrive = true
				pool.mu.Lock()
				pool.blockArrive = false
				if pool.chainconfig.IsHomestead(ev.Block.Number()) {
					pool.homestead = true
				}
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block
				pool.mu.Unlock()
			}
			// Be unsubscribed due to system stopped
		case err := <-pool.chainHeadSub.Err():
			log.Error("Pool.chainHeadSub.Err!", "err", err)
			return

			// Handle stats reporting ticks
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			stales := pool.priced.stales
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued || stales != prevStales {
				log.Debug("Transaction pool status report", "executable", pending, "queued", queued, "stales", stales)
				prevPending, prevQueued, prevStales = pending, queued, stales
			}

			// Handle inactive account transaction eviction
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// Skip local transactions from the eviction mechanism
				if pool.locals.contains(addr) {
					continue
				}
				// Any non-locals old enough should be removed
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					for _, tx := range pool.queue[addr].Flatten() {
						pool.removeTx(tx.Hash(), true)
					}
				}
			}
			pool.mu.Unlock()

			// Handle local transaction journal rotation
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					log.Warn("Failed to rotate local tx journal", "err", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}

func (pool *TxPool) CopyPendingState() *state.StateDB {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pendingState := state.ManageState(pool.currentState)

	for addr, list := range pool.pending {
		txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
		pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
	return pendingState.StateDB
}

func (pool *TxPool) CopyPendingStateToCurPos() *state.StateDB {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pendingState := state.ManageState(pool.currentState)

	for addr, list := range pool.pending {
		txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
		pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
	return pendingState.StateDB
}

// lockedReset is a wrapper around reset to allow calling it in a thread safe
// manner. This method is only ever used in the tester!
func (pool *TxPool) lockedReset(oldHead, newHead *types.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(oldHead, newHead)
}

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
func (pool *TxPool) reset(oldHead, newHead *types.Header) {
	// If we're reorging an old state, reinject all dropped transactions
	var reinject types.Transactions

	if oldHead != nil && oldHead.Hash() != newHead.ParentHash {
		// If the reorg is too deep, avoid doing it (will happen during fast sync)
		oldNum := oldHead.Number.Uint64()
		newNum := newHead.Number.Uint64()

		if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
			log.Debug("Skipping deep transaction reorg", "depth", depth)
		} else {
			// Reorg seems shallow enough to pull in all transactions into memory
			var discarded, included types.Transactions

			var (
				rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Number.Uint64())
				add = pool.chain.GetBlock(newHead.Hash(), newHead.Number.Uint64())
			)
			for rem.NumberU64() > add.NumberU64() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
					return
				}
			}
			for add.NumberU64() > rem.NumberU64() {
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
					return
				}
			}
			for rem.Hash() != add.Hash() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
					return
				}
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
					return
				}
			}
			reinject = types.TxDifference(discarded, included)
		}
	}
	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // Special case during testing
	}
	statedb, err := pool.chain.StateAt(newHead.Root)
	if err != nil {
		log.Error("Failed to reset txpool state", "err", err)
		return
	}
	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)
	if pool.currentMaxGas == 0 {
		pool.currentMaxGas = newHead.GasLimit
	}

	// Inject any transactions discarded due to reorgs
	log.Debug("Reinjecting stale transactions", "count", len(reinject))
	pool.addTxsLocked(reinject, false)

	// validate the pool of pending transactions, this will remove
	// any transactions that have been included in the block or
	// have been invalidated because of another transaction (e.g.
	// higher gas price)
	pool.demoteUnexecutables()

	// Update all accounts to the latest known pending nonce
	for addr, list := range pool.pending {
		txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
		pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
	// Check the queue and move transactions over to the pending if possible
	// or remove those that have become invalid
	pool.promoteExecutables(nil)
}

// Stop terminates the transaction pool.
func (pool *TxPool) Stop() {
	// Unsubscribe all subscriptions registered from txpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	log.Info("Transaction pool stopped")
}

// SubscribeTxPreEvent registers a subscription of TxPreEvent and
// starts sending event to the given channel.
func (pool *TxPool) SubscribeTxPreEvent(ch chan<- TxPreEvent) event.Subscription {
	return pool.scope.Track(pool.txFeed.Subscribe(ch))
}

// GasPrice returns the current gas price enforced by the transaction pool.
func (pool *TxPool) GasPrice() *big.Int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return new(big.Int).Set(pool.gasPrice)
}

// SetGasPrice updates the minimum price required by the transaction pool for a
// new transaction, and drops all transactions below this threshold.
func (pool *TxPool) SetGasPrice(price *big.Int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.gasPrice = price
	for _, tx := range pool.priced.Cap(price, pool.locals) {
		pool.removeTx(tx.Hash(), false)
	}
	log.Info("Transaction pool price threshold updated", "price", price)
}

// State returns the virtual managed state of the transaction pool.
func (pool *TxPool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

// Nonce returns the next nonce of an account, with all transactions executable
// by the pool already applied on top.
func (pool *TxPool) Nonce(addr common.Address) uint64 {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState.GetNonce(addr)
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

// Content retrieves the data content of the transaction pool, returning all the
// pending as well as queued transactions, grouped by account and sorted by nonce.
func (pool *TxPool) Content() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[common.Address]types.Transactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

// Pending retrieves all currently processable transactions, groupped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Pending() (map[common.Address]types.Transactions, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	return pending, nil
}

// local retrieves all currently known local transactions, groupped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) local() map[common.Address]types.Transactions {
	txs := make(map[common.Address]types.Transactions)
	for addr := range pool.locals.accounts {
		if pending := pool.pending[addr]; pending != nil {
			txs[addr] = append(txs[addr], pending.Flatten()...)
		}
		if queued := pool.queue[addr]; queued != nil {
			txs[addr] = append(txs[addr], queued.Flatten()...)
		}
	}
	return txs
}

func (pool *TxPool) Locals() []common.Address {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.locals.flatten()
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxPool) validateTx(tx *types.Transaction, local bool) error {
	// If the transaction fails basic validation, discard it
	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}
	// Ensure the transaction doesn't exceed the current block limit gas.
	if pool.currentMaxGas < tx.Gas() {
		return ErrGasLimit
	}
	to := tx.To()
	var from, balanceCheckAddress common.Address
	var signer types.Signer = types.HomesteadSigner{}
	if tx.Protected() {
		signer = pool.signer
	}
	if to != nil {
		if txfilter.IsRelayTxFromRelayer(*to) {
			subTx, err := types.DecodeTxFromHexBytes(tx.Data())
			if err != nil {
				return err
			}
			err = types.CheckRelayerTx(tx, subTx)
			if err != nil {
				return err
			}
			txForVerify, err := subTx.WithVRS(tx.RawSignatureValues())
			if err != nil {
				return err
			}
			// Make sure the transaction is signed properly
			from, err = types.Sender(signer, txForVerify)
			if err != nil {
				return ErrInvalidSender
			}
			tx.SetFrom(signer, from)
			relayer, err := types.DeriveRelayer(from, subTx)
			if err != nil {
				return err
			}
			fmt.Printf("txPool receives a relay tx from relayer. client %X relayer %X \n ", from, relayer)
			fmt.Printf("tx %v \n subTx %v \n ", tx, subTx)
			pool.relayTxInfo[tx.Hash()] = &RelayInfo{SubTx: subTx, RelayFrom: relayer}
			balanceCheckAddress = relayer
		} else {
			var err error
			// Make sure the transaction is signed properly
			from, err = types.Sender(signer, tx)
			if err != nil {
				return ErrInvalidSender
			}
			if txfilter.IsRelayTxFromClient(*to) {
				subTx, err := types.DecodeTxFromHexBytes(tx.Data())
				if err != nil {
					return err
				}
				err = types.CheckRelayerTx(tx, subTx)
				if err != nil {
					return err
				}
				relayer, err := types.DeriveRelayer(from, subTx)
				if err != nil {
					return err
				}
				fmt.Printf("txPool receives a relay tx from client. client %X relayer %X \n ", from, relayer)
				fmt.Printf("tx %v \n subTx %v \n ", tx, subTx)
				pool.relayTxInfo[tx.Hash()] = &RelayInfo{SubTx: subTx, RelayFrom: relayer}
				balanceCheckAddress = relayer
			} else if txfilter.IsMintTx(*to) {
				err := txfilter.IsMintBlocked(from)
				if err != nil {
					return err
				}
				balanceCheckAddress = from
			} else {
				balanceCheckAddress = from
			}
		}
	} else {
		var err error
		// Make sure the transaction is signed properly
		from, err = types.Sender(signer, tx)
		if err != nil {
			return ErrInvalidSender
		}
		balanceCheckAddress = from
	}

	// Drop non-local transactions under our own minimal accepted gas price
	local = local || pool.locals.contains(from) // account may be local even if the transaction arrived from the network
	if pool.gasPrice.Cmp(tx.GasPrice()) > 0 { //leilei comment local to set tx.GasPrice() as a barrier for all the cases
		return ErrUnderpriced
	}
	// Ensure the transaction adheres to nonce ordering
	if pool.currentState.GetNonce(from) > tx.Nonce() {
		return ErrNonceTooLow
	}
	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	if pool.currentState.GetBalance(balanceCheckAddress).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}
	intrGas, err := IntrinsicGas(tx.Data(), tx.To() == nil, pool.homestead)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}
	return nil
}

func (pool *TxPool) ValidateExist(hash common.Hash) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	// If the transaction is already known, return true
	if pool.all[hash] != nil {
		return true
	} else {
		return false
	}
}

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
//
// If a newly added transaction is marked as local, its sending account will be
// whitelisted, preventing any associated transaction from being dropped out of
// the pool due to pricing constraints.
func (pool *TxPool) add(tx *types.Transaction, local bool) (bool, error) {
	// If the transaction is already known, discard it
	hash := tx.Hash()
	if pool.all[hash] != nil {
		log.Trace("Discarding already known transaction", "hash", hash)
		return false, ErrAlreadyKnown
	}
	// If the transaction fails basic validation, discard it
	if err := pool.validateTx(tx, local); err != nil {
		log.Trace("Discarding invalid transaction", "hash", hash, "err", err)
		invalidTxCounter.Inc(1)
		return false, err
	}
	// If the transaction pool is full, discard underpriced transactions
	if uint64(len(pool.all)) >= pool.config.GlobalSlots+pool.config.GlobalQueue {
		// If the new transaction is underpriced, don't accept it
		if pool.priced.Underpriced(tx, pool.locals) {
			log.Trace("Discarding underpriced transaction", "hash", hash, "price", tx.GasPrice())
			underpricedTxCounter.Inc(1)
			return false, ErrUnderpriced
		}
		// New transaction is better than our worse ones, make room for it
		drop := pool.priced.Discard(len(pool.all)-int(pool.config.GlobalSlots+pool.config.GlobalQueue-1), pool.locals)
		for _, tx := range drop {
			log.Trace("Discarding freshly underpriced transaction", "hash", tx.Hash(), "price", tx.GasPrice())
			underpricedTxCounter.Inc(1)
			pool.removeTx(tx.Hash(), false)
		}
	}
	var signer types.Signer = types.HomesteadSigner{}
	if tx.Protected() {
		signer = pool.signer
	}
	// If the transaction is replacing an already pending one, do directly
	from, _ := types.Sender(signer, tx) // already validated
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
		// Nonce already pending, check if required price bump is met
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			pendingDiscardCounter.Inc(1)
			return false, ErrReplaceUnderpriced
		}
		// New transaction is better, replace old one
		if old != nil {
			delete(pool.all, old.Hash())
			delete(pool.relayTxInfo, old.Hash())
			pool.priced.Removed()
			pendingReplaceCounter.Inc(1)
		}
		pool.all[tx.Hash()] = tx
		pool.priced.Put(tx)
		pool.journalTx(from, tx)

		log.Trace("Pooled new executable transaction", "hash", hash, "from", from, "to", tx.To())

		// We've directly injected a replacement transaction, notify subsystems
		//pool.txFeed.Send(TxPreEvent{tx}) //leilei delete go routine call for ethermint checkTx

		return old != nil, nil
	}
	// New transaction isn't replacing a pending one, push into queue
	replace, err := pool.enqueueTx(hash, tx)
	if err != nil {
		return false, err
	}
	// Mark local addresses and journal local transactions
	if local {
		pool.locals.add(from)
	}
	pool.journalTx(from, tx)

	log.Trace("Pooled new future transaction", "hash", hash, "from", from, "to", tx.To())
	return replace, nil
}

// enqueueTx inserts a new transaction into the non-executable transaction queue.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) enqueueTx(hash common.Hash, tx *types.Transaction) (bool, error) {
	var signer types.Signer = types.HomesteadSigner{}
	if tx.Protected() {
		signer = pool.signer
	}
	// Try to insert the transaction into the future queue
	from, _ := types.Sender(signer, tx) // already validated
	if pool.queue[from] == nil {
		pool.queue[from] = newTxList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		queuedDiscardCounter.Inc(1)
		return false, ErrReplaceUnderpriced
	}
	// Discard any previous transaction and mark this
	if old != nil {
		delete(pool.all, old.Hash())
		delete(pool.relayTxInfo, old.Hash())
		pool.priced.Removed()
		queuedReplaceCounter.Inc(1)
	}
	if pool.all[hash] == nil {
		pool.all[hash] = tx
		pool.priced.Put(tx)
	}
	return old != nil, nil
}

// journalTx adds the specified transaction to the local disk journal if it is
// deemed to have been sent from a local account.
func (pool *TxPool) journalTx(from common.Address, tx *types.Transaction) {
	// Only journal if it's enabled and the transaction is local
	if pool.journal == nil || !pool.locals.contains(from) {
		return
	}
	if err := pool.journal.insert(tx); err != nil {
		log.Warn("Failed to journal local transaction", "err", err)
	}
}

// promoteTx adds a transaction to the pending (processable) list of transactions.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) promoteTx(addr common.Address, hash common.Hash, tx *types.Transaction) {
	// Try to insert the transaction into the pending queue
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		delete(pool.all, hash)
		delete(pool.relayTxInfo, hash)
		pool.priced.Removed()

		pendingDiscardCounter.Inc(1)
		return
	}
	// Otherwise discard any previous transaction and mark this
	if old != nil {
		delete(pool.all, old.Hash())
		delete(pool.relayTxInfo, old.Hash())
		pool.priced.Removed()

		pendingReplaceCounter.Inc(1)
	}
	// Failsafe to work around direct pending inserts (tests)
	if pool.all[hash] == nil {
		pool.all[hash] = tx
		pool.priced.Put(tx)
	}
	// Set the potentially new pending nonce and notify any subsystems of the new tx
	pool.beats[addr] = time.Now()
	pool.pendingState.SetNonce(addr, tx.Nonce()+1)

	txPreEvent, ok := pool.pendingTxPreEvent[tx.Hash()]
	if !ok {
		txPreEvent = &TxPreEvent{}
		txPreEvent.Result = make(chan error, 1)
	} else if txPreEvent.Result == nil {
		txPreEvent.Result = make(chan error, 1)
	}
	txPreEvent.From = addr
	txPreEvent.Tx = tx

	if relayInfo, ok := pool.relayTxInfo[tx.Hash()]; ok {
		txPreEvent.SubTx = relayInfo.SubTx
		txPreEvent.Relayer = relayInfo.RelayFrom
	}

	pool.txFeed.Send(*txPreEvent) //leilei delete go routine call for ethermint checkTx
}

// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *TxPool) AddLocal(tx *types.Transaction) error {
	return pool.addTx(tx, !pool.config.NoLocals)
}

// AddRemote enqueues a single transaction into the pool if it is valid. If the
// sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *TxPool) AddRemote(tx *types.Transaction) error {
	return pool.addTx(tx, false)
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *TxPool) AddLocals(txs []*types.Transaction) []error {
	return pool.addTxs(txs, !pool.config.NoLocals)
}

// AddRemotes enqueues a batch of transactions into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *TxPool) AddRemotes(txs []*types.Transaction) []error {
	return pool.addTxs(txs, false)
}

// AddLocalsInit is called in NewTxPool
func (pool *TxPool) AddLocalsInit(txs []*types.Transaction) (errs []error) {
	errs = make([]error, len(txs))
	pool.mu.Lock()
	defer pool.mu.Unlock()

	fmt.Printf("TxPool replay journals begin \n")
	for i, tx := range txs {
		if i >= cachedTxSize {
			errs[i] = fmt.Errorf("tx count %v excceeds cachedTxSize %v", i, cachedTxSize)
			fmt.Printf("tx count %v excceeds cachedTxSize %v", i, cachedTxSize)
			continue
		}
		pool.cachedTxs <- TxCallback{tx, true, nil}
	}
	fmt.Printf("TxPool replay journals end \n")
	return
}

func (pool *TxPool) IsFlowControlOpen() bool {
	return pool.config.MempoolSize > 0
}

func (pool *TxPool) SetFlowLimit(txsLen int) {
	if pool.config.MempoolSize <= 0 { //flow control is not open
		return
	}
	txsLen64 := uint64(txsLen)
	pool.mempoolTxsLen = txsLen
	if txsLen64 < pool.config.FlowLimitThreshold {
		pool.flowLimit = false
	} else {
		pool.flowLimit = true
	}
}

func (pool *TxPool) flowLimitHandle() {
	mempoolTxsLen := uint64(pool.mempoolTxsLen)
	if mempoolTxsLen >= pool.config.MempoolSize {
		log.Info("TxPool flowLimit trigger due to mempool full", "mempoolTxsLen", mempoolTxsLen, "sleepTime", pool.config.MaxFlowLimitSleepTime)
		time.Sleep(pool.config.MaxFlowLimitSleepTime)
	} else if mempoolTxsLen > pool.config.FlowLimitThreshold {
		sleepTime := time.Duration(float64(mempoolTxsLen-pool.config.FlowLimitThreshold) / float64(pool.config.MempoolSize-pool.config.FlowLimitThreshold) * float64(pool.config.MaxFlowLimitSleepTime))
		log.Info("TxPool flowLimit trigger due to mempool exceeds limit", "mempoolTxsLen", mempoolTxsLen, "sleepTime", sleepTime)
		time.Sleep(sleepTime)
	}
}

func (pool *TxPool) BeginConsume() {
	pool.appConsumer = true
}

func (pool *TxPool) HandleCachedTxs() {
	if !pool.appConsumer { //in replay, tendermint not fully started yet
		return
	}
	pool.mu.Lock()
	close(pool.cachedTxs) //this func is called after app.Commit, blockArrive should be false, no routine puts tx into cachedTxs directly, so it is safe to close it

	for txCallback := range pool.cachedTxs {
		if txCallback.tx == nil {
			panic("txCallback.tx cannot be nil")
		}
		// Try to inject the transaction and update any state
		replace, err := pool.add(txCallback.tx, txCallback.local)
		if err != nil {
			txCallback.result <- err
			continue
		}
		txPreEvent := &TxPreEvent{}
		// If we added a new transaction, run promotion checks and return
		if !replace {
			txPreEvent.Result = txCallback.result
			pool.pendingTxPreEvent[txCallback.tx.Hash()] = txPreEvent
			var signer types.Signer = types.HomesteadSigner{}
			if txCallback.tx.Protected() {
				signer = pool.signer
			}
			from, _ := types.Sender(signer, txCallback.tx) // already validated
			pool.promoteExecutables([]common.Address{from})
			delete(pool.pendingTxPreEvent, txCallback.tx.Hash())
		}
		if txPreEvent.Tx == nil { //has been put into pending
			txCallback.result <- err
		}
	}
	pool.cachedTxs = make(chan TxCallback, cachedTxSize)
	pool.hasCachedTxs = false
	pool.mu.Unlock()
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx *types.Transaction, local bool) error {
	if pool.flowLimit {
		pool.flowLimitHandle()
	}
	if pool.blockArrive {
		callback := make(chan error, 1)
		pool.cachedTxs <- TxCallback{tx, local, callback}
		pool.hasCachedTxs = true
		err := <-callback
		close(callback)
		return err
	}
	pool.mu.Lock()
	if pool.hasCachedTxs {
		callback := make(chan error, 1)
		pool.cachedTxs <- TxCallback{tx, local, callback}
		pool.mu.Unlock()
		err := <-callback
		close(callback)
		return err
	}
	// Try to inject the transaction and update any state
	replace, err := pool.add(tx, local)
	if err != nil {
		pool.mu.Unlock()
		return err
	}
	txPreEvent := &TxPreEvent{}
	pool.pendingTxPreEvent[tx.Hash()] = txPreEvent
	// If we added a new transaction, run promotion checks and return
	if !replace {
		var signer types.Signer = types.HomesteadSigner{}
		if tx.Protected() {
			signer = pool.signer
		}
		from, _ := types.Sender(signer, tx) // already validated
		pool.promoteExecutables([]common.Address{from})
	}
	delete(pool.pendingTxPreEvent, tx.Hash())
	if txPreEvent.Result == nil { //put into queue but not into pending
		pool.mu.Unlock()
		return nil
	}
	pool.mu.Unlock()

	err = <-txPreEvent.Result
	return err
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs []*types.Transaction, local bool) []error {
	if pool.flowLimit {
		pool.flowLimitHandle()
	}
	if pool.blockArrive {
		errs := make([]error, len(txs))
		callbacks := make([]chan error, len(txs))
		for i, tx := range txs {
			callback := make(chan error, 1)
			pool.cachedTxs <- TxCallback{tx, local, callback}
			callbacks[i] = callback
		}
		pool.hasCachedTxs = true
		for i, callback := range callbacks {
			errs[i] = <-callback
			close(callback)
		}
		return errs
	}
	pool.mu.Lock()
	if pool.hasCachedTxs {
		errs := make([]error, len(txs))
		callbacks := make([]chan error, 0)
		for i, tx := range txs {
			callback := make(chan error, 1)
			pool.cachedTxs <- TxCallback{tx, local, callback}
			callbacks[i] = callback
		}
		pool.mu.Unlock()
		for i, callback := range callbacks {
			errs[i] = <-callback
			close(callback)
		}
		return errs
	}
	errs := pool.addTxsLocked(txs, local)
	results := make([]chan error, len(txs))
	for i, tx := range txs {

		if errs[i] == nil {
			if txPreEvent, ok := pool.pendingTxPreEvent[tx.Hash()]; ok {
				if txPreEvent.Result != nil {
					results[i] = txPreEvent.Result
					continue
				} else {
					results[i] = make(chan error, 1)
					close(results[i])
				}
				delete(pool.pendingTxPreEvent, tx.Hash())
			}
		} else {
			results[i] = make(chan error, 1)
			results[i] <- errs[i]
		}
	}
	pool.mu.Unlock()

	for i, result := range results {
		err := <-result
		fmt.Printf("batch tx %v done. result: %v", i, err)
		errs[i] = err
	}

	return errs
}

// addTxsLocked attempts to queue a batch of transactions if they are valid,
// whilst assuming the transaction pool lock is already held.
func (pool *TxPool) addTxsLocked(txs []*types.Transaction, local bool) []error {
	// Add the batch of transaction, tracking the accepted ones
	dirty := make(map[common.Address]struct{})
	errs := make([]error, len(txs))

	for i, tx := range txs {
		var replace bool
		if replace, errs[i] = pool.add(tx, local); errs[i] == nil {
			if !replace {
				var signer types.Signer = types.HomesteadSigner{}
				if tx.Protected() {
					signer = pool.signer
				}
				from, _ := types.Sender(signer, tx) // already validated
				dirty[from] = struct{}{}
				txPreEvent := &TxPreEvent{}
				pool.pendingTxPreEvent[tx.Hash()] = txPreEvent
			}
		}
	}
	// Only reprocess the internal state if something was actually added
	if len(dirty) > 0 {
		addrs := make([]common.Address, 0, len(dirty))
		for addr := range dirty {
			addrs = append(addrs, addr)
		}
		pool.promoteExecutables(addrs)
	}

	return errs
}

// Status returns the status (unknown/pending/queued) of a batch of transactions
// identified by their hashes.
func (pool *TxPool) Status(hashes []common.Hash) []TxStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]TxStatus, len(hashes))
	for i, hash := range hashes {
		if tx := pool.all[hash]; tx != nil {
			var signer types.Signer = types.HomesteadSigner{}
			if tx.Protected() {
				signer = pool.signer
			}
			from, _ := types.Sender(signer, tx) // already validated
			if pool.pending[from] != nil && pool.pending[from].txs.items[tx.Nonce()] != nil {
				status[i] = TxStatusPending
			} else {
				status[i] = TxStatusQueued
			}
		}
	}
	return status
}

// Get returns a transaction if it is contained in the pool
// and nil otherwise.
func (pool *TxPool) Get(hash common.Hash) *types.Transaction {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.all[hash]
}

func (pool *TxPool) Has(hash common.Hash) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	_, ok := pool.all[hash]
	return ok
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
func (pool *TxPool) removeTx(hash common.Hash, outofbound bool) {
	// Fetch the transaction we wish to delete
	tx, ok := pool.all[hash]
	if !ok {
		fmt.Printf("TxPool: remove tx %X not exist! \n", hash)
		return
	}
	var signer types.Signer = types.HomesteadSigner{}
	if tx.Protected() {
		signer = pool.signer
	}
	addr, _ := types.Sender(signer, tx) // already validated during insertion
	fmt.Printf("TxPool: remove tx %X for addr %X \n", hash, addr)
	// Remove it from the list of known transactions
	delete(pool.all, hash)
	delete(pool.relayTxInfo, hash)
	if outofbound {
		pool.priced.Removed()
	}

	// Remove the transaction from the pending lists and reset the account nonce
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			// If no more pending transactions are left, remove the list
			if pending.Empty() {
				delete(pool.pending, addr)
				delete(pool.beats, addr)
			}
			// Postpone any invalidated transactions
			for _, tx := range invalids {
				pool.enqueueTx(tx.Hash(), tx)
			}
			// Update the account nonce if needed
			pendingNonce := pool.pendingState.GetNonce(addr)
			if nonce := tx.Nonce(); pendingNonce > nonce {
				pool.pendingState.SetNonce(addr, nonce)
				fmt.Printf("TxPool: addr %X nonce %v decreases to %v \n", addr, pendingNonce, nonce)
			}
			return
		}
	}
	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

func (pool *TxPool) RemoveTxs(hashes []common.Hash) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for _, hash := range hashes {
		pool.removeTx(hash, true)
	}
}

func (pool *TxPool) RemoveTx(hash common.Hash) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.removeTx(hash, true)
}

// promoteExecutables moves transactions that have become processable from the
// future queue to the set of pending transactions. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
func (pool *TxPool) promoteExecutables(accounts []common.Address) {
	// Gather all the accounts potentially needing updates
	if accounts == nil {
		accounts = make([]common.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			accounts = append(accounts, addr)
		}
	}
	// Iterate over all accounts and promote any executable transactions
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
			continue // Just in case someone calls with a non existing account
		}
		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(pool.currentState.GetNonce(addr)) {
			hash := tx.Hash()
			log.Trace("Removed old queued transaction", "hash", hash)
			delete(pool.all, hash)
			delete(pool.relayTxInfo, hash)
			pool.priced.Removed()
		}
		// Drop all transactions that are too costly (low balance or out of gas)
		drops, _ := list.Filter_relay(pool.currentState.GetBalance(addr), pool, pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			log.Trace("Removed unpayable queued transaction", "hash", hash)
			delete(pool.all, hash)
			delete(pool.relayTxInfo, hash)
			pool.priced.Removed()
			queuedNofundsCounter.Inc(1)
		}

		// Gather all executable transactions and promote them
		for _, tx := range list.Ready(pool.pendingState.GetNonce(addr)) {
			hash := tx.Hash()
			log.Trace("Promoting queued transaction", "hash", hash)
			pool.promoteTx(addr, hash, tx)
		}
		// Drop all transactions over the allowed limit
		if !pool.locals.contains(addr) {
			for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
				hash := tx.Hash()
				delete(pool.all, hash)
				delete(pool.relayTxInfo, hash)
				pool.priced.Removed()
				queuedRateLimitCounter.Inc(1)
				log.Trace("Removed cap-exceeding queued transaction", "hash", hash)
			}
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.queue, addr)
		}
	}
	// If the pending limit is overflown, start equalizing allowances
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	if pending > pool.config.GlobalSlots {
		pendingBeforeCap := pending
		// Assemble a spam order to penalize large transactors first
		spammers := prque.New()
		for addr, list := range pool.pending {
			// Only evict transactions from high rollers
			if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
				spammers.Push(addr, float32(list.Len()))
			}
		}
		// Gradually drop transactions from offenders
		offenders := []common.Address{}
		for pending > pool.config.GlobalSlots && !spammers.Empty() {
			// Retrieve the next offender if not local address
			offender, _ := spammers.Pop()
			offenders = append(offenders, offender.(common.Address))

			// Equalize balances until all the same or below threshold
			if len(offenders) > 1 {
				// Calculate the equalization threshold for all current offenders
				threshold := pool.pending[offender.(common.Address)].Len()

				// Iteratively reduce all offenders until below limit or threshold reached
				for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
					for i := 0; i < len(offenders)-1; i++ {
						list := pool.pending[offenders[i]]
						for _, tx := range list.Cap(list.Len() - 1) {
							// Drop the transaction from the global pools too
							hash := tx.Hash()
							delete(pool.all, hash)
							delete(pool.relayTxInfo, hash)
							pool.priced.Removed()

							// Update the account nonce to the dropped transaction
							if nonce := tx.Nonce(); pool.pendingState.GetNonce(offenders[i]) > nonce {
								pool.pendingState.SetNonce(offenders[i], nonce)
							}
							log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
						}
						pending--
					}
				}
			}
		}
		// If still above threshold, reduce to limit or min allowance
		if pending > pool.config.GlobalSlots && len(offenders) > 0 {
			for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
				for _, addr := range offenders {
					list := pool.pending[addr]
					for _, tx := range list.Cap(list.Len() - 1) {
						// Drop the transaction from the global pools too
						hash := tx.Hash()
						delete(pool.all, hash)
						delete(pool.relayTxInfo, hash)
						pool.priced.Removed()

						// Update the account nonce to the dropped transaction
						if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
							pool.pendingState.SetNonce(addr, nonce)
						}
						log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
					}
					pending--
				}
			}
		}
		pendingRateLimitCounter.Inc(int64(pendingBeforeCap - pending))
	}
	// If we've queued more transactions than the hard limit, drop oldest ones
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.config.GlobalQueue {
		// Sort all accounts with queued transactions by heartbeat
		addresses := make(addresssByHeartbeat, 0, len(pool.queue))
		for addr := range pool.queue {
			if !pool.locals.contains(addr) { // don't drop locals
				addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
			}
		}
		sort.Sort(addresses)

		// Drop transactions until the total is below the limit or only locals remain
		for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
			addr := addresses[len(addresses)-1]
			list := pool.queue[addr.address]

			addresses = addresses[:len(addresses)-1]

			// Drop all transactions if they are less than the overflow
			if size := uint64(list.Len()); size <= drop {
				for _, tx := range list.Flatten() {
					pool.removeTx(tx.Hash(), true)
				}
				drop -= size
				queuedRateLimitCounter.Inc(int64(size))
				continue
			}
			// Otherwise drop only last few transactions
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				pool.removeTx(txs[i].Hash(), true)
				drop--
				queuedRateLimitCounter.Inc(1)
			}
		}
	}
}

// demoteUnexecutables removes invalid and processed transactions from the pools
// executable/pending queue and any subsequent transactions that become unexecutable
// are moved back into the future queue.
func (pool *TxPool) demoteUnexecutables() {
	// Iterate over all accounts and demote any non-executable transactions
	for addr, list := range pool.pending {
		nonce := pool.currentState.GetNonce(addr)

		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			log.Trace("Removed old pending transaction", "hash", hash)
			delete(pool.all, hash)
			delete(pool.relayTxInfo, hash)
			pool.priced.Removed()
		}
		// Drop all transactions that are too costly (low balance or out of gas), and queue any invalids back for later
		drops, invalids := list.Filter_relay(pool.currentState.GetBalance(addr), pool, pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			log.Trace("Removed unpayable pending transaction", "hash", hash)
			delete(pool.all, hash)
			delete(pool.relayTxInfo, hash)
			pool.priced.Removed()
			pendingNofundsCounter.Inc(1)
		}
		for _, tx := range invalids {
			hash := tx.Hash()
			log.Trace("Demoting pending transaction", "hash", hash)
			pool.enqueueTx(hash, tx)
		}
		// If there's a gap in front, warn (should never happen) and postpone all transactions
		if list.Len() > 0 && list.txs.Get(nonce) == nil {
			for _, tx := range list.Cap(0) {
				hash := tx.Hash()
				log.Error("Demoting invalidated transaction", "hash", hash)
				pool.enqueueTx(hash, tx)
			}
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.pending, addr)
			delete(pool.beats, addr)
		}
	}
}

/*func (pool *TxPool) DebugMeomory(h *memsizeui.Handler) {
	h.Add("txPool", pool)
	h.Add("pending", &pool.pending)
	h.Add("all", &pool.all)
	h.Add("pendingTxPreEvent", &pool.pendingTxPreEvent)
	h.Add("priced", pool.priced)
	h.Add("queue", &pool.queue)
	h.Add("locals", pool.locals)
	h.Add("currentState", pool.currentState)
	h.Add("pendingState", pool.pendingState)
	h.Add("beats", &pool.beats)
	h.Add("journal", pool.journal)
	h.Add("chain", &pool.chain)
	h.Add("chainHeadCh", &pool.chainHeadCh)
	if blockchain1, ok := pool.chain.(*BlockChain); ok {
		blockchain1.DebugMeomory(h)
	}
}*/

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addresssByHeartbeat []addressByHeartbeat

func (a addresssByHeartbeat) Len() int           { return len(a) }
func (a addresssByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addresssByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence, and a signer
// capable of deriving addresses from transactions.
type accountSet struct {
	accounts map[common.Address]struct{}
	signer   types.Signer
	cache    *[]common.Address
}

// newAccountSet creates a new address set with an associated signer for sender
// derivations.
func newAccountSet(signer types.Signer, addrs ...common.Address) *accountSet {
	as := &accountSet{
		accounts: make(map[common.Address]struct{}),
		signer:   signer,
	}
	for _, addr := range addrs {
		as.add(addr)
	}
	return as
}

// contains checks if a given address is contained within the set.
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

// containsTx checks if the sender of a given tx is within the set. If the sender
// cannot be derived, this method returns false.
func (as *accountSet) containsTx(tx *types.Transaction) bool {
	if addr, err := types.Sender(as.signer, tx); err == nil {
		return as.contains(addr)
	}
	return false
}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
	as.cache = nil
}

// addTx adds the sender of tx into the set.
func (as *accountSet) addTx(tx *types.Transaction) {
	if addr, err := types.Sender(as.signer, tx); err == nil {
		as.add(addr)
	}
}

// flatten returns the list of addresses within this set, also caching it for later
// reuse. The returned slice should not be changed!
func (as *accountSet) flatten() []common.Address {
	if as.cache == nil {
		accounts := make([]common.Address, 0, len(as.accounts))
		for account := range as.accounts {
			accounts = append(accounts, account)
		}
		as.cache = &accounts
	}
	return *as.cache
}

// merge adds all addresses from the 'other' set into 'as'.
func (as *accountSet) merge(other *accountSet) {
	for addr := range other.accounts {
		as.accounts[addr] = struct{}{}
	}
	as.cache = nil
}
