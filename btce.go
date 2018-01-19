package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/aquiladev/btce/ledger"
	"github.com/aquiladev/btce/txout"
	"github.com/aquiladev/btce/utxo"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/limits"
)

const (
	// blockDbNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	blockDbNamePrefix = "blocks"
)

var (
	cfg *config
)

func btceMain(explorerChan chan<- *explorer) error {
	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	tcfg, _, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = tcfg
	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()

	// Get a channel that will be closed when a shutdown signal has been
	// triggered either from an OS signal such as SIGINT (Ctrl+C) or from
	// another subsystem such as the RPC server.
	interrupt := interruptListener()
	defer btceLog.Info("Shutdown complete")

	// Show version at startup.
	btceLog.Infof("Version %s", version())

	// Return now if an interrupt signal was triggered.
	if interruptRequested(interrupt) {
		return nil
	}

	// Load the block database.
	sourceDB, err := loadSourceDB()
	if err != nil {
		btceLog.Errorf("%v", err)
		return err
	}
	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		btceLog.Infof("Gracefully shutting down the database...")
		sourceDB.Close()
	}()

	// Return now if an interrupt signal was triggered.
	if interruptRequested(interrupt) {
		return nil
	}

	// TxOut DB
	txoutDBPath := filepath.Join(cfg.DataDir, "outputs")
	txoutDB, err := txout.NewDB(txoutDBPath)
	if err != nil {
		return err
	}
	defer func() {
		txoutDB.Close()
	}()

	// Ledger DB
	ledgerDBPath := filepath.Join(cfg.DataDir, "ledger")
	ledgerDB, err := ledger.NewDB(ledgerDBPath)
	if err != nil {
		return err
	}
	defer func() {
		ledgerDB.Close()
	}()

	// UTXO DB
	utxoDBPath := filepath.Join(cfg.DataDir, "utxo")
	utxoDB, err := utxo.NewDB(utxoDBPath)
	if err != nil {
		return err
	}
	defer func() {
		utxoDB.Close()
	}()

	// Explorers to start
	var explorersToStart []string
	if len(cfg.Explorers) > 0 {
		explorersToStart = strings.Split(cfg.Explorers, ",")
	}

	// Create explorer and start it.
	explorer, err := newExplorer(sourceDB, activeNetParams.Params, interrupt, txoutDB, ledgerDB, utxoDB, explorersToStart)
	if err != nil {
		btceLog.Errorf("Unable to start explorer: %v", err)
		return err
	}
	defer func() {
		btceLog.Infof("Gracefully shutting down the explorer...")
		explorer.Stop()
		explorer.WaitForShutdown()
		explLog.Infof("Explorer shutdown complete")
	}()
	explorer.Start()
	if explorerChan != nil {
		explorerChan <- explorer
	}

	// Wait until the interrupt signal is received from an OS signal or
	// shutdown is requested through one of the subsystems such as the RPC
	// server.
	<-interrupt
	return nil
}

// dbPath returns the path to the block database given a database type.
func sourceDbPath(dbType string) string {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName = dbName + ".db"
	}
	dbPath := filepath.Join(cfg.SourceDataDir, dbName)
	return dbPath
}

// loadSourceDB loads (or creates when needed) the block database taking into
// account the selected database backend and returns a handle to it.  It also
// contains additional logic such warning the user if there are multiple
// databases which consume space on the file system and ensuring the regression
// test database is clean when in regression test mode.
func loadSourceDB() (database.DB, error) {
	// The memdb backend does not have a file path associated with it, so
	// handle it uniquely.  We also don't want to worry about the multiple
	// database type warnings when running with the memory database.
	if cfg.DbType == "memdb" {
		btceLog.Infof("Creating block database in memory.")
		db, err := database.Create(cfg.DbType)
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	// The database name is based on the database type.
	dbPath := sourceDbPath(cfg.DbType)

	btceLog.Infof("Loading source database from '%s'", dbPath)
	db, err := database.Open(cfg.DbType, dbPath, activeNetParams.Net)
	if err != nil {
		return nil, err
	}

	btceLog.Info("Block database loaded")
	return db, nil
}

func main() {
	// Use all processor cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Block and transaction processing can cause bursty allocations.  This
	// limits the garbage collector from excessively overallocating during
	// bursts.  This value was arrived at with the help of profiling live
	// usage.
	debug.SetGCPercent(10)

	// Up some limits.
	if err := limits.SetLimits(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set limits: %v\n", err)
		os.Exit(1)
	}

	// Work around defer not working after os.Exit()
	if err := btceMain(nil); err != nil {
		os.Exit(1)
	}
}
