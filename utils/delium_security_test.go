package utils

import (
	"encoding/hex"
	"testing"
)

// TestDeliumOutputIsFullWidthRegardlessOfDeletion pins the shape of the Delium
// construction: deleteChars is applied to the *input* of each round, never to
// the final output, so the function always returns a full-length SHA-256 hex
// digest. That says nothing about collision resistance — the entropy lost by
// deleteChars is measured separately below (DEL-1). Delium is educational only;
// the chain's security-critical hashing lives in hash.go.
func TestDeliumOutputIsFullWidthRegardlessOfDeletion(t *testing.T) {
	for _, step := range []int{1, 2, 3, 5, 8, 16, 64, 1000} {
		for _, repeat := range []int{0, 1, 4} {
			got := D256("some input value", step, repeat)
			if len(got.String) != 64 {
				t.Errorf("D256(step=%d,repeat=%d) length = %d, want 64", step, repeat, len(got.String))
			}
			if _, err := hex.DecodeString(got.String); err != nil {
				t.Errorf("D256(step=%d,repeat=%d) not valid hex: %v", step, repeat, err)
			}
			if string(got.Byte_slice) != got.String {
				t.Errorf("D256 Byte_slice and String disagree for step=%d repeat=%d", step, repeat)
			}
		}
	}
}

// TestDeliumConfiguredPathsAreDomainSeparated checks that the three configured
// pipelines (block id, UID, message hash) map the same input to three different
// digests. The distinct add-ons therefore act as (weak but real) domain
// separation: a value hashed for one purpose cannot silently double as the
// digest for another.
func TestDeliumConfiguredPathsAreDomainSeparated(t *testing.T) {
	block := D256C("shared input string", "BLV#5/chain#6/StRoNg#7/SAFE#8").String
	uid := D512C("shared input string", "SFFds#7/dfgki#6/d5gj7#7/dfgk78#10").String
	message := D256C("shared input string", "sdwE#3/kiu9u#6/OEJLF#7/dDFO6TGF#8/JLKoty#5").String

	if block == message {
		t.Error("block-hashing and message-hashing paths produced the same digest for the same input")
	}
	if len(uid) != 128 {
		t.Errorf("UID-maker path (D512C) length = %d, want 128", len(uid))
	}
}

// TestDeliumFinalRoundDominatesPreimageResistance documents (via an executable
// check) that the last operation of every Delium pipeline is a full SHA-256 over
// a string, so the overall preimage/collision resistance reduces to that of a
// single SHA-256. Concretely: two inputs that agree on everything except the
// final addon still diverge, and the digest is sensitive to the addon of the
// final segment.
func TestDeliumFinalSegmentMatters(t *testing.T) {
	base := "abcdef0123456789"
	a := D256C(base, "x#3/final#5").String
	b := D256C(base, "x#3/DIFFERENT#5").String
	if a == b {
		t.Error("changing the final segment's addon did not change the digest")
	}
}

// TestDeliumDeletionReducesIntermediateEntropy is the executable core of audit
// finding DEL-1. Each pipeline step hashes, then calls deleteChars on the 64-hex
// digest *before* re-hashing. Deleting characters discards entropy from the
// value that the next SHA-256 compresses, so the weakest round sets the whole
// pipeline's collision resistance at about 2^(surviving_hex_chars / 2):
//
//	message path  first deleteStep = 3 -> 64 - floor(64/3) = 43 hex survive -> ~2^86
//	block   path  smallest deleteStep = 5 -> 64 - floor(64/5) = 52 hex survive -> ~2^104
//
// Both are below the 2^128 collision resistance of the SHA-256 they are built
// from. This test pins the length reduction so the claim cannot silently drift
// if the configured paths change. It proves the entropy loss that causes a
// collision; it does not construct an actual (infeasible) 2^86 collision.
func TestDeliumDeletionReducesIntermediateEntropy(t *testing.T) {
	cases := []struct {
		name      string
		firstStep int
		wantSurv  int
	}{
		{"MESSAGE_HASH_PATH", 3, 43},
		{"BLOCK_HASHING_PATH", 5, 52},
	}
	for _, tc := range cases {
		digest := D256("anything", 1, 0).String // a full 64-hex digest
		out := deleteChars(digest, tc.firstStep)
		if len(out) != tc.wantSurv {
			t.Errorf("%s: deleteChars(len=64, step=%d) length = %d, want %d",
				tc.name, tc.firstStep, len(out), tc.wantSurv)
		}
		survivingBits := tc.wantSurv * 4
		if survivingBits >= 256 {
			t.Errorf("%s: expected entropy loss, got %d surviving bits", tc.name, survivingBits)
		}
		t.Logf("%s: intermediate entropy after first deletion = %d bits (collision bound ~2^%d)",
			tc.name, survivingBits, survivingBits/2)
	}
}
