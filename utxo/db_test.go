package utxo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcutil"
	"github.com/stretchr/testify/assert"
)

func TestPutGetBalance(t *testing.T) {
	// Arrange
	appDir := btcutil.AppDataDir("btce", false)
	db, err := NewDB(filepath.Join(appDir, "data\\mainnet\\ledger"))
	assert.Equal(t, nil, err)

	// Act
	iterator := db.GetIterator([]byte("balancex"))
	for iterator.Next() {
		value, err := binary.ReadVarint(bytes.NewReader(iterator.Value()))
		if err != nil {
			fmt.Print(err)
			continue
		}

		fmt.Println(string(iterator.Key()), value)
	}
	iterator.Release()
}
