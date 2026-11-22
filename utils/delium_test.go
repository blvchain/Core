package utils

import (
	"strings"
	"testing"

	"blvchain/core/config"
)

func TestD256IsDeterministic(t *testing.T) {
	first := D256("hello", 5, 3)
	second := D256("hello", 5, 3)

	if first.String != second.String {
		t.Fatalf("same input produced different digests: %q vs %q", first.String, second.String)
	}
	if len(first.String) != 64 {
		t.Fatalf("SHA-256 hex digest must be 64 characters, got %d", len(first.String))
	}
	if string(first.Byte_slice) != first.String {
		t.Fatalf("Byte_slice and String disagree: %q vs %q", first.Byte_slice, first.String)
	}
}

func TestD256ChangesWithInput(t *testing.T) {
	if D256("hello", 5, 1).String == D256("world", 5, 1).String {
		t.Fatal("different inputs produced the same digest")
	}
}

func TestD256ChangesWithDeleteStep(t *testing.T) {
	if D256("hello", 5, 2).String == D256("hello", 7, 2).String {
		t.Fatal("different delete steps produced the same digest")
	}
}

func TestD256ZeroDeleteStepDoesNotPanic(t *testing.T) {
	// A deleteStep of 0 used to divide by zero and crash the node.
	for _, step := range []int{0, 1, -3} {
		if got := D256("hello", step, 1).String; len(got) != 64 {
			t.Fatalf("deleteStep %d produced %q", step, got)
		}
	}
}

func TestD512Length(t *testing.T) {
	got := D512("hello", 5, 2)
	if len(got.String) != 128 {
		t.Fatalf("SHA-512 hex digest must be 128 characters, got %d", len(got.String))
	}
}

func TestD256CMatchesManualPipeline(t *testing.T) {
	path := "a#3/b#5"

	// Reference implementation: hash, salt, delete, hash, salt, delete, hash.
	first := sha256Hex("payload")
	first = sha256Hex(first + "a")
	first = sha256Hex(deleteChars(first, 3))
	first = sha256Hex(first + "b")
	first = sha256Hex(deleteChars(first, 5))

	if got := D256C("payload", path).String; got != first {
		t.Fatalf("D256C = %q, want %q", got, first)
	}
}

func TestD256CMalformedPathIsSkipped(t *testing.T) {
	// A malformed segment used to call log.Fatal and kill the process.
	for _, path := range []string{"", "no-hash", "a#", "#5", "a#0", "a#abc", "a#5/broken"} {
		got := D256C("payload", path)
		if len(got.String) != 64 {
			t.Fatalf("path %q produced %q", path, got.String)
		}
	}
}

func TestD512CMatchesManualPipeline(t *testing.T) {
	path := "x#4"
	first := sha512Hex("payload")
	first = sha512Hex(first + "x")
	first = sha512Hex(deleteChars(first, 4))

	if got := D512C("payload", path).String; got != first {
		t.Fatalf("D512C = %q, want %q", got, first)
	}
}

func TestDeleteChars(t *testing.T) {
	cases := []struct {
		in   string
		step int
		want string
	}{
		{"abcdef", 2, "ace"},
		{"abcdef", 3, "abde"},
		{"abcdef", 7, "abcdef"},
		{"abcdef", 1, "abcdef"}, // step < 2 means "do not delete"
		{"", 2, ""},
	}
	for _, tc := range cases {
		if got := deleteChars(tc.in, tc.step); got != tc.want {
			t.Errorf("deleteChars(%q, %d) = %q, want %q", tc.in, tc.step, got, tc.want)
		}
	}
}

func TestConfiguredPipelineIsValid(t *testing.T) {
	if err := config.ValidateDeliumPath(config.DELIUM_CONFIG.BLOCK_HASHING_PATH); err != nil {
		t.Fatalf("shipped BLOCK_HASHING_PATH is invalid: %v", err)
	}
	if err := config.ValidateDeliumPath(config.DELIUM_CONFIG.UID_MAKER_PATH); err != nil {
		t.Fatalf("shipped UID_MAKER_PATH is invalid: %v", err)
	}
	if err := config.ValidateDeliumPath(config.DELIUM_CONFIG.MESSAGE_HASH_PATH); err != nil {
		t.Fatalf("shipped MESSAGE_HASH_PATH is invalid: %v", err)
	}
}

func TestD256CConfiguredPathLength(t *testing.T) {
	got := D256C("payload", config.DELIUM_CONFIG.BLOCK_HASHING_PATH).String
	if len(got) != 64 {
		t.Fatalf("digest length = %d, want 64", len(got))
	}
	if strings.ContainsAny(got, "/#") {
		t.Fatalf("digest contains path separators: %q", got)
	}
}
