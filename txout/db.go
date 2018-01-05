package txout

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/btcsuite/goleveldb/leveldb/opt"
)

var (
	heightKey = "height"
)

type KeyTx struct {
	Key []byte
	Tx *wire.TxOut
}

type DB interface {
	GetHeight() (int32, error)
	SetHeight(int32) error
	Get(key []byte) (*wire.TxOut, error)
	Put(key []byte, tx *wire.TxOut) error
	PutBatch([]KeyTx) error
	Close() error
}

type db struct {
	ldb *leveldb.DB
}

var _ DB = (*db)(nil)

func (o *db) GetHeight() (int32, error) {
	height := int64(0)
	data, err := o.ldb.Get([]byte(heightKey), nil)

	if err != nil {
		if strings.Index(err.Error(), "not found") == -1 {
			return 0, err
		}
	} else {
		height, err = binary.ReadVarint(bytes.NewReader(data))
		if err != nil {
			return 0, err
		}
	}

	return int32(height), nil
}

func (o *db) SetHeight(height int32) error {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(buf, int64(height))
	return o.ldb.Put([]byte(heightKey), buf, nil)
}

func (o *db) Get(key []byte) (*wire.TxOut, error) {
	serialized, err := o.ldb.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	pkScript := make([]byte, len(serialized)-binary.MaxVarintLen64)
	value := int64(binary.LittleEndian.Uint64(serialized[:binary.MaxVarintLen64]))
	copy(pkScript, serialized[binary.MaxVarintLen64:])

	return wire.NewTxOut(value, pkScript), nil
}

func (o *db) Put(key []byte, tx *wire.TxOut) error {
	value := make([]byte, len(tx.PkScript)+binary.MaxVarintLen64)
	binary.LittleEndian.PutUint64(value[:binary.MaxVarintLen64], uint64(tx.Value))
	copy(value[binary.MaxVarintLen64:], tx.PkScript[:])

	return o.ldb.Put(key, value[:], nil)
}

func (o *db) PutBatch(entries []KeyTx) error {
	batch := &leveldb.Batch{}

	for _, e := range entries {
		value := make([]byte, len(e.Tx.PkScript)+binary.MaxVarintLen64)
		binary.LittleEndian.PutUint64(value[:binary.MaxVarintLen64], uint64(e.Tx.Value))
		copy(value[binary.MaxVarintLen64:], e.Tx.PkScript[:])

		batch.Put(e.Key, value)
	}

	return o.ldb.Write(batch, nil)
}

func (o *db) Close() error {
	return o.ldb.Close()
}

func NewDB(path string) (*db, error) {
	opts := opt.Options{
		Strict:      opt.DefaultStrict,
		Compression: opt.NoCompression,
	}
	odb, err := leveldb.OpenFile(path, &opts)
	if err != nil {
		return nil, err
	}

	return &db{ldb: odb}, nil
}
