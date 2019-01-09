package txfilter

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"sync"
	abciTypes "github.com/tendermint/tendermint/abci/types"
	"github.com/spaolacci/murmur3"
	"math/rand"
	"container/heap"
)

// it means the lowest bond balance must equal or larger than the 1/1000 of totalBalance
const ThresholdUnit = 1000
const UnbondWaitEpochs = 3
const EpochBlocks = 200

var (
	EthPosTable *PosTable
)

func CreatePosTable() *PosTable {
	EthPosTable = NewPosTable()
	return EthPosTable
}

type PosTable struct {
	Mtx                     sync.RWMutex                          `json:"-"`
	InitFlag                bool                                  `json:"-"`
	PosItemMap              map[common.Address]*PosItem           `json:"pos_item_map"`
	SortedPosItems          *PosItemSortedQueue                   `json:"-"`
	PosItemIndexMap         map[common.Address]*PosItemWithSigner `json:"-"`
	TmAddressToSignerMap    map[string]common.Address             `json:"-"`
	BlsKeyStringToSignerMap map[string]common.Address             `json:"-"`
	TotalSlots              int64                                 `json:"-"`
	UnbondPosItemMap        map[common.Address]*PosItem           `json:"unbond_pos_item_map"`
	SortedUnbondPosItems    *UnbondPosItemSortedQueue             `json:"-"`
	UnbondPosItemIndexMap   map[common.Address]*PosItemWithSigner `json:"-"`
	Threshold               *big.Int                              `json:"threshold"` // threshold value of PosTable
	ChangedFlagThisBlock    bool                                  `json:"-"`
}

func NewPosTable() *PosTable {
	return &PosTable{
		PosItemMap:              make(map[common.Address]*PosItem),
		UnbondPosItemMap:        make(map[common.Address]*PosItem),
		SortedPosItems:          NewPosItemSortedQueue(),
		PosItemIndexMap:         make(map[common.Address]*PosItemWithSigner),
		TmAddressToSignerMap:    make(map[string]common.Address),
		BlsKeyStringToSignerMap: make(map[string]common.Address),
		SortedUnbondPosItems:    NewUnbondPosItemSortedQueue(),
		UnbondPosItemIndexMap:   make(map[common.Address]*PosItemWithSigner),
		TotalSlots:              0,
		ChangedFlagThisBlock:    false,
	}
}

func (posTable *PosTable) Copy() *PosTable {
	posTable.Mtx.RLock()
	defer posTable.Mtx.RUnlock()
	/*	posByte, _ := json.Marshal(posTable)
		newPosTable := NewPosTable(posTable.Threshold)
		json.Unmarshal(posByte, &newPosTable)*/
	newPosTable := NewPosTable()
	for signer, posItem := range posTable.PosItemMap {
		newPosTable.PosItemMap[signer] = posItem.Copy()
	}
	for signer, posItem := range posTable.UnbondPosItemMap {
		newPosTable.UnbondPosItemMap[signer] = posItem.Copy()
	}
	for tmAddress, signer := range posTable.TmAddressToSignerMap {
		newPosTable.TmAddressToSignerMap[tmAddress] = signer
	}
	for blsKeyString, signer := range posTable.BlsKeyStringToSignerMap {
		newPosTable.BlsKeyStringToSignerMap[blsKeyString] = signer
	}
	newPosTable.SortedPosItems = posTable.SortedPosItems.Copy()
	newPosTable.SortedUnbondPosItems = posTable.SortedUnbondPosItems.Copy()
	for _, posItem := range *posTable.SortedPosItems {
		newPosTable.PosItemIndexMap[posItem.Signer] = posItem
	}
	for _, posItem := range *posTable.SortedUnbondPosItems {
		newPosTable.PosItemIndexMap[posItem.Signer] = posItem
	}
	newPosTable.TotalSlots = posTable.TotalSlots
	copyThreashold := big.Int{}
	copyThreashold.Set(posTable.Threshold)
	newPosTable.Threshold = &copyThreashold
	newPosTable.InitFlag = true
	return newPosTable
}

func (posTable *PosTable) SetThreshold(threshold *big.Int) {
	posTable.Mtx.Lock()
	defer posTable.Mtx.Unlock()
	posTable.Threshold = threshold
}

