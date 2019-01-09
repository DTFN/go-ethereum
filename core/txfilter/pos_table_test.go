package txfilter

import (
	"github.com/stretchr/testify/require"
	"github.com/ethereum/go-ethereum/common"
	abciTypes "github.com/tendermint/tendermint/abci/types"
	"testing"
	"math/big"
)

func TestUpsertandRemovePosTable(t *testing.T) {
	//手动构造Postable
	table := NewPosTable()
	table.SetThreshold(big.NewInt(1000))
	PubKey1 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000001"),
	}
	PubKey2 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000002"),
	}
	PubKey3 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000003"),
	}
	PubKey4 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000004"),
	}
	Address1 := common.HexToAddress("0x0000000000000000000000000000000000000001")
	Address2 := common.HexToAddress("0x0000000000000000000000000000000000000002")
	Address3 := common.HexToAddress("0x0000000000000000000000000000000000000003")
	Address4 := common.HexToAddress("0x0000000000000000000000000000000000000004")
	tmAddress1 := "fake tmAddress1"
	tmAddress2 := "fake tmAddress2"
	tmAddress3 := "fake tmAddress3"
	tmAddress4 := "fake tmAddress4"

	BlsKeyString1 := "fake blsKeyString1"
	BlsKeyString2 := "fake blsKeyString2"
	BlsKeyString3 := "fake blsKeyString3"
	BlsKeyString4 := "fake blsKeyString4"

	PosItem1 := NewPosItem(
		100,
		10,
		PubKey1,
		tmAddress1,
		BlsKeyString1,
		Address1)
	PosItem2 := NewPosItem(
		100,
		11,
		PubKey2,
		tmAddress2,
		BlsKeyString2,
		Address2)

	//TestUpsertPosTable
	err := table.UpsertPosItem(Address1, PosItem1)
	require.NoError(t, nil, err)
	err = table.UpsertPosItem(Address2, PosItem2)
	require.NoError(t, nil, err)
	require.Equal(t, int64(21), table.TotalSlots)
	require.Equal(t, 0, table.PosItemIndexMap[Address2].index)
	require.Equal(t, 1, table.PosItemIndexMap[Address1].index)
	require.Equal(t, int64(10), table.PosItemMap[Address1].Slots)
	signers:=table.SortedSigners()
	require.Equal(t, signers[0], Address2)
	require.Equal(t, signers[1], Address1)

	PosItem1Copy:=PosItem1.Copy()
	PosItem1Copy.Slots = 90
	err = table.UpsertPosItem(Address1, PosItem1Copy)
	require.NoError(t, nil, err)
	require.Equal(t, int64(101), table.TotalSlots)
	require.Equal(t, 0, table.PosItemIndexMap[Address1].index)
	require.Equal(t, 1, table.PosItemIndexMap[Address2].index)
	require.Equal(t, int64(90), table.PosItemMap[Address1].Slots)
	signers=table.SortedSigners()
	require.Equal(t, signers[0], Address1)
	require.Equal(t, signers[1], Address2)

	PosItem3 := NewPosItem(
		110,
		51,
		PubKey3,
		tmAddress3,
		BlsKeyString3,
		Address3)

	err = table.UpsertPosItem(Address3, PosItem3)
	require.NoError(t, nil, err)
	require.Equal(t, int64(152), table.TotalSlots)
	require.Equal(t, 0, table.PosItemIndexMap[Address1].index)
	require.Equal(t, 1, table.PosItemIndexMap[Address2].index)
	require.Equal(t, 2, table.PosItemIndexMap[Address3].index)
	require.Equal(t, int64(90), table.PosItemMap[Address1].Slots)
	require.Equal(t, int64(51), table.PosItemMap[Address3].Slots)
	signers=table.SortedSigners()
	require.Equal(t, signers[0], Address1)
	require.Equal(t, signers[1], Address3)
	require.Equal(t, signers[2], Address2)

	PosItem4 := NewPosItem(
		120,
		30,
		PubKey4,
		tmAddress4,
		BlsKeyString4,
		Address4)

	err = table.UpsertPosItem(Address4, PosItem4)
	require.Equal(t, int64(182), table.TotalSlots)
	require.Equal(t, 0, table.PosItemIndexMap[Address1].index)
	require.Equal(t, 1, table.PosItemIndexMap[Address4].index)	//2 swap with 4
	require.Equal(t, 2, table.PosItemIndexMap[Address3].index)
	require.Equal(t, 3, table.PosItemIndexMap[Address2].index)
	require.Equal(t, int64(90), table.PosItemMap[Address1].Slots)
	require.Equal(t, int64(51), table.PosItemMap[Address3].Slots)
	require.Equal(t, int64(30), table.PosItemMap[Address4].Slots)
	signers=table.SortedSigners()
	require.Equal(t, signers[0], Address1)
	require.Equal(t, signers[1], Address3)
	require.Equal(t, signers[2], Address4)
	require.Equal(t, signers[3], Address2)

	/*	for i := 25; i < 45; i++ {
			require.Equal(t, Address1, table.PosArray[i])
		}
		upsertFlag, err = table.UpsertPosItem(Address1, big.NewInt(30999), Address1, PubKey1)
		require.Equal(t, big.NewInt(30998), table.PosItemMap[Address1].Balance)

		//引发val_sortlist.go错误处：更新一个比sortlist行首更小的数值
		upsertFlag, err = table.UpsertPosItem(Address2, big.NewInt(8000), Address3, PubKey3)
		require.NoError(t, nil, err)
		require.Equal(t, 53, table.PosArraySize)
		for i := 45; i < 53; i++ {
			require.Equal(t, Address2, table.PosArray[i])
			require.Equal(t, true, table.PosItemMap[Address2].Indexes[i])
		}

		//TestRemovePosTable
		upsertFlag, err = table.RemovePosItem(Address4)
		require.Error(t, fmt.Errorf("address not existed in the postable"))
		require.Equal(t, false, upsertFlag)

		upsertFlag, err = table.RemovePosItem(Address1)
		require.Equal(t, 15, table.PosArraySize)
		require.NotEqual(t, &PosItem1, table.PosItemMap[Address1])*/
}

