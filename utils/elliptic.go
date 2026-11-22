package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"blvchain/core/config"
)

const (
	// PubKeyCompressedLen is the byte length of a compressed P-256 public key.
	PubKeyCompressedLen = 33
	// SignatureLen is the byte length of the r||s signature used by the chain.
	SignatureLen = 64
	// coordSize is the byte length of one P-256 scalar.
	coordSize = 32
)

// Sign signs message with the hex-encoded P-256 private key and returns the
// signature as 128 hex characters (r||s, both zero padded to 32 bytes).
func Sign(hexPrivateKey string, message string) (string, error) {
	privateKey, err := privkeyHexToECDSA(hexPrivateKey)
	if err != nil {
		return "", err
	}

	hash := D256C(message, config.DELIUM_CONFIG.MESSAGE_HASH_PATH).Byte_slice

	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	return hex.EncodeToString(encodeSignature(r, s)), nil
}

// Verify checks an r||s signature against the compressed public key. The UID is
// recomputed from the public key so a caller cannot swap in a different key.
func Verify(hexPublicKey string, uid string, message string, hexSignature string) (bool, error) {
	publicKey, err := ParsePublicKey(hexPublicKey)
	if err != nil {
		return false, err
	}

	if madeUID := Make_UID(hexPublicKey); madeUID != uid {
		return false, errors.New("uid is not for this public key")
	}

	r, s, err := decodeSignature(hexSignature)
	if err != nil {
		return false, err
	}

	hash := D256C(message, config.DELIUM_CONFIG.MESSAGE_HASH_PATH).Byte_slice

	if !ecdsa.Verify(publicKey, hash, r, s) {
		return false, errors.New("signature is not valid")
	}
	return true, nil
}

// ParsePublicKey decodes a 33-byte compressed P-256 public key.
func ParsePublicKey(hexPublicKey string) (*ecdsa.PublicKey, error) {
	raw, err := hex.DecodeString(hexPublicKey)
	if err != nil {
		return nil, fmt.Errorf("senderPubKey is not valid hex: %w", err)
	}
	if len(raw) != PubKeyCompressedLen {
		return nil, fmt.Errorf("senderPubKey must be %d bytes (33) compressed P-256", PubKeyCompressedLen)
	}

	curve := elliptic.P256()
	x, y := elliptic.UnmarshalCompressed(curve, raw)
	if x == nil || y == nil {
		return nil, errors.New("senderPubKey is not a valid P-256 point")
	}

	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

// PubKeyHex returns the compressed hex encoding of a public key. It is the
// inverse of ParsePublicKey and is handy for clients and tests.
func PubKeyHex(publicKey *ecdsa.PublicKey) string {
	if publicKey == nil || publicKey.Curve == nil {
		return ""
	}
	return hex.EncodeToString(elliptic.MarshalCompressed(publicKey.Curve, publicKey.X, publicKey.Y))
}

// PrivateKeyHex returns the hex encoding of a P-256 private key.
func PrivateKeyHex(privateKey *ecdsa.PrivateKey) string {
	if privateKey == nil {
		return ""
	}
	return hex.EncodeToString(privateKey.D.FillBytes(make([]byte, coordSize)))
}

func privkeyHexToECDSA(privkey string) (*ecdsa.PrivateKey, error) {
	privkey = strings.TrimSpace(privkey)
	if privkey == "" {
		return nil, errors.New("private key is empty")
	}

	d, ok := new(big.Int).SetString(privkey, 16)
	if !ok {
		return nil, errors.New("private key is not valid hex")
	}

	curve := elliptic.P256()
	if d.Sign() <= 0 || d.Cmp(curve.Params().N) >= 0 {
		return nil, errors.New("private key is out of range for P-256")
	}

	privateKey := new(ecdsa.PrivateKey)
	privateKey.PublicKey.Curve = curve
	privateKey.D = d
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	return privateKey, nil
}

// encodeSignature pads r and s to 32 bytes each. The previous implementation
// concatenated the minimal big-endian encodings, which produced a variable
// length signature and made verification ambiguous whenever r or s had a leading
// zero byte.
func encodeSignature(r, s *big.Int) []byte {
	out := make([]byte, SignatureLen)
	r.FillBytes(out[:coordSize])
	s.FillBytes(out[coordSize:])
	return out
}

func decodeSignature(hexSignature string) (*big.Int, *big.Int, error) {
	raw, err := hex.DecodeString(hexSignature)
	if err != nil {
		return nil, nil, fmt.Errorf("signature is not valid hex: %w", err)
	}
	if len(raw) != SignatureLen {
		return nil, nil, fmt.Errorf("signature must be %d bytes (r||s), got %d", SignatureLen, len(raw))
	}

	r := new(big.Int).SetBytes(raw[:coordSize])
	s := new(big.Int).SetBytes(raw[coordSize:])

	if r.Sign() <= 0 || s.Sign() <= 0 {
		return nil, nil, errors.New("signature values must be positive")
	}

	return r, s, nil
}
