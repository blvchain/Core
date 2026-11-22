package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
)

// testKey generates a fresh P-256 key and returns its hex encodings.
func testKey(t *testing.T) (privHex, pubHex, uid string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privHex = PrivateKeyHex(priv)
	pubHex = PubKeyHex(&priv.PublicKey)
	if len(privHex) != 64 || len(pubHex) != 66 {
		t.Fatalf("unexpected key lengths: priv=%d pub=%d", len(privHex), len(pubHex))
	}

	return privHex, pubHex, Make_UID(pubHex)
}

func TestSignVerifyRoundTrip(t *testing.T) {
	privHex, pubHex, uid := testKey(t)

	message := "sender:alice|receiver:bob|payload:hello"
	signature, err := Sign(privHex, message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(signature) != 128 {
		t.Fatalf("signature must be 128 hex characters, got %d", len(signature))
	}

	ok, err := Verify(pubHex, uid, message, signature)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("Verify rejected a valid signature")
	}
}

func TestSignIsRandomisedButVerifiable(t *testing.T) {
	privHex, pubHex, uid := testKey(t)

	// ECDSA uses a random nonce, so two signatures over the same message differ.
	first, err := Sign(privHex, "same message")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	second, err := Sign(privHex, "same message")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if first == second {
		t.Fatal("expected two different ECDSA signatures")
	}

	for _, signature := range []string{first, second} {
		ok, err := Verify(pubHex, uid, "same message", signature)
		if err != nil || !ok {
			t.Fatalf("Verify(%q) = %v, %v", signature, ok, err)
		}
	}
}

func TestVerifyRejectsTamperedMessage(t *testing.T) {
	privHex, pubHex, uid := testKey(t)

	signature, err := Sign(privHex, "original")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	ok, err := Verify(pubHex, uid, "tampered", signature)
	if ok || err == nil {
		t.Fatalf("Verify accepted a tampered message (ok=%v err=%v)", ok, err)
	}
}

func TestVerifyRejectsWrongUID(t *testing.T) {
	privHex, pubHex, _ := testKey(t)

	signature, err := Sign(privHex, "payload")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	ok, err := Verify(pubHex, strings.Repeat("a", 32), "payload", signature)
	if ok || err == nil {
		t.Fatalf("Verify accepted a UID that does not belong to the key (ok=%v err=%v)", ok, err)
	}
}

func TestVerifyRejectsSignatureFromAnotherKey(t *testing.T) {
	privA, _, _ := testKey(t)
	_, pubB, uidB := testKey(t)

	signature, err := Sign(privA, "payload")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	ok, err := Verify(pubB, uidB, "payload", signature)
	if ok || err == nil {
		t.Fatalf("Verify accepted a signature made with another key (ok=%v err=%v)", ok, err)
	}
}

func TestVerifyRejectsMalformedSignatures(t *testing.T) {
	privHex, pubHex, uid := testKey(t)

	valid, err := Sign(privHex, "payload")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	raw, _ := hex.DecodeString(valid)
	flipped := append([]byte(nil), raw...)
	flipped[0] ^= 0xFF

	cases := map[string]string{
		"empty":         "",
		"not hex":       "zzzz",
		"too short":     "00",
		"zero scalar":   strings.Repeat("00", 64),
		"bit flipped":   hex.EncodeToString(flipped),
		"odd hex digit": strings.Repeat("0", 127),
	}

	for name, signature := range cases {
		ok, err := Verify(pubHex, uid, "payload", signature)
		if ok || err == nil {
			t.Errorf("%s: Verify accepted an invalid signature (ok=%v err=%v)", name, ok, err)
		}
	}
}

func TestSignRejectsInvalidPrivateKeys(t *testing.T) {
	curve := elliptic.P256()

	cases := map[string]string{
		"empty":       "",
		"whitespace":  "   ",
		"not hex":     "zzzz",
		"zero":        "0",
		"curve order": hex.EncodeToString(curve.Params().N.Bytes()),
	}

	for name, key := range cases {
		if _, err := Sign(key, "payload"); err == nil {
			t.Errorf("%s: Sign accepted an invalid private key", name)
		}
	}
}

func TestParsePublicKey(t *testing.T) {
	_, pubHex, _ := testKey(t)

	if _, err := ParsePublicKey(pubHex); err != nil {
		t.Fatalf("ParsePublicKey rejected a valid key: %v", err)
	}

	cases := map[string]string{
		"not hex":     "zzzz",
		"too short":   "02" + strings.Repeat("00", 31),
		"empty":       "",
		"not a point": "02" + strings.Repeat("ff", 32),
	}
	for name, key := range cases {
		if _, err := ParsePublicKey(key); err == nil {
			t.Errorf("%s: ParsePublicKey accepted an invalid key", name)
		}
	}
}

func TestPubKeyHexRoundTrip(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pubHex := PubKeyHex(&priv.PublicKey)
	parsed, err := ParsePublicKey(pubHex)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	if parsed.X.Cmp(priv.PublicKey.X) != 0 || parsed.Y.Cmp(priv.PublicKey.Y) != 0 {
		t.Fatal("round trip changed the public key")
	}
	if PrivateKeyHex(nil) != "" || PubKeyHex(nil) != "" {
		t.Fatal("nil keys should encode to the empty string")
	}
}

func TestMakeUIDIsStable(t *testing.T) {
	_, pubHex, _ := testKey(t)

	first := Make_UID(pubHex)
	second := Make_UID(pubHex)
	if first != second {
		t.Fatalf("Make_UID is not deterministic: %q vs %q", first, second)
	}
	if len(first) != 32 {
		t.Fatalf("UID must be 32 characters, got %d", len(first))
	}
}

func TestSignVerifyEmptyMessage(t *testing.T) {
	privHex, pubHex, uid := testKey(t)

	signature, err := Sign(privHex, "")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	ok, err := Verify(pubHex, uid, "", signature)
	if err != nil || !ok {
		t.Fatalf("Verify = %v, %v", ok, err)
	}
}
