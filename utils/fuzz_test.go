package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

// FuzzVerifyNoPanic exercises the signature path with arbitrary messages. It
// asserts the round-trip property and that a tampered message never verifies.
func FuzzVerifyNoPanic(f *testing.F) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		f.Fatalf("generate key: %v", err)
	}
	privHex := PrivateKeyHex(priv)
	pubHex := PubKeyHex(&priv.PublicKey)
	uid := Make_UID(pubHex)

	f.Add("hello")
	f.Add("")
	f.Add("sender:alice|receiver:bob|payload:hello")

	f.Fuzz(func(t *testing.T, message string) {
		signature, err := Sign(privHex, message)
		if err != nil {
			t.Fatalf("Sign(%q): %v", message, err)
		}

		ok, err := Verify(pubHex, uid, message, signature)
		if err != nil || !ok {
			t.Fatalf("Verify(%q) = %v, %v", message, ok, err)
		}

		// Any change to the message must invalidate the signature.
		if ok, _ := Verify(pubHex, uid, message+"\x00", signature); ok {
			t.Fatalf("Verify accepted a tampered message for %q", message)
		}
	})
}

// FuzzParsePublicKey ensures malformed key encodings return errors rather than
// panicking.
func FuzzParsePublicKey(f *testing.F) {
	f.Add("")
	f.Add("00")
	f.Add("02" + "00000000000000000000000000000000000000000000000000000000000000ff")

	f.Fuzz(func(t *testing.T, encoded string) {
		// Any result is fine; the point is that it does not panic.
		_, _ = ParsePublicKey(encoded)
	})
}
