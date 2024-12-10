package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"math/big"

	"blvchain/core/config"
)

func Sign(hexPrivateKey string, message string) (string, error) {

	message_hash := D256(message, config.DELIUM_CONFIG.MESSAGE.DELETE_STEP, config.DELIUM_CONFIG.MESSAGE.REPEAT).Byte_slice

	privateKey, privateKey_err := privkeyHexToECDSA(hexPrivateKey)
	if privateKey_err != nil {
		return "", privateKey_err
	}

	r, s, err := ecdsa.Sign(rand.Reader, privateKey, message_hash[:])
	if err != nil {
		return "", err
	}

	signature := append(r.Bytes(), s.Bytes()...)

	signatureHex := hex.EncodeToString(signature)

	return signatureHex, nil
}

func Verify(hexPublicKey string, message string, hexSignature string) (bool, error) {

	message_hash := D256(message, config.DELIUM_CONFIG.MESSAGE.DELETE_STEP, config.DELIUM_CONFIG.MESSAGE.REPEAT).Byte_slice

	pubKeyCompressed, _ := hex.DecodeString(hexPublicKey)
	curve := elliptic.P256()

	x, y := elliptic.UnmarshalCompressed(curve, pubKeyCompressed)

	publicKey := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	signatureBytes, err := hex.DecodeString(hexSignature)
	if err != nil {
		return false, err
	}

	r := new(big.Int).SetBytes(signatureBytes[:len(signatureBytes)/2])
	s := new(big.Int).SetBytes(signatureBytes[len(signatureBytes)/2:])

	valid := ecdsa.Verify(publicKey, message_hash, r, s)

	return valid, nil
}