func (posTable *PosTable) InitStruct() {
	posTable.Mtx.Lock()
	defer posTable.Mtx.Unlock()
	totalSlots := int64(0)
	for signer, posItem := range posTable.PosItemMap {
		posItemWithSigner := PosItemWithSigner{
			Height: posItem.Height,
			Signer: signer,
			Slots:  posItem.Slots,
		}
		posTable.SortedPosItems.insert(&posItemWithSigner)
		posTable.PosItemIndexMap[signer] = &posItemWithSigner
		totalSlots += posItem.Slots

		posTable.TmAddressToSignerMap[posItem.TmAddress] = signer
		posTable.BlsKeyStringToSignerMap[posItem.BlsKeyString] = signer
	}
	posTable.TotalSlots = totalSlots

	for signer, posItem := range posTable.UnbondPosItemMap {
		posItemWithSigner := PosItemWithSigner{
			Height: posItem.Height,
			Signer: signer,
			Slots:  posItem.Slots,
		}
		posTable.SortedUnbondPosItems.insert(&posItemWithSigner)
		posTable.UnbondPosItemIndexMap[signer] = &posItemWithSigner

		posTable.TmAddressToSignerMap[posItem.TmAddress] = signer
		posTable.BlsKeyStringToSignerMap[posItem.BlsKeyString] = signer
	}
	posTable.InitFlag = true
}

func (posTable *PosTable) UpsertPosItem(signer common.Address, pi *PosItem) error {
	posTable.Mtx.Lock()
	defer posTable.Mtx.Unlock()
	posTable.ChangedFlagThisBlock = true
	if existedItem, ok := posTable.PosItemMap[signer]; ok {
		if pi.Slots <= existedItem.Slots {
			panic(fmt.Sprintf("locked signer %v balance decreased", signer))
		}
		posTable.PosItemMap[signer] = pi
		posTable.SortedPosItems.update(pi, posTable.PosItemIndexMap[signer].index)
		posTable.TotalSlots += pi.Slots - existedItem.Slots
		return nil
	}
	posTable.PosItemMap[signer] = pi
	posItemWithSigner := PosItemWithSigner{
		Height: pi.Height,
		Signer: signer,
		Slots:  pi.Slots,
	}
	posTable.SortedPosItems.insert(&posItemWithSigner)
	posTable.PosItemIndexMap[signer] = &posItemWithSigner
	posTable.TotalSlots += pi.Slots

	posTable.TmAddressToSignerMap[pi.TmAddress] = signer
	posTable.BlsKeyStringToSignerMap[pi.BlsKeyString] = signer
	return nil
}

func (posTable *PosTable) RemovePosItem(signer common.Address, height int64) error {
	posTable.Mtx.Lock()
	defer posTable.Mtx.Unlock()
	if posItem, ok := posTable.PosItemMap[signer]; ok {
		posTable.ChangedFlagThisBlock = true
		if len(posTable.PosItemMap)-len(posTable.UnbondPosItemMap) <= 4 {
			return fmt.Errorf("cannot remove validator for consensus safety")
		}
		posItem.Height = height
		posTable.UnbondPosItemMap[signer] = posItem
		posItemWithSigner := PosItemWithSigner{
			Height: posItem.Height,
			Signer: signer,
			Slots:  posItem.Slots,
		}
		posTable.SortedUnbondPosItems.insert(&posItemWithSigner)
		posTable.UnbondPosItemIndexMap[signer] = &posItemWithSigner

		delete(posTable.PosItemMap, signer)
		delete(posTable.PosItemIndexMap, signer)
		posTable.SortedPosItems.remove(posTable.PosItemIndexMap[signer].index)
		posTable.TotalSlots -= posItem.Slots
		return nil
	} else {
		return fmt.Errorf("RemovePosItem. signer %v not exist in PosTable", signer)
	}
}

