package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// This file is the protocol's *cryptographic* hashing layer.
//
// It is deliberately separate from Delium (delium.go). Delium is an
// educational hash pipeline kept for teaching purposes only; it is not used for
// block IDs, UIDs, message digests or signatures. The reasons are set out in
// AUDIT_REPORT.md (DEL-1): its deleteChars step lowers collision resistance
// below the SHA-256 it wraps. Everything security-critical in the chain goes
// through the functions below, which are plain SHA-256 with domain separation.
//
// Domain separation: every digest is SHA-256(domain || 0x00 || data). The
// domain tag means a value hashed for one purpose (say a block ID) can never be
// reinterpreted as a value hashed for another (say a message digest), even if
// the underlying bytes are identical.
const (
	// ProtocolHashVersion is the identifier of the hashing profile used by this
	// build. It is part of every domain tag, so a future change to the hashing
	// scheme cannot silently collide with the current one. It is also what the
	// migration notes refer to.
	ProtocolHashVersion = "blvchain/v2"

	domainBlock   = ProtocolHashVersion + "/block"
	domainUID     = ProtocolHashVersion + "/uid"
	domainMessage = ProtocolHashVersion + "/message"
)

// HashDigest returns SHA-256(domain || 0x00 || data) as raw bytes.
func HashDigest(domain, data string) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte{0})
	h.Write([]byte(data))
	return h.Sum(nil)
}

// HashDigestHex is HashDigest rendered as lower-case hex.
func HashDigestHex(domain, data string) string {
	return hex.EncodeToString(HashDigest(domain, data))
}

// MessageDigest returns the 32-byte digest that ECDSA signs for a message.
//
// Signing the raw 32-byte digest (rather than, say, 64 ASCII hex characters)
// means the full 256 bits reach the curve arithmetic. The earlier implementation
// hashed to hex text and relied on ECDSA truncating it, which silently used only
// the first 128 bits (CRY-2).
func MessageDigest(message string) []byte {
	return HashDigest(domainMessage, message)
}

// BlockDigest returns the hex block identifier for an already canonically
// encoded block root. See db.BlockHashMaker for how the root is built.
func BlockDigest(root string) string {
	return HashDigestHex(domainBlock, root)
}

// UIDDigest derives the 32-hex-character node/client identifier from a
// compressed public key. It is deterministic: the same key always maps to the
// same UID. 128 bits is plenty for an identifier that is always checked against
// the key it was derived from.
func UIDDigest(pubkeyHex string) string {
	return HashDigestHex(domainUID, pubkeyHex)[:32]
}

// EncodeFields produces an unambiguous, canonical encoding of fields.
//
// Each field is written as "<decimal length>:<bytes>;". Length-prefixing means
// no two different field tuples can produce the same string, which is what the
// previous plain concatenation could not guarantee (e.g. ("ab","c") and
// ("a","bc") used to collide). This encoding is used for both the signed
// message and the block hash root, so both are unambiguous.
func EncodeFields(fields ...string) string {
	var b strings.Builder
	for _, field := range fields {
		b.WriteString(strconv.Itoa(len(field)))
		b.WriteByte(':')
		b.WriteString(field)
		b.WriteByte(';')
	}
	return b.String()
}
