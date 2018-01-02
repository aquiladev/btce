package balance

import (
	"bytes"
	"encoding/binary"

	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/btcsuite/goleveldb/leveldb/opt"
	"github.com/btcsuite/goleveldb/leveldb/filter"
)

type Balance struct {
	PublicKey string
	Value     int64
}

type DB interface {
	Get(publicKey string) (*Balance, error)
	Put(balance *Balance) error
	Close() error
}

type db struct {
	ldb *leveldb.DB
}

var _ DB = (*db)(nil)

func (bdb *db) Get(publicKey string) (*Balance, error) {
	data, err := bdb.ldb.Get([]byte(publicKey), nil)
	if err == leveldb.ErrNotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	value, err := binary.ReadVarint(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	balance := &Balance{
		PublicKey: publicKey,
		Value:     value,
	}

	return balance, nil
}

func (bdb *db) Put(balance *Balance) error {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(buf, balance.Value)
	return bdb.ldb.Put([]byte(balance.PublicKey), buf, nil)
}

func (bdb *db) Close() error {
	return bdb.ldb.Close()
}

func NewDB(path string) (*db, error) {
	opts := opt.Options{
		Strict:       opt.DefaultStrict,
		Compression:  opt.NoCompression,
		Filter:       filter.NewBloomFilter(10),
	}
	ldb, err := leveldb.OpenFile(path, &opts)

	if err != nil {
		return nil, err
	}

	return &db{ldb: ldb}, nil
}
