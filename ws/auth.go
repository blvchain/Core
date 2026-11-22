package ws

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"blvchain/core/config"
	"blvchain/core/utils"
)

// protocolAuthTag domain-separates the peer-authentication message from record
// signatures and every other hash in the system.
const protocolAuthTag = "blvchain/auth/v2"

// newNonce returns a fresh 32-byte random nonce in hex.
func newNonce() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// authMessage is the exact string a dialer signs to prove it owns a node key.
// It binds the server's nonce (replay protection), the claimed node UID and the
// dialer's public key (so a signature cannot be lifted onto another key).
func authMessage(nonce, uid, pubkey string) string {
	return utils.EncodeFields(protocolAuthTag, nonce, uid, pubkey)
}

// lookupSeed returns the dns_seed.json entry for uid.
func lookupSeed(uid string) (config.Dns_seed_config, bool) {
	for _, seed := range config.DNS_SEED_LIST {
		if seed.UID == uid {
			return seed, true
		}
	}
	return config.Dns_seed_config{}, false
}

// authenticatePeer performs the server side of the handshake. It returns the
// authenticated node UID. The peer is authenticated by a signature over the
// server's fresh nonce, verified against the public key configured for that UID
// in dns_seed.json — an HTTP header alone can no longer impersonate a peer.
func authenticatePeer(safe *safeConn) (string, error) {
	nonce, err := newNonce()
	if err != nil {
		return "", err
	}

	challenge := WS_Challenge{
		Method:       MethodChallenge,
		Nonce:        nonce,
		ServerUID:    config.SELF_UID,
		ServerPubKey: config.NODE_PUBLIC_KEY,
	}
	if err := safe.writeJSON(challenge); err != nil {
		return "", fmt.Errorf("send challenge: %w", err)
	}

	var auth WS_Auth
	if err := safe.readJSON(&auth, config.WS_HANDSHAKE_TIMEOUT); err != nil {
		return "", fmt.Errorf("read auth: %w", err)
	}
	if auth.Method != MethodAuth {
		return "", errors.New("first message was not an auth response")
	}

	seed, known := lookupSeed(auth.UID)
	if !known || auth.UID == config.SELF_UID {
		return "", fmt.Errorf("unknown or self node UID %q", auth.UID)
	}
	if seed.PubKey == "" {
		return "", fmt.Errorf("node %q has no public key in dns_seed.json", auth.UID)
	}
	// The dialer must use exactly the key the operator pinned for this UID.
	if !strings.EqualFold(seed.PubKey, auth.PubKey) {
		return "", fmt.Errorf("node %q presented a public key that is not pinned for it", auth.UID)
	}

	ok, err := utils.VerifySignature(seed.PubKey, authMessage(nonce, auth.UID, auth.PubKey), auth.Signature)
	if err != nil || !ok {
		_ = safe.writeJSON(WS_Auth_Res{IsSuccess: false, Error: "authentication failed"})
		return "", fmt.Errorf("signature verification failed for node %q: %v", auth.UID, err)
	}

	if err := safe.writeJSON(WS_Auth_Res{IsSuccess: true}); err != nil {
		return "", fmt.Errorf("send auth result: %w", err)
	}
	return auth.UID, nil
}

// authenticateClient performs the dialer side of the handshake. When the server
// advertises a public key it must match the key pinned in dns_seed.json, which
// gives the dialer a (one-way) assurance about which node it is talking to.
func authenticateClient(safe *safeConn, seed config.Dns_seed_config) error {
	var challenge WS_Challenge
	if err := safe.readJSON(&challenge, config.WS_HANDSHAKE_TIMEOUT); err != nil {
		return fmt.Errorf("read challenge: %w", err)
	}
	if challenge.Method != MethodChallenge {
		return errors.New("server did not send a challenge")
	}

	if challenge.ServerPubKey != "" {
		if seed.PubKey == "" {
			return fmt.Errorf("server %q presented a key but dns_seed.json pins none", seed.UID)
		}
		if !strings.EqualFold(challenge.ServerPubKey, seed.PubKey) {
			return fmt.Errorf("server %q presented a key that does not match dns_seed.json", seed.UID)
		}
	}

	priv, err := config.LoadNodePrivateKey()
	if err != nil {
		return err
	}
	pub, err := utils.DerivePubKeyHex(priv)
	if err != nil {
		return err
	}
	if config.NODE_PUBLIC_KEY != "" && !strings.EqualFold(config.NODE_PUBLIC_KEY, pub) {
		return errors.New("NODE_PUBLIC_KEY does not match the public key derived from NODE_PRIVATE_KEY")
	}

	signature, err := utils.Sign(priv, authMessage(challenge.Nonce, config.SELF_UID, pub))
	if err != nil {
		return fmt.Errorf("sign challenge: %w", err)
	}

	auth := WS_Auth{Method: MethodAuth, UID: config.SELF_UID, PubKey: pub, Signature: signature}
	if err := safe.writeJSON(auth); err != nil {
		return fmt.Errorf("send auth: %w", err)
	}

	var res WS_Auth_Res
	if err := safe.readJSON(&res, config.WS_HANDSHAKE_TIMEOUT); err != nil {
		return fmt.Errorf("read auth result: %w", err)
	}
	if !res.IsSuccess {
		return fmt.Errorf("server rejected authentication: %s", res.Error)
	}
	return nil
}
