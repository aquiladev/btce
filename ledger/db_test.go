package ledger

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
)

func TestPutBatchGetIterator(t *testing.T) {
	// Arrange
	db, _ := NewDB("d:\\btce-test")
	hs1, _ := chainhash.NewHashFromStr("f4b5c0df7339a7ad1bc6b2ae57613af2d6c262c5799f560007c95d3a1bc007f1")
	hs2, _ := chainhash.NewHashFromStr("b28a89449e1a427248921cd3eff0fd35cef67e95b72774645494d0948ca3174f")

	entries := []Entry{
		{
			Address: []byte("0xf993dbd84dc713862eb6ead40deff0f2e8bed2e4"),
			TxHash:  hs1,
			In:      true,
			Value:   int64(1119393900022),
		},
		{
			Address: []byte("0xac60E10c4f29c4B8D7Ce5D3F01Ee4Cd631447CD0"),
			TxHash:  hs2,
			In:      false,
			Value:   int64(24135325342),
		},
	}
	err := db.PutBatch(entries)
	assert.Equal(t, nil, err)

	// Act-Assert 0
	iter := db.GetIterator(entries[0].Address)
	count := 0
	for iter.Next() {
		count++
		entry, err := ToEntry(iter.Key(), iter.Value())
		assert.Equal(t, nil, err)
		assert.Equal(t, &entries[0], entry)
	}
	iter.Release()
	assert.Equal(t, 1, count)

	// Act-Assert 1
	iter = db.GetIterator(entries[1].Address)
	count = 0
	for iter.Next() {
		count++
		entry, err := ToEntry(iter.Key(), iter.Value())
		assert.Equal(t, nil, err)
		assert.Equal(t, &entries[1], entry)
	}
	iter.Release()
	assert.Equal(t, 1, count)
}

func TestPutGetBalance(t *testing.T) {
	// Arrange
	db, _ := NewDB("d:\\btce-test")
	balance := &Balance{
		Key:   []byte{0x01, 0x02, 0x03},
		Value: 123321,
	}

	// Act
	err := db.PutBalance(balance)
	assert.Equal(t, nil, err)

	// Assert
	res, err := db.GetBalance([]byte{0x01, 0x02, 0x03})
	assert.Equal(t, nil, err)
	assert.Equal(t, balance, res)
}

func iTestAddrIdx(t *testing.T) {
	db, _ := NewDB("C:\\Users\\BOMK354928\\AppData\\Local\\Btce\\data\\mainnet\\ledger")

	//iter := db.GetIterator([]byte("addressidx"))
	//count := 0
	//for iter.Next() {
	//	count++
	//
	//	fmt.Println(string(iter.Value()))
	//
	//	if count > 100 {
	//		break
	//	}
	//}
	//iter.Release()
	//
	//fmt.Println(count)

	iter1 := db.GetIterator([]byte("1DKMg2KmTyQmfiQxDfDsNKVx4PWYa7xga4"))
	for iter1.Next() {
		fmt.Println(ToEntry(iter1.Key(), iter1.Value()))
	}
	iter1.Release()
}
