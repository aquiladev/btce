package ledger

import (
	"testing"

	"github.com/aquiladev/btce/txout"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/stretchr/testify/assert"
	"github.com/btcsuite/btclog"
)

type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func TestZeroHash(t *testing.T) {
	hash, err := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000000")
	assert.Equal(t, nil, err)

	assert.Equal(t, hash, &chainhash.Hash{})
}

func BenchmarkExplore(b *testing.B) {
	b.StopTimer()
	// Arrange
	backendLog := btclog.NewBackend(logWriter{})
	UseLogger(backendLog.Logger("TEST"))

	sourceDB, err := loadSourceDB()
	assert.Equal(b, nil, err)
	defer sourceDB.Close()

	chain, err := blockchain.New(&blockchain.Config{
		DB:          sourceDB,
		ChainParams: &chaincfg.MainNetParams,
		Interrupt:   nil,
		TimeSource:  blockchain.NewMedianTime(),
	})
	assert.Equal(b, nil, err)

	txoutDB, err := loadTxOutDB()
	assert.Equal(b, nil, err)
	defer txoutDB.Close()

	db, err := loadDB()
	assert.Equal(b, nil, err)
	defer db.Close()

	explorer := NewExplorer(chain, &chaincfg.MainNetParams, txoutDB, db, 0)

	// Act
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err := explorer.explore(5992)
		assert.Equal(b, nil, err)
	}
}

func loadSourceDB() (database.DB, error) {
	db, err := database.Open("ffldb", "D:\\btcd\\data\\mainnet\\blocks_ffldb", chaincfg.MainNetParams.Net)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func loadTxOutDB() (txout.DB, error) {
	db, err := txout.NewDB("C:\\Users\\BOMK354928\\AppData\\Local\\Btce\\data\\mainnet\\outputs")
	if err != nil {
		return nil, err
	}

	return db, nil
}

func loadDB() (DB, error) {
	db, err := NewDB("C:\\Users\\BOMK354928\\AppData\\Local\\Btce\\data\\mainnet\\ledger")
	if err != nil {
		return nil, err
	}

	return db, nil
}
