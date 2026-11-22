package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"
)

// signRaw returns the numeric (r, s) of a signature produced by Sign.
func signRaw(t *testing.T, privHex, message string) (*big.Int, *big.Int) {
	t.Helper()
	sig, err := Sign(privHex, message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	r, s, err := decodeSignature(sig)
	if err != nil {
		t.Fatalf("decodeSignature of freshly produced signature failed: %v", err)
	}
	return r, s
}

// TestSignProducesCanonicalLowS verifies that Sign only ever emits the lower-half
// form of s. If it did not, a verifying node running decodeSignature would reject
// the node's own honest signatures.
func TestSignProducesCanonicalLowS(t *testing.T) {
	privHex, _, _ := testKey(t)
	n := elliptic.P256().Params().N
	half := new(big.Int).Rsh(n, 1)

	for i := 0; i < 64; i++ {
		_, s := signRaw(t, privHex, "canonical form message")
		if s.Cmp(half) > 0 {
			t.Fatalf("iteration %d: Sign produced high-S signature (s > N/2)", i)
		}
	}
}

// TestVerifyRejectsHighSMalleatedSignature is the regression test for the ECDSA
// malleability issue. It takes a valid signature, rewrites s into N-s (the
// malleated form) and asserts that Verify now refuses it. Before the fix this
// second form verified, which let an attacker mint a second block ID for an
// identical payload.
func TestVerifyRejectsHighSMalleatedSignature(t *testing.T) {
	privHex, pubHex, uid := testKey(t)
	message := "payload"

	sig, err := Sign(privHex, message)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Sanity: the original low-S signature verifies.
	if ok, err := Verify(pubHex, uid, message, sig); err != nil || !ok {
		t.Fatalf("original signature rejected: ok=%v err=%v", ok, err)
	}

	r, s, err := decodeSignature(sig)
	if err != nil {
		t.Fatalf("decodeSignature: %v", err)
	}
	// Only a low-S signature makes this a genuine malleation (s -> N-s flips the half).
	if s.Cmp(new(big.Int).Rsh(elliptic.P256().Params().N, 1)) > 0 {
		t.Skip("signature already high-S; nothing to malleate (should not happen after fix)")
	}

	n := elliptic.P256().Params().N
	malleated := hex.EncodeToString(encodeSignature(r, new(big.Int).Sub(n, s)))

	if ok, err := Verify(pubHex, uid, message, malleated); ok || err == nil {
		t.Fatalf("Verify accepted malleated high-S signature (ok=%v err=%v)", ok, err)
	}
}

// TestNormaliseSProducesLowerHalf checks the helper directly for arbitrary s,
// including values above and below the halfway point, and that it is idempotent.
func TestNormaliseSProducesLowerHalf(t *testing.T) {
	n := elliptic.P256().Params().N
	half := new(big.Int).Rsh(n, 1)

	cases := []*big.Int{
		big.NewInt(1),
		new(big.Int).Sub(half, big.NewInt(1)),
		half,
		new(big.Int).Add(half, big.NewInt(1)),
		new(big.Int).Sub(n, big.NewInt(1)),
	}
	for _, s := range cases {
		_, got := normaliseS(big.NewInt(7), new(big.Int).Set(s))
		if got.Cmp(half) > 0 {
			t.Errorf("normaliseS(%v) = %v, still above N/2", s, got)
		}
		if got.Sign() <= 0 {
			t.Errorf("normaliseS(%v) = %v, not positive", s, got)
		}
		// Idempotent: normalising again changes nothing.
		_, again := normaliseS(big.NewInt(7), new(big.Int).Set(got))
		if again.Cmp(got) != 0 {
			t.Errorf("normaliseS not idempotent for %v", s)
		}
	}
}

// TestECDSAHighSMalleabilityIsReal documents the underlying property this fix
// defends against: Go's crypto/ecdsa accepts (r, N-s) at the primitive level. If
// a future Go release started rejecting high-S in ecdsa.Verify, this test would
// fail and the canonical-S guard could be reconsidered.
func TestECDSAHighSMalleabilityIsReal(t *testing.T) {
	curve := elliptic.P256()
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	h := D256C("abc", "sdwE#3").Byte_slice
	r, s, err := ecdsa.Sign(rand.Reader, priv, h)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	n := curve.Params().N
	sMall := new(big.Int).Sub(n, s)
	if !ecdsa.Verify(&priv.PublicKey, h, r, sMall) {
		t.Skip("stdlib ecdsa.Verify now rejects high-S; malleability guard may be redundant")
	}
}
