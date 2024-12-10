package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"math/big"

	"blvchain/core/config"
)

func Make_Privkey_Pubkey(mnemonic string) (string, string) {

	curve := elliptic.P256()
	curveOrder := curve.Params().N

	seed := D256C(mnemonic, config.DELIUM_SEED_PATH).String

	privateKey := new(ecdsa.PrivateKey)
	privateKey.PublicKey.Curve = curve
	privateKeyBigInt, _ := new(big.Int).SetString(seed, 16)
	privateKey.D = new(big.Int).Mod(privateKeyBigInt, curveOrder)
	if privateKey.D.Sign() == 0 {
		privateKey.D.Add(privateKey.D, big.NewInt(1))
	}
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(privateKey.D.Bytes())
	privkeyStr := hex.EncodeToString(privateKey.D.Bytes())

	pubkeyX := privateKey.PublicKey.X
	pubkeyY := privateKey.PublicKey.Y
	prefix := byte(0x02)
	if pubkeyY.Bit(0) == 1 {
		prefix = 0x03
	}
	compressedPubKey := make([]byte, 1+len(pubkeyX.Bytes()))
	compressedPubKey[0] = prefix
	copy(compressedPubKey[1:], pubkeyX.Bytes())
	pubkeyStr := hex.EncodeToString(compressedPubKey)

	return privkeyStr, pubkeyStr
}

func Sign(hexPrivateKey string, message string) (string, error) {

	message_hash := D256(message, config.MESSAGE_DELIUM_CONFIG.Delete_step, config.MESSAGE_DELIUM_CONFIG.Repeat).Byte_slice

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

	message_hash := D256(message, config.MESSAGE_DELIUM_CONFIG.Delete_step, config.MESSAGE_DELIUM_CONFIG.Repeat).Byte_slice

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

func Make_UID(pubkey_str string) string {
	hash := D512(pubkey_str, config.WALLET_DELIUM_CONFIG.Delete_step, config.WALLET_DELIUM_CONFIG.Repeat).String
	return hash[:32]
}
