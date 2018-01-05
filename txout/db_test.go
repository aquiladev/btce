package txout

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
)

func TestPutGet(t *testing.T) {
	db, _ := NewDB("d:\\btce-test")

	value := int64(1119393900022)
	pkScript := []byte{0x00, 0x01, 0x00}
	err := db.Put([]byte("qwerty"), wire.NewTxOut(value, pkScript))
	assert.Equal(t, nil, err)

	tx, err := db.Get([]byte("qwerty"))
	assert.Equal(t, nil, err)
	assert.Equal(t, value, tx.Value)
	assert.Equal(t, pkScript, tx.PkScript)
}

func TestGet(t *testing.T) {
	db, _ := NewDB("C:\\Users\\BOMK354928\\AppData\\Local\\Btce\\data\\mainnet\\outputs")

	tx, err := db.Get([]byte("2e8fa683ff6777b929d38d119dc7c1e15b6a6e78629487775c659fa53765498f:0"))
	assert.Equal(t, nil, err)

	fmt.Println(tx, err)


	tx, err = db.Get([]byte("99442173fd7cae05c4469966fc9870c99b94d6c9784be2a80ddbb5bfd247bc0c:0"))
	assert.Equal(t, nil, err)

	fmt.Println(tx, err)
}