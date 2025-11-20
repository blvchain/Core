package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"

	"blvchain/core/config"
)

func Sign(hexPrivateKey string, message string) (string, error) {

	message_hash := D256C(message, config.DELIUM_CONFIG.MESSAGE_HASH_PATH).Byte_slice

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

func Verify(hexPublicKey string, uid string, message string, hexSignature string) (bool, error) {

	message_hash := D256C(message, config.DELIUM_CONFIG.MESSAGE_HASH_PATH).Byte_slice

	pubKeyCompressed, pubKeyCompressed_err := hex.DecodeString(hexPublicKey)
	if pubKeyCompressed_err != nil {
		return false, pubKeyCompressed_err
	}

	curve := elliptic.P256()

	x, y := elliptic.UnmarshalCompressed(curve, pubKeyCompressed)

	if x == nil || y == nil {
		return false, fmt.Errorf("password is too short")
	}

	publicKey := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	madeUID := Make_UID(hexPublicKey)

	if madeUID != uid {
		return false, fmt.Errorf("uid is not for this public key")
	}

	signatureBytes, err := hex.DecodeString(hexSignature)
	if err != nil {
		return false, err
	}

	r := new(big.Int).SetBytes(signatureBytes[:len(signatureBytes)/2])
	s := new(big.Int).SetBytes(signatureBytes[len(signatureBytes)/2:])

	valid := ecdsa.Verify(publicKey, message_hash, r, s)

	if valid {
		return valid, nil
	} else {
		return false, fmt.Errorf("signature is not valid")
	}
}
