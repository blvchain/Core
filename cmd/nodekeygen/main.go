// Command nodekeygen generates a P-256 node identity for the WebSocket
// peer-authentication handshake (NET-2).
//
// Usage:
//
//	go run ./cmd/nodekeygen <node-uid>
//
// It prints the private key (keep it secret, put it in config/node_keys.json or
// the NODE_PRIVATE_KEY environment variable) and the public key (publish it in
// config/dns_seed.json so peers can authenticate this node).
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"blvchain/core/utils"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: nodekeygen <node-uid>")
		os.Exit(2)
	}
	uid := os.Args[1]

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate key:", err)
		os.Exit(1)
	}

	out := map[string]string{
		"uid":        uid,
		"privateKey": utils.PrivateKeyHex(priv),
		"publicKey":  utils.PubKeyHex(&priv.PublicKey),
	}
	encoded, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(encoded))
}