/*func TestSelectItemByHeightValue(t *testing.T) {
	table := NewPosTable(big.NewInt(1000))
	//table.PosArray[0] = newPosItem(common.HexToAddress("0xe41bf6b389b9007a3436ea1de3257583241ebe3d"), big.NewInt(500), common.HexToAddress("0xa62142888aba8370742be823c1782d17a0389da1"), pubk)
	//table.PosArray[1] = newPosItem(common.HexToAddress("0xa62142888aba8370742be823c1782d17a0389da1"), big.NewInt(1500), common.HexToAddress("0xe41bf6b389b9007a3436ea1de3257583241ebe3d"), pubk)
	table.PosArraySize = 2
	for height := 200; height <= 210; height++ {
		signer, testItem := table.SelectItemByHeightValue(int64(height))
		// 根据SelectItemByRandomValue逻辑，我们已经设定PosArray的具体长度为2,内部元素为table.PosArray[0]与[1]
		// 所以随机选取时,肯定在table.PosArray[0]与[1]中选,那么Balance的值不是500就是1500
		if signer == common.HexToAddress("0xe41bf6b389b9007a3436ea1de3257583241ebe3d") {
			require.Equal(t, big.NewInt(500), testItem.Balance)
		} else {
			require.Equal(t, big.NewInt(1500), testItem.Balance)
		}
	}
}*/
/*
func TestSelectItemBySeedValue(t *testing.T) {
	//手动构造Postable
	table := NewPosTable(big.NewInt(1000))
	PubKey1 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000001"),
	}
	PubKey2 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000002"),
	}
	PubKey3 := abciTypes.PubKey{
		Type: "ed25519",
		Data: []byte("00000000000000000000000000000003"),
	}
	Address1 := common.HexToAddress("0x0000000000000000000000000000000000000001")
	Address2 := common.HexToAddress("0x0000000000000000000000000000000000000002")
	Address3 := common.HexToAddress("0x0000000000000000000000000000000000000003")
	Address4 := common.HexToAddress("0x0000000000000000000000000000000000000004")
	Indexes1 := map[int]bool{}
	Indexes2 := map[int]bool{}
	PosItem1 := PosItem{
		false,
		big.NewInt(0),
		PubKey1,
		Indexes1,
		big.NewInt(0),
	}
	PosItem2 := PosItem{
		false,
		big.NewInt(0),
		PubKey2,
		Indexes2,
		big.NewInt(0),
	}
	var PosItemMap = map[common.Address]*PosItem{
		Address1: &PosItem1,
		Address2: &PosItem2,
	}
	table.PosItemMap = PosItemMap

	_, err := table.UpsertPosItem(Address1, big.NewInt(60000), Address1, PubKey1)
	_, err = table.UpsertPosItem(Address3, big.NewInt(30000), Address3, PubKey3)
	require.NoError(t, nil, err)
	require.Equal(t, Address1, table.PosArray[20])
	require.Equal(t, Address3, table.PosArray[70])
	require.Equal(t, 90, table.PosArraySize)
	//TestSelectItemBySeedValue
	vrf := []byte("00000000000000000000000000000003") //32字
	//PosItem_vrf := PosItem{}
	list := map[common.Address]int{
		Address1: 0,
		Address2: 0,
		Address3: 0,
		Address4: 0,
	}
	for i := 0; i < 256; i++ {
		signer, _ := table.SelectItemBySeedValue(vrf, i)
		list[signer]++
	}

	fmt.Print("结果情况Address1:", list[Address1])
	fmt.Print("结果情况Address2:", list[Address2])
	fmt.Print("结果情况Address3:", list[Address3])
	fmt.Print("结果情况Address4:", list[Address4])

}
*/
