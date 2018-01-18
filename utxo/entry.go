package utxo

import (
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

var ErrKeySize = fmt.Errorf("wrong key length")

type Entry struct {
	Address []byte
	TxHash  *chainhash.Hash
	Value   int64
}

func toPair(entry *Entry) (key, value []byte) {
	addressLen := len(entry.Address)

	key = make([]byte, addressLen+chainhash.HashSize)
	copy(key[:addressLen], entry.Address[:])
	copy(key[addressLen:], entry.TxHash.CloneBytes())

	value = make([]byte, binary.MaxVarintLen64)
	binary.LittleEndian.PutUint64(value[:], uint64(entry.Value))

	return
}

func toEntry(key, value []byte) (*Entry, error) {
	if len(key) < chainhash.HashSize {
		return nil, ErrKeySize
	}

	addressLen := len(key) - chainhash.HashSize
	address := make([]byte, addressLen)
	copy(address, key[:addressLen])

	hashBytes := make([]byte, chainhash.HashSize)
	copy(hashBytes, key[addressLen:])
	hash, err := chainhash.NewHash(hashBytes)
	if err != nil {
		return nil, err
	}

	return &Entry{
		Address: address,
		TxHash:  hash,
		Value:   int64(binary.LittleEndian.Uint64(value[:])),
	}, nil
}
