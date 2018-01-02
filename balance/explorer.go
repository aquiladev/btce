package balance

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aquiladev/btce/txout"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type keyValue struct {
	key   string
	value int64
}

type Explorer struct {
	started     int32
	shutdown    int32
	startupTime int64

	wg          sync.WaitGroup
	quit        chan struct{}
	chain       *blockchain.BlockChain
	chainParams *chaincfg.Params

	txoutDB       txout.DB
	balanceDB     DB
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

		e.explore2()
		e.height++
		e.handledLogBlk++
	}
}

func (e *Explorer) explore() {
	done := make(chan bool)
	defer close(done)

	batch := 500

	for i := 0; i < batch; i++ {
		go func(height int32, ch chan bool) {
			block, err := e.chain.BlockByHeight(height)
			if err != nil {
				log.Error(err)
				close(e.quit)
				return
			}

			e.exploreBlock(block)
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

func (e *Explorer) explore2() {
	block, err := e.chain.BlockByHeight(e.height)
	if err != nil {
		log.Error(err)
		close(e.quit)
		return
	}

	e.exploreBlock(block)
	e.logProgress()
}

func (e *Explorer) exploreBlock(block *btcutil.Block) {
	addrMap := make(map[string]int64)

	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()

		// explore input transactions
		for i, txIn := range msgTx.TxIn {
			prevOut := txIn.PreviousOutPoint
			if prevOut.Hash.String() == "0000000000000000000000000000000000000000000000000000000000000000" {
				continue
			}

			var originPkScript []byte
			var originValue int64

			txOut, err := e.getTxOut(&prevOut.Hash, prevOut.Index)
			if err != nil {
				log.Error(err)
			}

			if txOut == nil {
				log.Infof("TxIn #%d %+v", i, txIn)
				log.Infof("Tx %+v", tx)
				log.Infof("Block height %+v", block.Height())
				panic("aaaaaaaaaaaaaaa")
			}
			originValue = txOut.Value
			originPkScript = txOut.PkScript

			_, addresses, _, _ := txscript.ExtractPkScriptAddrs(originPkScript, e.chainParams)

			if len(addresses) != 1 {
				log.Debugf("Number of inputs %d, Inputs: %+v, PrevOut: %+v", len(addresses), addresses, &prevOut)
				continue
			}

			pubKey, err := convertToPubKey(addresses[0])
			if err != nil {
				log.Infof("TxOut %+v, Type: %+v", addresses[0], reflect.TypeOf(addresses[0]))
				log.Error(err)
				continue
			}

			log.Debugf("TxIn #%d, PrevOut: %+v, Address: %+v, OriginalValue: %d", i, &prevOut, addresses[0], originValue)
			addrMap[pubKey] -= originValue
		}

		// explore output transactions
		for i, txOut := range msgTx.TxOut {
			_, addresses, _, _ := txscript.ExtractPkScriptAddrs(txOut.PkScript, e.chainParams)

			if len(addresses) != 1 {
				log.Debugf("Number of outputs %d, Outputs: %+v, Hash: %+v, Tx#: %d, Value: %+v", len(addresses), addresses, tx.Hash(), i, txOut.Value)
				continue
			}

			pubKey, err := convertToPubKey(addresses[0])
			if err != nil {
				log.Infof("TxOut %+v, Type: %+v", addresses[0], reflect.TypeOf(addresses[0]))
				log.Error(err)
				continue
			}

			addrMap[pubKey] += txOut.Value
		}

		e.handledLogTx++
	}

	e.writeTxs(addrMap)
}

func (e *Explorer) getTxOut(hash *chainhash.Hash, index uint32) (*wire.TxOut, error) {
	key := fmt.Sprintf("%s:%d", hash.String(), index)
	return e.txoutDB.Get([]byte(key))
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
		log.Debugf("AddressWitnessScriptHash %+v", addr)
		return addr.EncodeAddress(), nil

	case *btcutil.AddressWitnessPubKeyHash:
		log.Debugf("AddressWitnessPubKeyHash %+v", addr)
		return addr.EncodeAddress(), nil
	}

	errUnsupportedAddressType := errors.New("address type is not supported " +
		"by the address index")
	return "", errUnsupportedAddressType
}

func (e *Explorer) writeTxs(addressMap map[string]int64) {
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

func (e *Explorer) writeTx(c chan bool, item *keyValue) {
	entry, err := e.balanceDB.Get(item.key)
	if err != nil {
		log.Error(err)
		panic("WWWWWW")
	}

	if entry == nil {
		bEntry := new(Balance)
		bEntry.PublicKey = item.key
		bEntry.Value = item.value

		err := e.balanceDB.Put(bEntry)
		if err != nil {
			log.Error(err)
			c <- false
			panic("WWWWWW")
		}
		c <- true
		return
	}

	entry.Value += item.value

	err = e.balanceDB.Put(entry)
	if err != nil {
		log.Error(err)
		c <- false
		panic("WWWWWW")
	}
	c <- true
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

	log.Trace("Starting Balance explorer")
	e.wg.Add(1)
	go e.start()
}

func (e *Explorer) Stop() {
	if atomic.AddInt32(&e.shutdown, 1) != 1 {
		log.Warnf("Balance explorer is already in the process of shutting down")
	}

	log.Infof("Balance explorer shutting down")
	close(e.quit)
	e.wg.Wait()
}

func NewExplorer(
	chain *blockchain.BlockChain,
	chainParams *chaincfg.Params,
	txoutDB txout.DB,
	balanceDB DB) *Explorer {
	return &Explorer{
		quit:        make(chan struct{}),
		chainParams: chainParams,
		txoutDB:     txoutDB,
		balanceDB:   balanceDB,
		chain:       chain,
		lastLogTime: time.Now(),
		height:      0,
	}
}