func (posTable *PosTable) TryRemoveUnbondPosItems(currentHeight int64) int {
	count := 0
	posTable.Mtx.Lock()
	defer posTable.Mtx.Unlock()
	for _, posItemWithSigner := range *posTable.SortedUnbondPosItems {
		if (posItemWithSigner.Height/EpochBlocks+UnbondWaitEpochs)*EpochBlocks <= currentHeight {
			posItem, ok := posTable.UnbondPosItemMap[posItemWithSigner.Signer]
			if !ok {
				panic(fmt.Sprintf("PosTable UnbondPosItemMap mismatch with SortedUnbondPosItems. %v ", posTable))
			}
			if len(posTable.PosItemMap)-1 <= 4 {
				panic("cannot remove validator for consensus safety")
			}
			delete(posTable.UnbondPosItemMap, posItemWithSigner.Signer)
			delete(posTable.UnbondPosItemIndexMap, posItemWithSigner.Signer)
			posTable.SortedUnbondPosItems.remove(posTable.UnbondPosItemIndexMap[posItemWithSigner.Signer].index)

			delete(posTable.TmAddressToSignerMap, posItem.TmAddress)
			delete(posTable.BlsKeyStringToSignerMap, posItem.BlsKeyString)
			count++
		} else {
			break
		}
	}
	return count
}

func (posTable *PosTable) SortedSigners() []common.Address {
	topKSigners := []common.Address{}
	copyQueue := posTable.SortedPosItems.Copy()
	len := len(*copyQueue)
	for i := 0; i < len; i++ {
		posItemWithSigner := heap.Pop(copyQueue).(*PosItemWithSigner)
		topKSigners = append(topKSigners, posItemWithSigner.Signer)
	}
	return topKSigners
}

func (posTable *PosTable) TopKSigners(k int) []common.Address {
	topKSigners := []common.Address{}
	posTable.Mtx.RLock()
	copyQueue := posTable.SortedPosItems.Copy()
	posTable.Mtx.RUnlock()
	len := len(*copyQueue)
	if k > len {
		k = len
	}
	for i := 0; i < k; i++ {
		posItemWithSigner := heap.Pop(copyQueue).(*PosItemWithSigner)
		topKSigners = append(topKSigners, posItemWithSigner.Signer)
	}
	return topKSigners
}

func (posTable *PosTable) SelectItemByHeightValue(random int64) (common.Address, PosItem) {
	r := rand.New(rand.NewSource(random))
	index := int64(r.Intn(int(posTable.TotalSlots)))
	sumSlots := int64(0)
	signers := posTable.SortedSigners()
	for _, signer := range signers {
		posItem := posTable.PosItemMap[signer]
		sumSlots += posItem.Slots
		if sumSlots >= index {
			return signer, *posItem
		}
	}
	panic(fmt.Sprintf("random index %v out of SortedPosItems total slots range", index))
}

func (posTable *PosTable) SelectItemBySeedValue(vrf []byte, len int) (common.Address, PosItem) {
	res64 := murmur3.Sum32(vrf)
	r := rand.New(rand.NewSource(int64(res64) + int64(len)))
	index := int64(r.Intn(int(posTable.TotalSlots)))
	sumSlots := int64(0)
	signers := posTable.SortedSigners()
	for _, signer := range signers {
		posItem := posTable.PosItemMap[signer]
		sumSlots += posItem.Slots
		if sumSlots >= index {
			return signer, *posItem
		}
	}
	panic(fmt.Sprintf("random index %v out of SortedPosItems total slots range", index))
}

type PosItem struct {
	Height           int64            `json:"height"`
	Slots            int64            `json:"slots"`
	PubKey           abciTypes.PubKey `json:"pubKey"`
	TmAddress        string           `json:"tm_address"`
	BlsKeyString     string           `json:"bls_key_string"`
	Beneficiary      common.Address   `json:"beneficiary"`
	BeneficiaryBonus *big.Int         `json:"beneficiary_bonus"` //currently not used
}

func NewPosItem(height int64, slots int64, pubKey abciTypes.PubKey, tmAddress string, blsKeyString string, beneficiary common.Address) *PosItem {
	return &PosItem{
		Height:       height,
		Slots:        slots,
		PubKey:       pubKey,
		TmAddress:    tmAddress,
		BlsKeyString: blsKeyString,
		Beneficiary:  beneficiary,
	}
}

