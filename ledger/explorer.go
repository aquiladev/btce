package ledger

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aquiladev/btce/txout"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"
)

var ErrTxOutNotFound = fmt.Errorf("tx out not found")

type Explorer struct {
	started  int32
	shutdown int32

	wg          sync.WaitGroup
	quit        chan struct{}
	chain       *blockchain.BlockChain
	chainParams *chaincfg.Params

	txoutDB              txout.DB
	db                   DB
	balanceCalc          *BalanceCalc
	handledLogBlk        int64
	handledLogTx         int64
	lastLogTime          time.Time
	lastBalanceCalc      int32
	height               int32
	batchSize            int
	balanceCalcPeriod    int32
	balanceCalcThreshold int32
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
			if strings.Contains(err.Error(), "no block at height") {
				time.Sleep(1 * time.Minute)
				continue
			}
			log.Error(err)
			break out
		}
		err = e.calcBalances()
		if err != nil {
			log.Error(err)
			break out
		}
		e.logProgress(false)

		e.height++
		e.handledLogBlk++
	}
	e.logProgress(true)

	err := e.db.SetHeight(e.height)
	if err != nil {
		log.Error(err)
	}

	e.wg.Done()
}

func (e *Explorer) startBatch() {
	done := make(chan error)
	defer close(done)

out:
	for {
		select {
		case <-e.quit:
			break out
		default:
		}

		for i := 0; i < e.batchSize; i++ {
			go func(height int32, ch chan error) {
				ch <- e.explore(height)
			}(e.height+int32(i), done)
		}
		for i := 0; i < e.batchSize; i++ {
			err := <-done
			if err != nil {
				if strings.Contains(err.Error(), "no block at height") {
					time.Sleep(1 * time.Minute)
					continue
				}
				log.Error(err)
				break out
			}
			err = e.calcBalances()
			if err != nil {
				log.Error(err)
				break out
			}
			e.logProgress(false)

			e.handledLogBlk++
		}
		e.height += int32(e.batchSize)
	}
	e.logProgress(true)

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

	return e.exploreBlock(block)
}

func (e *Explorer) exploreBlock(block *btcutil.Block) error {
	var entries []Entry
	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()

		// explore input transactions
		for i, txIn := range msgTx.TxIn {
			prevOut := txIn.PreviousOutPoint
			if prevOut.Hash.IsEqual(&chainhash.Hash{}) {
				continue
			}

			key := fmt.Sprintf("%s:%d", &prevOut.Hash, prevOut.Index)
			txOut, err := e.txoutDB.Get([]byte(key))
			if err != nil {
				return err
			}

			if txOut == nil {
				log.Infof("TxIn #%d %+v", i, txIn)
				log.Infof("Tx %+v", tx)
				log.Infof("Block height %+v", block.Height())
				return ErrTxOutNotFound
			}

			_, addresses, _, _ := txscript.ExtractPkScriptAddrs(txOut.PkScript, e.chainParams)

			if len(addresses) != 1 {
				log.Infof("Number of inputs %d, Inputs: %+v, PrevOut: %+v, TxHash: %+v", len(addresses), addresses, &prevOut, tx.Hash())
				continue
			}

			pubKey, err := convertToPubKey(addresses[0])
			if err != nil {
				log.Infof("TxOut %+v, Type: %+v", addresses[0], reflect.TypeOf(addresses[0]))
				return err
			}

			log.Debugf("TxIn #%d, PrevOut: %+v, Address: %+v, OriginalValue: %d", i, &prevOut, addresses[0], txOut.Value)

			entries = append(entries, Entry{
				Address: []byte(pubKey),
				TxHash:  tx.Hash(),
				In:      false,
				Value:   txOut.Value,
			})
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
				return err
			}

			entries = append(entries, Entry{
				Address: []byte(pubKey),
				TxHash:  tx.Hash(),
				In:      true,
				Value:   txOut.Value,
			})
		}

		e.handledLogTx++
	}

	return e.db.PutBatch(entries)
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

func (e *Explorer) calcBalances() error {
	diff := e.height - e.lastBalanceCalc
	if diff < e.balanceCalcPeriod ||
		e.height < e.balanceCalcThreshold {
		return nil
	}

	err := e.balanceCalc.Calc()
	if err != nil {
		return err
	}

	e.lastBalanceCalc = e.height
	return nil
}

func (e *Explorer) logProgress(force bool) {
	now := time.Now()
	duration := now.Sub(e.lastLogTime)
	if duration < time.Second*time.Duration(20) && !force {
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

func (e *Explorer) GetType() string {
	return "ledger"
}

func (e *Explorer) Start() {
	// Already started?
	if atomic.AddInt32(&e.started, 1) != 1 {
		return
	}

	log.Info("Starting Ledger explorer")
	e.wg.Add(1)
	go e.startBatch()
}

func (e *Explorer) Stop() {
	if atomic.AddInt32(&e.shutdown, 1) != 1 {
		log.Warnf("Ledger explorer is already in the process of shutting down")
	}

	log.Infof("Ledger explorer shutting down")
	close(e.quit)
	e.wg.Wait()
}

func (e *Explorer) WaitForShutdown() {
	e.wg.Wait()
}

func NewExplorer(
	chain *blockchain.BlockChain,
	chainParams *chaincfg.Params,
	txoutDB txout.DB,
	db DB,
	config *Config) *Explorer {

	// Get height
	height, err := db.GetHeight()
	if err != nil {
		log.Error(err)
		panic(err)
	}
	log.Infof("Height: %d", height)

	return &Explorer{
		quit:                 make(chan struct{}),
		chainParams:          chainParams,
		txoutDB:              txoutDB,
		db:                   db,
		balanceCalc:          newBalanceCalc(db),
		chain:                chain,
		lastLogTime:          time.Now(),
		height:               height,
		lastBalanceCalc:      height,
		batchSize:            config.BatchSize,
		balanceCalcPeriod:    config.BalanceCalcPeriod,
		balanceCalcThreshold: config.BalanceCalcThreshold,
	}
}
