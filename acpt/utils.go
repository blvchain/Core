package acpt

import (
	"crypto/sha256"
)

// KeyValue is the standard data structure for updates
type KeyValue struct {
	Key   []byte
	Value []byte
}

// Hash performs a single SHA-256 hash
func Hash(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

// HashPair combines two hashes (Left + Right) -> Hash
func HashPair(left, right []byte) []byte {
	return Hash(append(left, right...))
}
