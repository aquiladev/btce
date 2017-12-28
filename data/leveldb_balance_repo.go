package data

import (
	"bytes"
	"encoding/binary"

	"github.com/btcsuite/goleveldb/leveldb"
)

type LevelDbBalanceRepository struct {
	db *leveldb.DB
}

func NewLevelDbBalanceRepository(path string) (*LevelDbBalanceRepository, error) {
	db, err := leveldb.OpenFile(path, nil)

	if err != nil {
		return nil, err
	}

	return &LevelDbBalanceRepository{
		db: db,
	}, nil
}

func (t *LevelDbBalanceRepository) Get(publicKey string) (*Balance, error) {
	data, err := t.db.Get([]byte(publicKey), nil)
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

func (t *LevelDbBalanceRepository) Insert(balance *Balance) error {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(buf, balance.Value)
	return t.db.Put([]byte(balance.PublicKey), buf, nil)
}

func (t *LevelDbBalanceRepository) Update(balance *Balance) error {
	return t.Insert(balance)
}