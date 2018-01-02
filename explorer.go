package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/aquiladev/btce/balance"
	"github.com/aquiladev/btce/txout"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
)

type Explorer interface {
	Start()
	Stop()
}

type explorer struct {
	started     int32
	shutdown    int32
	startupTime int64

	wg         sync.WaitGroup
	quit       chan struct{}
	timeSource blockchain.MedianTimeSource
	explorers  map[int][]Explorer
}

// Start begins accepting connections from peers.
func (e *explorer) Start() {
	// Already started?
	if atomic.AddInt32(&e.started, 1) != 1 {
		return
	}

	explLog.Trace("Starting explorer")

	// Explorer startup time. Used for the uptime command for uptime calculation.
	e.startupTime = time.Now().Unix()

	for _, ex := range e.explorers {
		for _, e := range ex {
			go e.Start()
		}
	}

	// Start the peer handler which in turn starts the address and block managers.
	e.wg.Add(1)
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
		for _, e := range ex {
			go e.Stop()
		}
	}

	// Signal the remaining goroutines to quit.
	close(e.quit)

	//TODO: stop exploring
	e.wg.Done()
	return nil
}

// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (e *explorer) WaitForShutdown() {
	e.wg.Wait()
}

func newExplorer(
	db database.DB,
	chainParams *chaincfg.Params,
	interrupt <-chan struct{},
	txoutDB txout.DB,
	balanceDB balance.DB) (*explorer, error) {
	e := explorer{
		quit:       make(chan struct{}),
		timeSource: blockchain.NewMedianTime(),
	}

	// Create a new block chain instance with the appropriate configuration.
	chain, err := blockchain.New(&blockchain.Config{
		DB:          db,
		ChainParams: chainParams,
		Interrupt:   interrupt,
		TimeSource:  e.timeSource,
	})
	if err != nil {
		return nil, err
	}

	// Add explorers
	e.explorers = make(map[int][]Explorer)
	e.explorers[0] = []Explorer{
		txout.NewExplorer(txoutDB, chain),
		balance.NewExplorer(chain, chainParams, txoutDB, balanceDB),
	}

	return &e, nil
}
