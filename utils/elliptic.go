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

	// Sign the raw 32-byte SHA-256 digest (CRY-2). Domain separation is applied
	// inside MessageDigest so a signature can never be replayed in another
	// context (block hash, UID derivation, ...).
	hash := MessageDigest(message)

	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	return hex.EncodeToString(encodeSignature(normaliseS(r, s))), nil
}

// normaliseS returns (r, s) with s in the lower half of the P-256 group order.
//
// ECDSA is malleable: for any valid (r, s) the pair (r, N-s) is also a valid
// signature over the same message (N is the curve order). Go's ecdsa package
// does not canonicalise this, so without normalisation a third party who sees a
// signature could rewrite s into N-s and produce a *second* valid signature for
// the same block. Because the signature is part of the block hash, that would
// yield a distinct block ID for identical content and defeat the duplicate-block
// guard. Producing only low-S signatures (and rejecting high-S ones in
// decodeSignature, see below) removes this malleability, the same approach used
// by Bitcoin (BIP-62) and Ethereum (EIP-2).
func normaliseS(r, s *big.Int) (*big.Int, *big.Int) {
	n := elliptic.P256().Params().N
	half := new(big.Int).Rsh(n, 1)
	if s.Cmp(half) > 0 {
		s = new(big.Int).Sub(n, s)
	}
	return r, s
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

	hash := MessageDigest(message)

	if !ecdsa.Verify(publicKey, hash, r, s) {
		return false, errors.New("signature is not valid")
	}
	return true, nil
}

// VerifySignature checks an r||s signature against a compressed public key,
// without the UID-binding check that Verify performs.
//
// It exists for node authentication (NET-2): a node's UID is an
// operator-assigned label from dns_seed.json, not a key-derived identifier, so
// the Make_UID(pubkey) == uid rule does not apply there. For record signatures,
// use Verify.
func VerifySignature(hexPublicKey string, message string, hexSignature string) (bool, error) {
	publicKey, err := ParsePublicKey(hexPublicKey)
	if err != nil {
		return false, err
	}

	r, s, err := decodeSignature(hexSignature)
	if err != nil {
		return false, err
	}

	if !ecdsa.Verify(publicKey, MessageDigest(message), r, s) {
		return false, errors.New("signature is not valid")
	}
	return true, nil
}

// DerivePubKeyHex returns the compressed hex encoding of the public key that
// belongs to hexPrivateKey.
func DerivePubKeyHex(hexPrivateKey string) (string, error) {
	privateKey, err := privkeyHexToECDSA(hexPrivateKey)
	if err != nil {
		return "", err
	}
	return PubKeyHex(&privateKey.PublicKey), nil
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

	// Reject non-canonical (malleated) signatures. r and s must both be in
	// [1, N-1], and s must be in the lower half [1, N/2]. This complements
	// normaliseS in Sign so the chain only ever carries one valid encoding per
	// (message, key) pair.
	n := elliptic.P256().Params().N
	half := new(big.Int).Rsh(n, 1)
	if r.Cmp(n) >= 0 {
		return nil, nil, errors.New("signature r is out of range")
	}
	if s.Cmp(half) > 0 {
		return nil, nil, errors.New("signature s is not in canonical low-S form")
	}

	return r, s, nil
}
