package utils

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestMessageDigestIsRawSHA256Width(t *testing.T) {
	digest := MessageDigest("payload")
	if len(digest) != 32 {
		t.Fatalf("MessageDigest length = %d, want 32 (raw SHA-256)", len(digest))
	}
	if _, err := hex.DecodeString(hex.EncodeToString(digest)); err != nil {
		t.Fatalf("digest is not hex-encodable: %v", err)
	}
}

func TestHashDomainsAreSeparated(t *testing.T) {
	const input = "identical input bytes"

	block := HashDigestHex("blvchain/v2/block", input)
	uid := HashDigestHex("blvchain/v2/uid", input)
	message := HashDigestHex("blvchain/v2/message", input)

	if block == uid || block == message || uid == message {
		t.Fatal("different hash domains produced the same digest")
	}
}

func TestEncodeFieldsIsUnambiguous(t *testing.T) {
	// ("ab","c") and ("a","bc") collided under plain concatenation.
	if EncodeFields("ab", "c") == EncodeFields("a", "bc") {
		t.Fatal("EncodeFields is ambiguous")
	}
	if EncodeFields("a", "b") == EncodeFields("b", "a") {
		t.Fatal("EncodeFields ignores field order")
	}
	if EncodeFields("") == EncodeFields("", "") {
		t.Fatal("EncodeFields does not distinguish field count")
	}
}

func TestUIDDigestIsStableAndSized(t *testing.T) {
	pub := strings.Repeat("ab", 33)
	first := UIDDigest(pub)
	second := UIDDigest(pub)

	if first != second {
		t.Fatalf("UIDDigest is not deterministic: %q vs %q", first, second)
	}
	if len(first) != 32 {
		t.Fatalf("UID length = %d, want 32", len(first))
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Fatalf("UID is not hex: %v", err)
	}
	if UIDDigest(pub+"cd") == first {
		t.Fatal("UIDDigest ignored a changed public key")
	}
}

func TestBlockDigestChangesWithRoot(t *testing.T) {
	a := BlockDigest("root-a")
	b := BlockDigest("root-b")
	if a == b {
		t.Fatal("BlockDigest ignored the root")
	}
	if len(a) != 64 {
		t.Fatalf("block digest length = %d, want 64 hex characters", len(a))
	}
}

// TestProtocolHashVersionIsStable guards against an accidental change to the
// version tag: changing it re-keys the whole chain, so it must be deliberate.
func TestProtocolHashVersionIsStable(t *testing.T) {
	if ProtocolHashVersion != "blvchain/v2" {
		t.Fatalf("ProtocolHashVersion = %q; changing it is a chain-wide migration", ProtocolHashVersion)
	}
}
