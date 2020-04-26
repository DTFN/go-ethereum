package txfilter

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
)

var (
	EthAuthTable     *AuthTable
	EthAuthTableCopy *AuthTable
)

// TODO: merge this map into PosTable
type AuthTable struct {
	AuthItemMap         map[common.Address]*AuthItem `json:"auth_item_map"`
	RevertAuthTable     *RevertAuthTable             `json:"-"` //store it in another place, for the struct has been stored in the production version
	ThisBlockChangedMap map[string]bool              `json:"-"` //key is tm_address, value is true means let tm add this item, else means let it remove this item
}

// TODO: merge this map into AuthTable
type RevertAuthTable struct {
	TmAddressToSignerMap map[string]common.Address `json:"tm_address_signer_map"` //TODO: do not persist this, add a tm_address field in AuthItem
}

type AuthItem struct {
	ApprovedTxDataHash []byte `json:"approved_tx_data_hash"`
	PermitHeight       int64  `json:"auth_height"`
	StartHeight        int64  `json:"start_height"`
	EndHeight          int64  `json:"end_height"`
}

func (authItem *AuthItem) Copy() *AuthItem {
	copyAuthItem := &AuthItem{
		PermitHeight: authItem.PermitHeight,
		StartHeight:  authItem.StartHeight,
		EndHeight:    authItem.EndHeight,
	}
	copy(copyAuthItem.ApprovedTxDataHash, authItem.ApprovedTxDataHash)
	return copyAuthItem
}

func CreateAuthTable() *AuthTable {
	if EthAuthTable != nil {
		panic("txfilter.EthPermitTable already exist")
	}
	EthAuthTable = NewAuthTable()
	return EthAuthTable
}

func NewAuthTable() *AuthTable {
	return &AuthTable{
		AuthItemMap: make(map[common.Address]*AuthItem),
		RevertAuthTable: &RevertAuthTable{
			TmAddressToSignerMap: make(map[string]common.Address),
		},
		ThisBlockChangedMap: make(map[string]bool),
	}
}

func (authTable *AuthTable) Copy() *AuthTable {
	copyAuthTable := NewAuthTable()
	for addr, authItem := range authTable.AuthItemMap {
		copyAuthTable.AuthItemMap[addr] = authItem.Copy()
	}
	for tmAddr, signer := range authTable.RevertAuthTable.TmAddressToSignerMap {
		copyAuthTable.RevertAuthTable.TmAddressToSignerMap[tmAddr] = signer
	}
	return copyAuthTable
}

func (authTable *AuthTable) InsertAuthItem(permittedAddr common.Address, pi *AuthItem) error {
	fmt.Printf("insert pmi %v for permittedAddr %X", pi, permittedAddr)
	NextPosTable.ChangedFlagThisBlock = true
	if _, ok := authTable.AuthItemMap[permittedAddr]; ok {
		return fmt.Errorf("InsertAuthItem, permittedAddr %X already exists", permittedAddr)
	}
	authTable.AuthItemMap[permittedAddr] = pi
	return nil
}

func (authTable *AuthTable) DeleteAuthItem(permittedAddr common.Address) error {
	fmt.Printf("delete pmi for permittedAddr %X", permittedAddr)
	if _, ok := authTable.AuthItemMap[permittedAddr]; !ok {
		fmt.Printf("DeleteAuthItem, permittedAddr %X does not exist \n", permittedAddr)
		return fmt.Errorf("DeletePermitItem, permittedAddr %X does not exist \n", permittedAddr)
	}
	NextPosTable.ChangedFlagThisBlock = true
	delete(authTable.AuthItemMap, permittedAddr)
	return nil
}

func (authTable *AuthTable) InsertTmAddrSignerPair(tmAddr string, permittedAddr common.Address) error {
	if _, ok := authTable.RevertAuthTable.TmAddressToSignerMap[tmAddr]; ok {
		return fmt.Errorf("InsertTmAddrSignerPair, tmAddr %X already exists", tmAddr)
	}
	authTable.RevertAuthTable.TmAddressToSignerMap[tmAddr] = permittedAddr
	return nil
}

func (authTable *AuthTable) DeleteTmAddrSignerPair(tmAddr string) error {
	if storedSigner, ok := authTable.RevertAuthTable.TmAddressToSignerMap[tmAddr]; !ok {
		return fmt.Errorf("DeleteTmAddrSignerPair, tmAddr %X does not exists", tmAddr)
	}
	delete(authTable.RevertAuthTable.TmAddressToSignerMap, tmAddr)
	return nil
}
