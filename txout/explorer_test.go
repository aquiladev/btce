package txout

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcutil"
	"github.com/stretchr/testify/assert"
)

func TestTx(t *testing.T) {
	// Arrange
	sourceDB, err := loadSourceDB()
	assert.Equal(t, nil, err)

	chain, err := blockchain.New(&blockchain.Config{
		DB:          sourceDB,
		ChainParams: &chaincfg.MainNetParams,
		Interrupt:   nil,
		TimeSource:  blockchain.NewMedianTime(),
	})
	assert.Equal(t, nil, err)

	//db, err := loadDB()
	//assert.Equal(t, nil, err)

	//explorer := NewExplorer(db, chain)

	//explorer.explore(5992)

	block, err := chain.BlockByHeight(5992)
	assert.Equal(t, nil, err)

	var tx btcutil.Tx
	for _, t := range block.Transactions() {
		if t.Hash().String() == "99442173fd7cae05c4469966fc9870c99b94d6c9784be2a80ddbb5bfd247bc0c" {
			tx = *t
		}
	}

	msgTx := tx.MsgTx()

	for i, txIn := range msgTx.TxIn {
		fmt.Printf("IN #%d - %+v\n", i, txIn)
	}

	for i, txOut := range msgTx.TxOut {
		fmt.Printf("OUT #%d - %+v\n", i, txOut)
	}
}

func loadSourceDB() (database.DB, error) {
	db, err := database.Open("ffldb", "D:\\btcd\\data\\mainnet\\blocks_ffldb", chaincfg.MainNetParams.Net)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func loadDB() (DB, error) {
	db, err := NewDB("C:\\Users\\BOMK354928\\AppData\\Local\\Btce\\data\\mainnet\\outputs")
	if err != nil {
		return nil, err
	}

	return db, nil
}
