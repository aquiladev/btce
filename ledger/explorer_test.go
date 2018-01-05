package ledger

import (
	"testing"
	"path/filepath"

	"github.com/aquiladev/btce/txout"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcutil"
	"github.com/stretchr/testify/assert"
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

	explorer := NewExplorer(chain, &chaincfg.MainNetParams, txoutDB, db, nil)

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
	appDir := btcutil.AppDataDir("btce", false)
	db, err := txout.NewDB(filepath.Join(appDir, "data\\mainnet\\outputs"))
	if err != nil {
		return nil, err
	}

	return db, nil
}

func loadDB() (DB, error) {
	appDir := btcutil.AppDataDir("btce", false)
	db, err := NewDB(filepath.Join(appDir, "data\\mainnet\\ledger"))
	if err != nil {
		return nil, err
	}

	return db, nil
}
