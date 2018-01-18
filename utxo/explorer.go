package utxo

import (
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"
)

type Explorer struct {
	started  int32
	shutdown int32

	wg          sync.WaitGroup
	quit        chan struct{}
	chain       *blockchain.BlockChain
	chainParams *chaincfg.Params

	db                   DB
	handledLogBlk        int64
	handledLogTx         int64
	lastLogTime          time.Time
	height               int32
	batchSize            int
	balanceCalc          *BalanceCalc
	enableCalc           bool
	lastBalanceCalc      int32
	balanceCalcPeriod    int32
	balanceCalcThreshold int32
}

func (e *Explorer) start() {
	if err := e.calcBalances(); err != nil {
		log.Error(err)
	}

out:
	for {
		select {
		case <-e.quit:
			e.logProgress(true)
			break out
		default:
		}

		if err := e.explore(e.height); err != nil {
			if strings.Contains(err.Error(), "no block at height") {
				time.Sleep(1 * time.Minute)
				continue
			}
			log.Error(err)
			break out
		}
		e.logProgress(false)

		e.height++
		e.handledLogBlk++
	}

	if err := e.db.SetHeight(e.height); err != nil {
		log.Error(err)
	}

	e.wg.Done()
}

func (e *Explorer) calcBalances() error {
	if !e.enableCalc {
		return nil
	}

	diff := e.height - e.lastBalanceCalc
	if diff < e.balanceCalcPeriod ||
		e.height < e.balanceCalcThreshold {
		return nil
	}

	if err := e.balanceCalc.Calc(); err != nil {
		return err
	}

	e.lastBalanceCalc = e.height
	return nil
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
		hash := tx.Hash()

		entry, err := e.chain.FetchUtxoEntry(hash)
		if err != nil {
			return err
		}
		if entry == nil {
			continue
		}

		if entry.IsFullySpent() {
			continue
		}

		amount := uint32(len(msgTx.TxOut))
		for i := uint32(0); i < amount; i++ {
			if entry.IsOutputSpent(i) {
				continue
			}

			_, addresses, _, _ := txscript.ExtractPkScriptAddrs(entry.PkScriptByIndex(i), e.chainParams)
			if len(addresses) != 1 {
				log.Infof("Number of inputs %d, Inputs: %+v, TxHash: %+v", len(addresses), addresses, hash)
				continue
			}

			pubKey, err := convertToPubKey(addresses[0])
			if err != nil {
				log.Infof("TxOut %+v, Type: %+v", addresses[0], reflect.TypeOf(addresses[0]))
				return err
			}

			entries = append(entries, Entry{
				Address: []byte(pubKey),
				TxHash:  hash,
				Value:   entry.AmountByIndex(i),
			})
		}

		e.handledLogTx++
	}

	return e.db.PutBatch(entries)
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

func (e *Explorer) GetType() string {
	return "utxo"
}

func (e *Explorer) Start() {
	// Already started?
	if atomic.AddInt32(&e.started, 1) != 1 {
		return
	}

	log.Info("Starting UTXO explorer")
	e.wg.Add(1)
	go e.start()
}

func (e *Explorer) Stop() {
	if atomic.AddInt32(&e.shutdown, 1) != 1 {
		log.Warnf("UTXO explorer is already in the process of shutting down")
	}

	log.Infof("UTXO explorer shutting down")
	close(e.quit)
	e.wg.Wait()
}

func (e *Explorer) WaitForShutdown() {
	e.wg.Wait()
}

func NewExplorer(
	chain *blockchain.BlockChain,
	chainParams *chaincfg.Params,
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
		db:                   db,
		chain:                chain,
		lastLogTime:          time.Now(),
		height:               height,
		balanceCalc:          newBalanceCalc(db),
		enableCalc:           config.EnableBalanceCalc,
		lastBalanceCalc:      0,
		balanceCalcPeriod:    config.BalanceCalcPeriod,
		balanceCalcThreshold: config.BalanceCalcThreshold,
	}
}
