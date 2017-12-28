package main

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aquiladev/btce/data"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type keyValue struct {
	key   string
	value int64
}

type explorer struct {
	started     int32
	shutdown    int32
	startupTime int64

	wg          sync.WaitGroup
	quit        chan struct{}
	db          database.DB
	timeSource  blockchain.MedianTimeSource
	chain       *blockchain.BlockChain
	chainParams *chaincfg.Params

	balanceRepo  data.IBalanceRepository
	handledLogTx int64
	lastLogTime  time.Time
}

func (e *explorer) explore() {
	txs := make(map[chainhash.Hash]int32)

	height := int32(0)
	for true {
		block, err := e.chain.BlockByHeight(height)
		if err != nil {
			explLog.Error(err)
			return
		}

		e.exploreBlock(block, txs)
		height++
	}
}

func (e *explorer) fetchBlockHashes() (hashes []chainhash.Hash, err error) {
	blockIdxName := []byte("ffldb-blockidx")
	hashes = make([]chainhash.Hash, 0, 600000)

	err = e.db.View(func(tx database.Tx) error {
		blockIdxBucket := tx.Metadata().Bucket(blockIdxName)

		numLoaded := 0
		startTime := time.Now()
		blockIdxBucket.ForEach(func(k, v []byte) error {
			var hash chainhash.Hash
			copy(hash[:], k)
			hashes = append(hashes, hash)
			numLoaded++
			return nil
		})
		explLog.Infof("Loaded %d hashes in %v", numLoaded, time.Since(startTime))
		return nil
	})
	return
}

func (e *explorer) exploreBlock(block *btcutil.Block, txs map[chainhash.Hash]int32) {
	addrMap := make(map[string]int64)

	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()
		txs[*tx.Hash()] = block.Height()

		// explore input transactions
		for i, txIn := range msgTx.TxIn {
			prevOut := txIn.PreviousOutPoint
			if prevOut.Hash.String() == "0000000000000000000000000000000000000000000000000000000000000000" {
				continue
			}

			var originPkScript []byte
			var originValue int64

			txOut, err := e.getTxOut(&prevOut.Hash, prevOut.Index, txs[prevOut.Hash])
			if err != nil {
				explLog.Error(err)
			}

			if txOut == nil {
				explLog.Infof("TxIn #%d %+v", i, txIn)
				explLog.Infof("Tx %+v", tx)
				explLog.Infof("Block height %+v", block.Height())
				panic("aaaaaaaaaaaaaaa")
			}
			originValue = txOut.Value
			originPkScript = txOut.PkScript

			_, addresses, _, _ := txscript.ExtractPkScriptAddrs(originPkScript, e.chainParams)

			if len(addresses) != 1 {
				explLog.Warnf("Number of inputs %d, Inputs: %+v, PrevOut: %+v", len(addresses), addresses, &prevOut)
				continue
			}

			pubKey, err := convertToPubKey(addresses[0])
			if err != nil {
				explLog.Infof("TxOut %+v, Type: %+v", addresses[0], reflect.TypeOf(addresses[0]))
				explLog.Error(err)
				continue
			}

			explLog.Debugf("TxIn #%d, PrevOut: %+v, Address: %+v, OriginalValue: %d", i, &prevOut, addresses[0], originValue)
			addrMap[pubKey] -= originValue
		}

		// explore output transactions
		for i, txOut := range msgTx.TxOut {
			_, addresses, _, _ := txscript.ExtractPkScriptAddrs(txOut.PkScript, e.chainParams)

			if len(addresses) != 1 {
				explLog.Warnf("Number of outputs %d, Outputs: %+v, Hash: %+v, Tx#: %d, Value: %+v", len(addresses), addresses, tx.Hash(), i, txOut.Value)
				continue
			}

			pubKey, err := convertToPubKey(addresses[0])
			if err != nil {
				explLog.Infof("TxOut %+v, Type: %+v", addresses[0], reflect.TypeOf(addresses[0]))
				explLog.Error(err)
				continue
			}

			addrMap[pubKey] += txOut.Value
		}

		e.logProgress()
	}

	e.writeTxs(addrMap)
}

