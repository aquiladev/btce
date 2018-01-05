package ledger

import (
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

var ErrKeySize = fmt.Errorf("wrong key length")

type Entry struct {
	Address []byte
	TxHash  *chainhash.Hash
	In      bool
	Value   int64
}

func toPair(entry *Entry) (key, value []byte) {
	addressLen := len(entry.Address)

	key = make([]byte, addressLen+1+chainhash.HashSize)
	copy(key[:addressLen], entry.Address[:])
	key[addressLen] = convertBoolToByte(entry.In)
	copy(key[addressLen+1:], entry.TxHash.CloneBytes())

	value = make([]byte, binary.MaxVarintLen64)
	binary.LittleEndian.PutUint64(value[:], uint64(entry.Value))

	return
}

func ToEntry(key, value []byte) (*Entry, error) {
	if len(key) < chainhash.HashSize {
		return nil, ErrKeySize
	}

	addressLen := len(key) - chainhash.HashSize - 1
	address := make([]byte, addressLen)
	copy(address, key[:addressLen])

	hashBytes := make([]byte, chainhash.HashSize)
	copy(hashBytes, key[addressLen+1:])
	hash, err := chainhash.NewHash(hashBytes)
	if err != nil {
		return nil, err
	}

	return &Entry{
		Address: address,
		TxHash:  hash,
		In:      convertByteToBool(key[addressLen]),
		Value:   int64(binary.LittleEndian.Uint64(value[:])),
	}, nil
}

func convertBoolToByte(value bool) byte {
	if value {
		return 0x01
	}
	return 0x00
}

func convertByteToBool(value byte) bool {
	return value == 0x01
}
