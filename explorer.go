package main

import (
	"sync/atomic"

	"github.com/aquiladev/btce/ledger"
	"github.com/aquiladev/btce/txout"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
)

type Explorer interface {
	GetType() string
	Start()
	Stop()
	WaitForShutdown()
}

type explorer struct {
	started  int32
	shutdown int32

	explorers        []Explorer
	explorersToStart []string
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

// Start begins accepting connections from peers.
func (e *explorer) Start() {
	// Already started?
	if atomic.AddInt32(&e.started, 1) != 1 {
		return
	}

	explLog.Trace("Starting explorer")

	for _, ex := range e.explorers {
		if len(e.explorersToStart) != 0 &&
			!contains(e.explorersToStart, ex.GetType()) {
			continue
		}
		ex.Start()
	}
}

// Stop gracefully shuts down the explorer by stopping and disconnecting all
// peers and the main listener.
func (e *explorer) Stop() error {
	// Make sure this only happens once.
	if atomic.AddInt32(&e.shutdown, 1) != 1 {
		explLog.Infof("Explorer is already in the process of shutting down")
		return nil
	}

	explLog.Warnf("Explorer shutting down")

	for _, ex := range e.explorers {
		go ex.Stop()
	}
	return nil
}

// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (e *explorer) WaitForShutdown() {
	for _, ex := range e.explorers {
		ex.WaitForShutdown()
	}
}

func newExplorer(
	db database.DB,
	chainParams *chaincfg.Params,
	interrupt <-chan struct{},
	txoutDB txout.DB,
	ledgerDB ledger.DB,
	explorersToStart []string) (*explorer, error) {

	// Create a new block chain instance with the appropriate configuration.
	chain, err := blockchain.New(&blockchain.Config{
		DB:          db,
		ChainParams: chainParams,
		Interrupt:   interrupt,
		TimeSource:  blockchain.NewMedianTime(),
	})
	if err != nil {
		return nil, err
	}

	return &explorer{
		explorersToStart: explorersToStart,
		explorers: []Explorer{
			txout.NewExplorer(txoutDB, chain, cfg.BatchSize),
			ledger.NewExplorer(chain, chainParams, txoutDB, ledgerDB, &ledger.Config{
				BatchSize:            cfg.BatchSize,
				BalanceCalcPeriod:    cfg.BalanceCalcPeriod,
				BalanceCalcThreshold: cfg.BalanceCalcThreshold,
			}),
		},
	}, nil
}
