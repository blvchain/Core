package ws

import "blvchain/core/db"

// WebSocket message methods.
const (
	MethodGet       = "get"
	MethodAdd       = "add"
	MethodInventory = "inventory"
	MethodGetBlocks = "getblocks"

	// Handshake methods (NET-2). The server sends MethodChallenge as soon as the
	// connection is upgraded; the dialer answers with MethodAuth.
	MethodChallenge = "challenge"
	MethodAuth      = "auth"
)

// WS_Req is a request sent by a peer node.
//
// Which fields are populated depends on Method:
//   - "get":       Block is a query (one field or a full ID).
//   - "add":       Block is the replicated block.
//   - "inventory": Cursor is the last _id the requester saw ("" to start).
//   - "getblocks": IDs lists the block IDs to return.
type WS_Req struct {
	Method string   `json:"method"`
	Block  db.Block `json:"block,omitempty"`
	Cursor string   `json:"cursor,omitempty"`
	IDs    []string `json:"ids,omitempty"`
}

// WS_Res is the reply to "get" and "add".
type WS_Res struct {
	IsSuccess bool     `json:"isSuccess"`
	Block     db.Block `json:"block"`
}

// WS_Inventory_Res is the reply to "inventory": one page of block IDs in
// ascending _id order, plus the cursor to pass back for the next page.
type WS_Inventory_Res struct {
	IsSuccess  bool     `json:"isSuccess"`
	IDs        []string `json:"ids"`
	NextCursor string   `json:"nextCursor"`
	HasMore    bool     `json:"hasMore"`
}

// WS_Blocks_Res is the reply to "getblocks".
type WS_Blocks_Res struct {
	IsSuccess bool       `json:"isSuccess"`
	Blocks    []db.Block `json:"blocks"`
}

// WS_Challenge is the server's handshake nonce. ServerPubKey is optional; when
// set the dialer verifies it against the trust anchor in dns_seed.json.
type WS_Challenge struct {
	Method       string `json:"method"`
	Nonce        string `json:"nonce"`
	ServerUID    string `json:"serverUid,omitempty"`
	ServerPubKey string `json:"serverPubKey,omitempty"`
}

// WS_Auth is the dialer's answer to a challenge. Signature covers the nonce,
// the claimed UID and the dialer's public key (see authMessage).
type WS_Auth struct {
	Method    string `json:"method"`
	UID       string `json:"uid"`
	PubKey    string `json:"pubKey"`
	Signature string `json:"signature"`
}

// WS_Auth_Res is the server's verdict on a handshake.
type WS_Auth_Res struct {
	IsSuccess bool   `json:"isSuccess"`
	Error     string `json:"error,omitempty"`
}