func (pi *PosItem) Copy() *PosItem {
	copyPubKey := abciTypes.PubKey{Type: pi.PubKey.Type, Data: make([]byte, len(pi.PubKey.Data))}
	copy(copyPubKey.Data, pi.PubKey.Data)
	copyBeneficiary := common.Address{}
	copyBeneficiary.SetBytes(pi.Beneficiary.Bytes())
	return &PosItem{
		Height:       pi.Height,
		Slots:        pi.Slots,
		PubKey:       copyPubKey,
		TmAddress:    pi.TmAddress,
		BlsKeyString: pi.BlsKeyString,
		Beneficiary:  copyBeneficiary,
	}
}

//=============================================================================
type PosItemWithSigner struct {
	Height int64
	Signer common.Address
	Slots  int64
	index  int
}

func (pi *PosItemWithSigner) Copy() *PosItemWithSigner {
	copySigner := common.Address{}
	copySigner.SetBytes(pi.Signer.Bytes())
	return &PosItemWithSigner{
		Height: pi.Height,
		Signer: copySigner,
		Slots:  pi.Slots,
		index:  pi.index,
	}
}

type PosItemSortedQueue []*PosItemWithSigner

func NewPosItemSortedQueue() *PosItemSortedQueue {
	q := make([]*PosItemWithSigner, 0)
	sq := PosItemSortedQueue(q)
	return &sq
}

func (pq *PosItemSortedQueue) Copy() *PosItemSortedQueue {
	newQueue := NewPosItemSortedQueue()
	for _, pi := range *pq {
		*newQueue = append(*newQueue, pi.Copy())
	}
	return newQueue
}

func (pq *PosItemSortedQueue) Len() int { return len(*pq) }

func (pq *PosItemSortedQueue) Less(i, j int) bool {
	if (*pq)[i].Slots != (*pq)[j].Slots {
		return (*pq)[i].Slots > (*pq)[j].Slots
	}
	return (*pq)[i].Signer.String() > (*pq)[j].Signer.String()
}

func (pq *PosItemSortedQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
	(*pq)[i].index = i
	(*pq)[j].index = j
}

func (pq *PosItemSortedQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PosItemWithSigner)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PosItemSortedQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PosItemSortedQueue) insert(item *PosItemWithSigner) {
	heap.Push(pq, item)
}

// update modifies the priority and value of an Item in the queue.
func (pq *PosItemSortedQueue) update(item *PosItem, index int) {
	targetItem := (*pq)[index]
	targetItem.Slots = item.Slots
	targetItem.Height = item.Height
	heap.Fix(pq, index)
}

func (pq *PosItemSortedQueue) remove(index int) {
	heap.Remove(pq, index)
}

//=============================================================

type UnbondPosItemSortedQueue []*PosItemWithSigner

func NewUnbondPosItemSortedQueue() *UnbondPosItemSortedQueue {
	q := make([]*PosItemWithSigner, 0)
	sq := UnbondPosItemSortedQueue(q)
	return &sq
}

func (pq *UnbondPosItemSortedQueue) Copy() *UnbondPosItemSortedQueue {
	newQueue := NewUnbondPosItemSortedQueue()
	for _, pi := range *pq {
		*newQueue = append(*newQueue, pi.Copy())
	}
	return newQueue
}

func (pq *UnbondPosItemSortedQueue) Len() int { return len(*pq) }

func (pq *UnbondPosItemSortedQueue) Less(i, j int) bool {
	if (*pq)[i].Height != (*pq)[j].Height {
		return (*pq)[i].Height > (*pq)[j].Height
	}
	return (*pq)[i].Signer.String() > (*pq)[j].Signer.String()
}

func (pq *UnbondPosItemSortedQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
	(*pq)[i].index = i
	(*pq)[j].index = j
}

func (pq *UnbondPosItemSortedQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PosItemWithSigner)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *UnbondPosItemSortedQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *UnbondPosItemSortedQueue) insert(item *PosItemWithSigner) {
	heap.Push(pq, item)
	heap.Fix(pq, item.index)
}

// update modifies the priority and value of an Item in the queue.
func (pq *UnbondPosItemSortedQueue) update(item *PosItem, index int) {
	targetItem := (*pq)[index]
	targetItem.Slots = item.Slots
	targetItem.Height = item.Height
	heap.Fix(pq, index)
}

func (pq *UnbondPosItemSortedQueue) remove(index int) {
	heap.Remove(pq, index)
}
