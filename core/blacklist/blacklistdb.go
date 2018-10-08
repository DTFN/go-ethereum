package blacklist

import (
	"time"
	"github.com/syndtr/goleveldb/leveldb"
	"sync"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/ethereum/go-ethereum/common"
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
)

var (
	blacklistDBEntryExpiration int64                   = 4
	blacklistDBCleanupCycle                            = time.Hour
	sendToLock                                         = "0x7777777777777777777777777777777777777777"
	sendToUnlock                                       = "0x8888888888888888888888888888888888888888"
	w                          map[common.Address]bool = make(map[common.Address]bool)
)

var BlacklistDB *blacklistDB = newBlacklistDB("")

func init() {
	w[common.HexToAddress(sendToLock)] = true
	w[common.HexToAddress(sendToUnlock)] = true
}

type blacklistDB struct {
	lvl           *leveldb.DB
	runner        sync.Once
	quit          chan struct{}
	currentHeight int64
	sync.RWMutex
}

var (
	blacklistDBItemPrefix = "b:"
)

func newBlacklistDB(path string) *blacklistDB {
	//if path == "" {
	log.Info("new blacklist db")
	db := newMemoryBlacklistDB()
	db.ensureExpirer()
	return db
	//}
	//return newPersistentBlacklistDB(path, version)
}

func newMemoryBlacklistDB() *blacklistDB {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil
	}
	return &blacklistDB{
		lvl:  db,
		quit: make(chan struct{}),
	}
}

func makeKey(address common.Address) []byte {
	return []byte(blacklistDBItemPrefix + address.Hex())
}

func (db *blacklistDB) SetCurrentHeight(height int64) {
	if db.getCurrentHeight() == height {
		return;
	}
	db.Lock()
	defer db.Unlock()
	db.currentHeight = height
}

func (db *blacklistDB) getCurrentHeight() int64 {
	db.RLock()
	defer db.RUnlock()
	return db.currentHeight
}

func (db *blacklistDB) fetchInt64(key []byte) (int64, bool) {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return 0, false
	}
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0, false
	}
	return val, true
}

func (db *blacklistDB) storeInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.lvl.Put(key, blob, nil)
}

func (db *blacklistDB) IsBlocked(from common.Address, to *common.Address) bool {
	h, ok := db.fetchInt64(makeKey(from))
	if !ok {
		return false;
	}
	// from 在黑名单或者仍在删除锁定期内，并且 to 为空或者非 (0x777,0x888)
	return (h == -1 || h+blacklistDBEntryExpiration >= db.getCurrentHeight()) && (to == nil || !w[*to]);
}

func (db *blacklistDB) Add(address common.Address) error {
	return db.storeInt64(makeKey(address), -1)
}

func (db *blacklistDB) Remove(address common.Address) error {
	key := makeKey(address)
	v, _ := db.fetchInt64(key)
	if v == -1 {
		return nil
	}
	return db.storeInt64(key, db.getCurrentHeight())
}

func (db *blacklistDB) ensureExpirer() {
	db.runner.Do(func() { go db.expirer() })
}

func (db *blacklistDB) expirer() {
	tick := time.NewTicker(blacklistDBCleanupCycle)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			log.Info("blacklist expirer...")
			if err := db.expireNodes(); err != nil {
				log.Error(fmt.Sprintf("Failed to expire nodedb items: %v", err))
			}
		case <-db.quit:
			return
		}
	}
}

func (db *blacklistDB) expireNodes() error {
	it := db.lvl.NewIterator(nil, nil)
	defer it.Release()
	for it.Next() {
		blob := it.Value()
		val, _ := binary.Varint(blob)
		if val == -1 || val+blacklistDBEntryExpiration > db.getCurrentHeight() {
			continue
		}
		db.lvl.Delete(it.Key(), nil)
	}
	return nil
}

func IsLockTx(to common.Address) bool {
	return to == common.HexToAddress(sendToLock)
}

func IsUnlockTx(to common.Address) bool {
	return to == common.HexToAddress(sendToUnlock)
}