func (e *explorer) getTxOut(hash *chainhash.Hash, index uint32, height int32) (*wire.TxOut, error) {
	block, err := e.chain.BlockByHeight(height)
	if err != nil {
		return nil, err
	}

	for _, tx := range block.Transactions() {
		if !tx.Hash().IsEqual(hash) {
			continue
		}

		return tx.MsgTx().TxOut[index], nil
	}

	return nil, errors.New("tx not found")
}

func convertToPubKey(addr btcutil.Address) (string, error) {
	switch addr := addr.(type) {
	case *btcutil.AddressPubKeyHash:
		return addr.EncodeAddress(), nil

	case *btcutil.AddressScriptHash:
		//log.Infof("AddressScriptHash %+v", addr)
		return addr.EncodeAddress(), nil

	case *btcutil.AddressPubKey:
		return addr.AddressPubKeyHash().String(), nil

	case *btcutil.AddressWitnessScriptHash:
		explLog.Infof("AddressWitnessScriptHash %+v", addr)
		return addr.EncodeAddress(), nil

	case *btcutil.AddressWitnessPubKeyHash:
		explLog.Infof("AddressWitnessPubKeyHash %+v", addr)
		return addr.EncodeAddress(), nil
	}

	errUnsupportedAddressType := errors.New("address type is not supported " +
		"by the address index")
	return "", errUnsupportedAddressType
}

func (e *explorer) writeTxs(addressMap map[string]int64) {
	done := make(chan bool)

	amount := 0
	for k := range addressMap {
		if addressMap[k] == 0 {
			continue
		}

		pair := &keyValue{
			key:   k,
			value: addressMap[k],
		}

		amount++
		go e.writeTx(done, pair)
	}

	for i := 0; i < amount; i++ {
		<-done
	}
}

func (e *explorer) writeTx(c chan bool, item *keyValue) {
	entry, err := e.balanceRepo.Get(item.key)
	if err != nil {
		explLog.Error(err)
		panic("WWWWWW")
	}

	if entry == nil {
		bEntry := new(data.Balance)
		bEntry.PublicKey = item.key
		bEntry.Value = item.value

		err := e.balanceRepo.Insert(bEntry)
		if err != nil {
			explLog.Error(err)
			c <- false
			panic("WWWWWW")
		}
		c <- true
		return
	}

	entry.Value += item.value

	err = e.balanceRepo.Update(entry)
	if err != nil {
		explLog.Error(err)
		c <- false
		panic("WWWWWW")
	}
	c <- true
}

func (e *explorer) logProgress() {
	e.handledLogTx++

	now := time.Now()
	duration := now.Sub(e.lastLogTime)
	if duration < time.Second*time.Duration(20) {
		return
	}

	// Truncate the duration to 10s of milliseconds.
	durationMillis := int64(duration / time.Millisecond)
	tDuration := 10 * time.Millisecond * time.Duration(durationMillis/10)

	// Log information about messages.
	messageStr := "txs"
	if e.handledLogTx == 1 {
		messageStr = "tx"
	}

	explLog.Infof("Handled %d %s in the last %s",
		e.handledLogTx, messageStr, tDuration)

	e.handledLogTx = 0
	e.lastLogTime = now
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

	e.explore()

	// Start the peer handler which in turn starts the address and block
	// managers.
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

func newExplorer(db database.DB, chainParams *chaincfg.Params, interrupt <-chan struct{}, balanceRepo data.IBalanceRepository) (*explorer, error) {
	e := explorer{
		quit:        make(chan struct{}),
		db:          db,
		chainParams: chainParams,
		timeSource:  blockchain.NewMedianTime(),
		balanceRepo: balanceRepo,
		lastLogTime: time.Now(),
	}

	// Create a new block chain instance with the appropriate configuration.
	var err error
	e.chain, err = blockchain.New(&blockchain.Config{
		DB:          e.db,
		Interrupt:   interrupt,
		ChainParams: e.chainParams,
		TimeSource:  e.timeSource,
	})
	if err != nil {
		return nil, err
	}

	return &e, nil
}
