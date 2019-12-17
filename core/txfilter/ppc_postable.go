package txfilter

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/spaolacci/murmur3"
	"math/rand"
)

func (posTable *PosTable) PPCSelectItemByHeightValue(random int64) (common.Address, PosItem) {
	r := rand.New(rand.NewSource(random))

	//wenbin add,change select strategy.
	rSeed := r.Intn(len(posTable.SortedSigners))
	signer := posTable.SortedSigners[rSeed]
	posItem := posTable.PosItemMap[signer]
	return signer, *posItem

	//index := int64(r.Intn(int(posTable.TotalSlots)))
	//sumSlots := int64(0)
	//for _, signer := range posTable.SortedSigners {
	//	posItem := posTable.PosItemMap[signer]
	//	sumSlots += posItem.Slots
	//	if sumSlots >= index {
	//		return signer, *posItem
	//	}
	//}
	//panic(fmt.Sprintf("random index %v out of SortedPosItems total slots range", index))
}

func (posTable *PosTable) PPCSelectItemBySeedValue(vrf []byte, length int) (common.Address, PosItem) {
	res64 := murmur3.Sum32(vrf)
	r := rand.New(rand.NewSource(int64(res64) + int64(length)))

	//wenbin add,change select strategy.
	rSeed := r.Intn(len(posTable.SortedSigners))
	signer := posTable.SortedSigners[rSeed]
	posItem := posTable.PosItemMap[signer]
	return signer, *posItem

	//index := int64(r.Intn(int(posTable.TotalSlots)))
	//sumSlots := int64(0)
	//for _, signer := range posTable.SortedSigners {
	//	posItem := posTable.PosItemMap[signer]
	//	sumSlots += posItem.Slots
	//	if sumSlots >= index {
	//		return signer, *posItem
	//	}
	//}
	//panic(fmt.Sprintf("random index %v out of SortedPosItems total slots range", index))
}
