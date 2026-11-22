package utils

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"strconv"
	"strings"
)

// This file implements Delium, the project's original educational hash
// pipeline. It is deliberately kept as a teaching artefact and is **not** used
// for anything security-critical. Block IDs, UIDs, message digests and
// signatures all use the domain-separated SHA-256 in hash.go instead.
//
// Why Delium is not a cryptographic primitive: every pipeline step hashes, then
// calls deleteChars on the 64-hex digest before re-hashing. Deleting characters
// discards entropy from the value the next SHA-256 compresses, so the weakest
// round caps the whole pipeline's collision resistance below the 2^128 of the
// SHA-256 it wraps (~2^86 for the configured message path, ~2^104 for the block
// path). See AUDIT_REPORT.md (DEL-1). Do not use Delium where collisions matter.

// D_hash is the result of a Delium hash. String is the hex digest and
// Byte_slice holds the same digest as raw bytes (still ASCII hex).
type D_hash struct {
	Byte_slice []byte
	String     string
}

// D256 hashes strData with SHA-256, then applies `repeat` rounds of
// "remove every deleteStep-th character, hash again". A deleteStep below 2 is
// treated as "do not delete anything" so the function can never panic.
func D256(strData string, deleteStep int, repeat int) *D_hash {
	current := sha256Hex(strData)
	for i := 0; i < repeat; i++ {
		current = sha256Hex(deleteChars(current, deleteStep))
	}
	return &D_hash{Byte_slice: []byte(current), String: current}
}

// D512 is D256 with SHA-512.
func D512(strData string, deleteStep int, repeat int) *D_hash {
	current := sha512Hex(strData)
	for i := 0; i < repeat; i++ {
		current = sha512Hex(deleteChars(current, deleteStep))
	}
	return &D_hash{Byte_slice: []byte(current), String: current}
}

// D256C runs the configured Delium pipeline described by path. Each segment has
// the form "addon#deleteStep"; the digest is salted with addon and then hashed
// again. Malformed segments are skipped instead of aborting the process, so a
// typo in delium_config.json cannot take the node down.
func D256C(strData string, path string) *D_hash {
	current := sha256Hex(strData)
	for _, step := range parseHashPath(path) {
		current = sha256Hex(current + step.addon)
		current = sha256Hex(deleteChars(current, step.deleteStep))
	}
	return &D_hash{Byte_slice: []byte(current), String: current}
}

// D512C is D256C with SHA-512.
func D512C(strData string, path string) *D_hash {
	current := sha512Hex(strData)
	for _, step := range parseHashPath(path) {
		current = sha512Hex(current + step.addon)
		current = sha512Hex(deleteChars(current, step.deleteStep))
	}
	return &D_hash{Byte_slice: []byte(current), String: current}
}

type hashStep struct {
	addon      string
	deleteStep int
}

func parseHashPath(path string) []hashStep {
	var steps []hashStep
	for _, part := range strings.Split(path, "/") {
		fields := strings.Split(part, "#")
		if len(fields) != 2 || fields[0] == "" {
			continue
		}
		step, err := strconv.Atoi(fields[1])
		if err != nil || step < 1 {
			continue
		}
		steps = append(steps, hashStep{addon: fields[0], deleteStep: step})
	}
	return steps
}

// deleteChars removes every step-th character (1-based) from s.
func deleteChars(s string, step int) string {
	if step < 2 || len(s) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if (i+1)%step != 0 {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sha512Hex(s string) string {
	sum := sha512.Sum512([]byte(s))
	return hex.EncodeToString(sum[:])
}
