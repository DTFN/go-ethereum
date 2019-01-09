package txfilter

/*
func TestUpsertandRemovePosTable(t *testing.T) {
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

	//TestUpsertPosTable
	upsertFlag, err := table.UpsertPosItem(Address1, big.NewInt(10000), Address1, PubKey1)
	require.NoError(t, nil, err)
	require.Equal(t, 10, table.PosArraySize)
	require.Equal(t, 10, len(table.PosItemMap[Address1].Indexes))
	for i := 0; i < 10; i++ {
		require.Equal(t, true, table.PosItemMap[Address1].Indexes[i])
	}
	require.Equal(t, big.NewInt(10000), table.PosItemMap[Address1].Balance)

	upsertFlag, err = table.UpsertPosItem(Address1, big.NewInt(9000), Address1, PubKey1)
	require.Error(t, fmt.Errorf("situation shouldn't happened in real world"))
	require.Equal(t, false, upsertFlag)

	upsertFlag, err = table.UpsertPosItem(Address3, big.NewInt(15000), Address3, PubKey3)
	require.NoError(t, nil, err)
	require.Equal(t, 25, table.PosArraySize)

	for i := 0; i < 10; i++ {
		require.Equal(t, Address1, table.PosArray[i])
	}
	for i := 10; i < 25; i++ {
		require.Equal(t, Address3, table.PosArray[i])
		require.Equal(t, true, table.PosItemMap[Address3].Indexes[i])
	}
	require.Equal(t, 15, len(table.PosItemMap[Address3].Indexes))
	upsertFlag, err = table.UpsertPosItem(Address1, big.NewInt(30998), Address1, PubKey1)
	require.NoError(t, nil, err)
	require.Equal(t, 45, table.PosArraySize)
	require.Equal(t, big.NewInt(30998), table.PosItemMap[Address1].Balance)
	//require.Equal(t,45,len(table.))
	for i := 25; i < 45; i++ {
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
	require.NotEqual(t, &PosItem1, table.PosItemMap[Address1])
}

func TestSelectItemByHeightValue(t *testing.T) {
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
}

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