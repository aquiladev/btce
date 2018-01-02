package txout

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/wire"
)

func TestPutGet(t *testing.T) {
	db, _ := NewDB("d:\\btce-test")

	value := int64(1119393900022)
	pkScript := []byte{0x00, 0x01, 0x00}
	err := db.Put([]byte("qwerty"), wire.NewTxOut(value, pkScript))
	if err != nil {
		t.Error(err)
	}

	tx, err := db.Get([]byte("qwerty"))
	if err != nil {
		t.Error(err)
	}

	if value != tx.Value {
		t.Error("Wrong value")
	}

	if bytes.Compare(pkScript, tx.PkScript) != 0 {
		t.Error("Wrong pkScript")
	}
}

func TestGet(t *testing.T) {
	db, _ := NewDB("C:\\Users\\BOMK354928\\AppData\\Local\\Btce\\data\\mainnet\\outputs")

	tx, err := db.Get([]byte("db3f14e43fecc80eb3e0827cecce85b3499654694d12272bf91b1b2b8c33b5cb:0"))
	if err != nil {
		t.Error(err)
	}

	fmt.Println(tx)
}
