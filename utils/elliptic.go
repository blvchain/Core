package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"

	"blvchain/core/config"
)

func Verify(publicKeyByte []byte, uid []byte, message []byte, signatureBytes []byte) (bool, error) {

	message_hash, _ := D256C(ToMongoBinary(message), config.DELIUM_CONFIG.MESSAGE_HASH_PATH)

	curve := elliptic.P256()

	x, y := elliptic.UnmarshalCompressed(curve, publicKeyByte)

	if x == nil || y == nil {
		return false, fmt.Errorf("password is too short")
	}

	publicKey := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	madeUID, _ := Make_UID(ToMongoBinary(message))

	if !bytes.Equal(madeUID.Byte_slice, uid) {
		return false, fmt.Errorf("uid is not for this public key")
	}

	r := new(big.Int).SetBytes(signatureBytes[:len(signatureBytes)/2])
	s := new(big.Int).SetBytes(signatureBytes[len(signatureBytes)/2:])

	valid := ecdsa.Verify(publicKey, message_hash.Byte_slice, r, s)

	if valid {
		return valid, nil
	} else {
		return false, fmt.Errorf("signature is not valid")
	}
}
