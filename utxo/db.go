package utxo

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/btcsuite/goleveldb/leveldb/filter"
	"github.com/btcsuite/goleveldb/leveldb/iterator"
	"github.com/btcsuite/goleveldb/leveldb/opt"
	"github.com/btcsuite/goleveldb/leveldb/util"
)

type Balance struct {
	Key   []byte
	Value int64
}

var (
	heightKey = "height"

	addressIdxBucketName = []byte("addressidx")
	balanceBucketName    = []byte("balancex")
)

type DB interface {
	GetHeight() (int32, error)
	SetHeight(int32) error
	Get([]byte) (*Entry, error)
	PutBatch([]Entry) error
	GetBalance([]byte) (*Balance, error)
	PutBalance(*Balance) error
	GetIterator(prefix []byte) iterator.Iterator
	Close() error
}

type db struct {
	ldb *leveldb.DB
}

var _ DB = (*db)(nil)

func (l *db) putBatch(entries []Entry) error {
	batch := &leveldb.Batch{}
	bucketNameLen := len(addressIdxBucketName)

	for _, e := range entries {
		key := make([]byte, bucketNameLen+len(e.Address))
		copy(key[:bucketNameLen], addressIdxBucketName[:])
		copy(key[bucketNameLen:], e.Address[:])

		batch.Put(key, e.Address)
	}

	return l.ldb.Write(batch, nil)
}
func (l *db) GetHeight() (int32, error) {
	height := int64(0)
	data, err := l.ldb.Get([]byte(heightKey), nil)

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

func (l *db) SetHeight(height int32) error {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(buf, int64(height))
	return l.ldb.Put([]byte(heightKey), buf, nil)
}

func (l *db) Get(key []byte) (*Entry, error) {
	serialized, err := l.ldb.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return toEntry(key, serialized)
}

func (l *db) PutBatch(entries []Entry) error {
	batch := &leveldb.Batch{}

	for _, e := range entries {
		k, v := toPair(&e)
		batch.Put(k, v)
	}

	err := l.putBatch(entries)
	if err != nil {
		return err
	}

	return l.ldb.Write(batch, nil)
}

func (l *db) GetBalance(key []byte) (*Balance, error) {
	bucketNameLen := len(balanceBucketName)

	k := make([]byte, bucketNameLen+len(key))
	copy(k[:bucketNameLen], balanceBucketName[:])
	copy(k[bucketNameLen:], key[:])

	data, err := l.ldb.Get(k, nil)
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

	return &Balance{
		Key:   key,
		Value: value,
	}, nil
}

func (l *db) PutBalance(balance *Balance) error {
	bucketNameLen := len(balanceBucketName)

	key := make([]byte, bucketNameLen+len(balance.Key))
	copy(key[:bucketNameLen], balanceBucketName[:])
	copy(key[bucketNameLen:], balance.Key[:])

	value := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(value, balance.Value)

	return l.ldb.Put(key, value, nil)
}

func (l *db) GetIterator(prefix []byte) iterator.Iterator {
	return l.ldb.NewIterator(util.BytesPrefix(prefix), nil)
}

func (l *db) Close() error {
	return l.ldb.Close()
}

func NewDB(path string) (*db, error) {
	opts := opt.Options{
		Strict:      opt.DefaultStrict,
		Compression: opt.NoCompression,
		Filter:      filter.NewBloomFilter(10),
	}
	odb, err := leveldb.OpenFile(path, &opts)
	if err != nil {
		return nil, err
	}

	return &db{ldb: odb}, nil
}