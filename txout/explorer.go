package txout

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcutil"
)

type Explorer struct {
	started     int32
	shutdown    int32

	wg    sync.WaitGroup
	quit  chan struct{}
	chain *blockchain.BlockChain

	db            DB
	handledLogBlk int64
	handledLogTx  int64
	lastLogTime   time.Time
	height        int32
}

func (e *Explorer) start() {
out:
	for {
		select {
		case <-e.quit:
			break out
		default:
		}

		err := e.explore(e.height)
		if err != nil {
			log.Error(err)
			break out
		}
		e.logProgress()

		e.height++
		e.handledLogBlk++
	}

	err := e.db.SetHeight(e.height)
	if err != nil {
		log.Error(err)
	}

	e.wg.Done()
}

func (e *Explorer) startBatch() {
	done := make(chan bool)
	defer close(done)
	batch := 100

out:
	for {
		select {
		case <-e.quit:
			break out
		default:
		}

		for i := 0; i < batch; i++ {
			go func(height int32, ch chan bool) {
				err := e.explore(height)
				if err != nil {
					log.Error(err)
					ch <- false
					return
				}
				ch <- true
			}(e.height, done)

			e.height++
			e.handledLogBlk++
		}
		for i := 0; i < batch; i++ {
			<-done
			e.logProgress()
		}
	}

	err := e.db.SetHeight(e.height)
	if err != nil {
		log.Error(err)
	}

	e.wg.Done()
}

func (e *Explorer) explore(height int32) error {
	block, err := e.chain.BlockByHeight(height)
	if err != nil {
		return err
	}

	return e.store(block)
}

func (e *Explorer) store(block *btcutil.Block) error {
	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()

		for i, txOut := range msgTx.TxOut {
			// Store transaction's output
			key := fmt.Sprintf("%s:%d", tx.Hash(), i)
			err := e.db.Put([]byte(key), txOut)
			if err != nil {
				return err
			}
		}
		e.handledLogTx++
	}
	return nil
}

func (e *Explorer) logProgress() {
	now := time.Now()
	duration := now.Sub(e.lastLogTime)
	if duration < time.Second*time.Duration(20) {
		return
	}

	// Truncate the duration to 10s of milliseconds.
	durationMillis := int64(duration / time.Millisecond)
	tDuration := 10 * time.Millisecond * time.Duration(durationMillis/10)

	// Log information about messages.
	messageStr := "blocks"
	if e.handledLogBlk == 1 {
		messageStr = "block"
	}

	log.Infof("Handled %d %s in the last %s (%d transactions, height %d)",
		e.handledLogBlk, messageStr, tDuration, e.handledLogTx, e.height)

	e.handledLogBlk = 0
	e.handledLogTx = 0
	e.lastLogTime = now
}

func (e *Explorer) Start() {
	// Already started?
	if atomic.AddInt32(&e.started, 1) != 1 {
		return
	}

	log.Trace("Starting TxOut explorer")
	e.wg.Add(1)
	go e.startBatch()
}

func (e *Explorer) Stop() {
	if atomic.AddInt32(&e.shutdown, 1) != 1 {
		log.Warnf("TxOut explorer is already in the process of shutting down")
	}

	log.Infof("TxOut explorer shutting down")
	close(e.quit)
	e.wg.Wait()
}

func (e *Explorer) WaitForShutdown() {
	e.wg.Wait()
}

func NewExplorer(db DB, chain *blockchain.BlockChain) *Explorer {
	// Get height
	height, err := db.GetHeight()
	if err != nil {
		log.Error(err)
		panic(err)
	}
	log.Infof("Height: %d", height)

	return &Explorer{
		quit:        make(chan struct{}),
		db:          db,
		chain:       chain,
		lastLogTime: time.Now(),
		height:      height,
	}
}
