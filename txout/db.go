package txout

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/goleveldb/leveldb"
)

var (
	heightKey = "height"
)

type DB interface {
	Get(key []byte) (*wire.TxOut, error)
	Put(key []byte, tx *wire.TxOut) error
	GetHeight() (int32, error)
	SetHeight(int32) error
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
	serialized := make([]byte, len(tx.PkScript)+binary.MaxVarintLen64)
	binary.LittleEndian.PutUint64(serialized[0:binary.MaxVarintLen64], uint64(tx.Value))
	copy(serialized[binary.MaxVarintLen64:], tx.PkScript[:])

	return o.ldb.Put(key, serialized[:], nil)
}

func (o *db) Close() error {
	return o.ldb.Close()
}

func NewDB(path string) (*db, error) {
	odb, err := leveldb.OpenFile(path, nil)

	if err != nil {
		return nil, err
	}

	return &db{ldb: odb}, nil
}
