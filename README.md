# BLVchain Core

An **educational blockchain node** written in Go. It stores signed, hash‑chained blocks in MongoDB, exposes a gRPC API, replicates blocks to peer nodes over an authenticated WebSocket protocol, and stores user‑supplied WebAssembly contracts.

> ⚠️ **Educational project — not a production blockchain.** There is no consensus algorithm, no account/balance model, and no TLS. Do not use it to secure anything of value.
>
> **Protocol v2:** this revision replaces Delium with domain‑separated SHA‑256 for all security‑critical hashing and changes the sync and peer‑authentication protocols. Existing v1 databases are **not** silently migrated — see [`docs/MIGRATION.md`](docs/MIGRATION.md).

- 🇬🇧 **English** documentation: [start here](#english)
- 🇮🇷 **مستندات فارسی**: [از اینجا شروع کنید](#فارسی)

---

## English

### Table of contents

1. [Features](#features)
2. [Architecture](#architecture)
3. [Project structure](#project-structure)
4. [Requirements](#requirements)
5. [Quick start](#quick-start)
6. [Configuration](#configuration)
7. [gRPC API](#grpc-api)
8. [WebSocket protocol](#websocket-protocol)
9. [Smart contracts (BVM)](#smart-contracts-bvm)
10. [Testing](#testing)
11. [What changed in this revision](#what-changed-in-this-revision)
12. [Known limitations](#known-limitations)
13. [Security notes](#security-notes)
14. [License](#license)

---

### Features

| Feature | Notes |
|---|---|
| **Hash‑chained blocks** | Every block ID is a domain‑separated SHA‑256 hash over its header, payload, contract metadata and the previous block hash. |
| **Signatures** | ECDSA over the P‑256 curve (canonical low‑S). The sender UID is derived from the public key, so a key cannot be swapped after the fact. Each block also carries the producing node's signature over its ID and authenticated metadata. |
| **Standard hashing** | `utils/hash.go` provides SHA‑256 with domain separation for block IDs, UIDs and message digests. Delium is kept only as an educational example and is **not** used by the protocol. |
| **gRPC API** | `AddData` and `ReadData`, protected by an API key and a per‑client token‑bucket rate limiter. |
| **Peer replication** | Nodes connect over WebSocket using an **inventory exchange** (`inventory` / `getblocks`), authenticated by a challenge/response handshake. |
| **Smart contracts** | WASM modules are **stored**; the [wazero](https://wazero.io/) runner (`bvm`) is a development‑only helper with a wall‑clock timeout and memory cap. |
| **Structured logging** | Rotating log files (lumberjack) split by outcome: gRPC, WebSocket, smart contract, internal, signature. |
| **Test suite** | 190+ tests covering every package, including an in‑memory MongoDB fake so no server is required. |

---

### Architecture

```
                       ┌──────────────────────────────────────────┐
                       │              BLVchain node               │
                       │                                          │
  gRPC clients ───────▶│  protos/ (AddData, ReadData)             │
  (API key + TLS* )    │      │             │                   │
                       │      ▼             ▼                   │
                       │  utils/ (ECDSA,  db/ (MongoDB)          │
                       │   SHA‑256)         │                   │
                       │      │             │                   │
                       │      ▼             │                   │
                       │  bvm/ (WASM,       │                   │
                       │   dev helper)      │                   │
                       │                    │                   │
  Peer nodes ◀────────▶│  ws/ (inventory /  │                   │
  (challenge/response) │   getblocks / add) │                   │
                       └────────────────────┼───────────────────┘
                                            │
                                     ┌──────▼──────┐
                                     │  MongoDB    │
                                     └─────────────┘
```

\* TLS is not built in; terminate it in a reverse proxy.

**Request path for `AddData`**

1. `validateAuth` reads the API key from the gRPC metadata.
2. `validateAddDataRequest` checks field lengths, hex encodings, sizes and timestamps.
3. `utils.Verify` recomputes the UID from the public key and verifies the `r‖s` signature over `db.MessageMaker`.
4. The sender's previous block is looked up to set `PreBlockHash`.
5. If a WASM payload is present it is decoded, checksum‑verified and written to `SMART_CONTRACT_DIR`.
6. The block is hashed, inserted into MongoDB and replicated to the peers.

---

### Project structure

```
.
├── main.go                 # Entry point: config → DB → genesis → WS → gRPC
├── build.sh                # Portable build script (optional garble/upx)
├── run_dev.sh              # Air hot reload, or `go run .`
├── make_proto.sh           # Regenerates protos/*.pb.go
├── s1.sh / s2.sh / s3.sh   # Run three local nodes
│
├── config/
│   ├── .env                # Environment variables
│   ├── .env.example        # Documented template
│   ├── api_key.json        # Allowed API keys (true = enabled)
│   ├── delium_config.json  # Hashing pipeline definition
│   ├── dns_seed.json       # Peer discovery list
│   ├── constants.go        # Genesis values, limits, defaults
│   ├── lib.go              # Loaders + Validate()
│   ├── structs.go          # Config file structs
│   └── vars.go             # Runtime configuration
│
├── db/
│   ├── model.go            # Block / BlockMeta / BlockData / Contract
│   ├── lib.go              # Hashing, signing, validation, genesis
│   ├── mongo.go            # Collection interface + CRUD
│   └── dbtest/
│       └── memory.go       # In-memory MongoDB fake used by tests
│
├── logger/
│   └── logger.go           # Rotating loggers + SetOutput()
│
├── protos/
│   ├── gate.proto          # Service definition
│   ├── gate.pb.go          # Generated messages
│   ├── gate_grpc.pb.go     # Generated gRPC stubs
│   ├── gate.service.go     # AddData / ReadData implementations
│   ├── lib.go              # Request + auth validation
│   ├── ratelimit.go        # Per-method, per-client token bucket
│   └── struct.go           # Service types
│
├── utils/
│   ├── delium.go           # D256 / D512 / D256C / D512C
│   ├── elliptic.go         # ECDSA P-256 sign / verify / key helpers
│   └── lib.go              # Formatting, hex, files, validators
│
├── ws/
│   ├── client.go           # Outbound client manager (see manager.go)
│   ├── manager.go          # Dial, reconnect, inventory sync, replication
│   ├── auth.go             # Challenge/response peer authentication
│   ├── server.go           # Inbound handler (get / add / inventory / getblocks)
│   ├── conn.go             # Goroutine-safe WebSocket wrapper
│   ├── struct.go           # Wire types
│   └── var.go              # Upgrader + managers
│
└── bvm/
    └── bvm.go              # wazero runtime, limits, host functions
```

---

### Requirements

- **Go 1.24 or newer** (`go.mod` declares `go 1.24.0`).
- **MongoDB 5.0+** reachable at `MONGO_URI`.
- Optional: `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` to regenerate the protobuf code.
- Optional: [`air`](https://github.com/air-verse/air) for hot reload, [`garble`](https://github.com/burrowers/garble) and [`upx`](https://upx.github.io/) for hardened builds.

---

### Quick start

#### 1. Start MongoDB

```bash
# Docker
docker run -d --name blvchain-mongo -p 27017:27017 mongo:7

# or use a local installation and make sure mongod is running
```

#### 2. Configure the node

```bash
cp config/.env.example config/.env
```

Edit `config/.env` and set at least:

```env
MONGO_URI=mongodb://localhost:27017/
SELF_UID=node_1_729ea2f23d456b626df2f27183047912
```

`SELF_UID` must be **32 hex characters**. For multi‑node setups it must also appear in `config/dns_seed.json`.

The API key checked by the server lives in `config/api_key.json`. The shipped file contains one enabled key:

```json
{
  "384f98b6-ff7e-42c7-97b3-95a8dad59963": true
}
```

#### 3. Run

```bash
# from the repository root — the config directory is resolved relative to it
go run .

# or build a binary
./build.sh
./core
```

On the first start the node:

1. validates the configuration,
2. connects to MongoDB and creates the indexes,
3. writes the genesis block if the collection is empty,
4. starts the WebSocket server and the initial sync,
5. starts the gRPC server.

The process prints its progress and stops on `Ctrl+C`.

#### 4. Three local nodes

```bash
./s1.sh   # SELF_UID=node_1_...  DB=BLVchain_1  WP=:8081  GP=:50051
./s2.sh   # SELF_UID=node_2_...  DB=BLVchain_2  WP=:8082  GP=:50052
./s3.sh   # SELF_UID=node_3_...  DB=BLVchain_3  WP=:8083  GP=:50053
```

Each script only sets the identity and ports; `MONGO_URI` still comes from `config/.env`. The three UIDs already match `config/dns_seed.json`, so the nodes discover each other automatically.

#### 5. Call the API

Install [`grpcurl`](https://github.com/fullstorydev/grpcurl) and list the services:

```bash
grpcurl -plaintext localhost:50051 list
# gate.AddData
# gate.ReadData
```

`ReadData` example (replace the API key with the one in `config/api_key.json`):

```bash
grpcurl -plaintext \
  -H 'authorization: 384f98b6-ff7e-42c7-97b3-95a8dad59963' \
  -d '{"Limit": 10, "Skip": 0, "SenderUID": "0123456789abcdef0123456789abcdef"}' \
  localhost:50051 gate.ReadData/readData
```

`AddData` needs a real signature, so it is easier to call from Go:

```go
package main

import (
	"context"
	"fmt"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/protos"
	"blvchain/core/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func main() {
	// Load config/delium_config.json so the same hashing pipeline is used.
	config.SetConfigDir("./config")
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()

	// --- client identity (normally generated once and stored safely) ---
	privHex := "4c0883a69102937d6231471b5dbb6204fe512909508000000000000000" // example
	pubHex := "02e493dbbf1d2b7c1f8f9b0f4f6b5c1e1a5d3f2c9b8a7d6e5f4a3b2c1d0e9f8a"
	uid := utils.Make_UID(pubHex)

	blockData := db.BlockData{
		SenderUID:    uid,
		SenderRole:   1,
		SenderPubKey: pubHex,
		ReceiverUID:  "0123456789abcdef0123456789abcdef",
		ReceiverRole: 1,
		Data:         "hello chain",
		TimeStamp:    utils.NowTimeInt64UnixMilli(),
	}

	sig, err := utils.Sign(privHex, db.MessageMaker(blockData))
	if err != nil {
		panic(err)
	}

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"authorization", "384f98b6-ff7e-42c7-97b3-95a8dad59963")

	res, err := protos.NewAddDataClient(conn).AddData(ctx, &protos.BlockData{
		SenderUID: uid, SenderRole: 1, SenderPubKey: pubHex, Signature: sig,
		ReceiverUID: blockData.ReceiverUID, ReceiverRole: 1,
		Data: blockData.Data, TimeStamp: blockData.TimeStamp,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("success=%v hash=%s log=%s\n", res.IsSuccess, res.BlockHash, res.Log)
}
```

> The example private key above is a placeholder. Generate a real key pair with `utils.PrivateKeyHex` / `utils.PubKeyHex` (see `utils/elliptic_test.go`) and never commit it.

---

### Configuration

All settings come from `config/.env` (or the real environment, which takes precedence).

| Variable | Default | Description |
|---|---|---|
| `MONGO_URI` | — **(required)** | MongoDB connection string. |
| `DB` | `BLVchain` | Database name. |
| `SELF_UID` | — **(required)** | This node's identifier (for example `node_1_729ea2…`). It is an operator-assigned label, not a key-derived value. |
| `NODE_PRIVATE_KEY` | — | This node's P‑256 private key (hex). If unset, the node reads `config/node_keys.json` (gitignored) keyed by `SELF_UID`. Required to produce blocks and to dial peers. |
| `NODE_PUBLIC_KEY` | — | Optional. If set, it must match the key derived from `NODE_PRIVATE_KEY`, and is advertised to peers. |
| `TIMESTAMP_SKEW_MS` | `86400000` | Maximum tolerated difference between this node's clock and a client-supplied block timestamp (24 h). |
| `WP` | `:8080` | WebSocket listen address. |
| `GP` | `:50051` | gRPC listen address. |
| `READ_DATA_R` | `0.1` | `ReadData` rate limit, tokens per second. |
| `READ_DATA_BURST` | `30` | `ReadData` burst size. |
| `ADD_DATA_R` | `0.1` | `AddData` rate limit, tokens per second. |
| `ADD_DATA_BURST` | `5` | `AddData` burst size. |
| `MAX_DATA_SIZE_KB` | `2048` | Maximum accepted block payload, in KB. |
| `SMART_CONTRACT_DIR` | `smart_contracts` | Directory where uploaded WASM files are stored. |
| `SMART_CONTRACT_PATH` | — | Path to the WASM file used by the development helper `bvm.RunBVM`. |
| `EXECUTION_TIMEOUT_SECONDS` | `10` | Wall‑clock limit for one contract execution. |
| `MAX_MEMORY_PAGES` | `256` | WASM memory cap in 64 KiB pages (256 = 16 MiB). |
| `MAX_WASM_SIZE_KB` | `1024` | Maximum accepted contract upload, in KB. |
| `DB_OP_TIMEOUT_SECONDS` | `10` | Timeout for a single MongoDB operation. |
| `BLVCHAIN_CONFIG_DIR` | — | Overrides the directory that holds `.env` and the JSON files. |

The node refuses to start if a required value is missing or malformed; the exact list of problems is printed before exit.

**`config/api_key.json`** (gitignored — copy from `config/api_key.example.json`)

```json
{
  "replace-with-a-strong-random-key": true
}
```

A key mapped to `false` is present but always rejected. At least one enabled key is required. **Never commit a real key.** If a key was ever committed, rotate it.

**`config/dns_seed.json`**

```json
[
  {
    "UID": "node_1_729ea2f23d456b626df2f27183047912",
    "Address": "ws://localhost:8081",
    "PubKey": "0328a433dd71861b466366be03e21597b2d5ae9b4f096555124408ce88369e9cf1"
  }
]
```

A node dials every entry whose `UID` differs from its own `SELF_UID`. `PubKey` is the node's compressed P‑256 public key and is the **trust anchor** for peer authentication and block node-signatures. A peer must prove possession of the matching private key during the WebSocket handshake; the `X‑UID` header alone is not authentication. Generate keys with `go run ./cmd/nodekeygen <uid>`.

**`config/delium_config.json`** — educational only

```json
{
  "BLOCK_HASHING_PATH": "BLV#5/chain#6/StRoNg#7/SAFE#8",
  "UID_MAKER_PATH": "SFFds#7/dfgki#6/d5gj7#7/dfgk78#10",
  "MESSAGE_HASH_PATH": "sdwE#3/kiu9u#6/OEJLF#7/dDFO6TGF#8/JLKoty#5"
}
```

Delium is **not** used by the protocol. It is retained as a teaching example of a hash pipeline; its `deleteChars` step lowers collision resistance below SHA‑256, so it must not be used for anything security-critical. The protocol uses domain-separated SHA‑256 (`utils/hash.go`).

---

### gRPC API

Two services, both requiring an API key in the metadata under `authorization`, `x-api-key` or `auth`.

#### `gate.AddData/addData`

Request (`BlockData`):

| Field | Type | Rules |
|---|---|---|
| `SenderUID` | string | 32 hex characters. Must equal `Make_UID(SenderPubKey)`. |
| `SenderRole` | int64 | `1 … 10 000 000`. |
| `SenderPubKey` | string | 66 hex characters (compressed P‑256). |
| `Signature` | string | 128 hex characters (`r‖s`, each 32 bytes). |
| `ReceiverUID` | string | 32 hex characters. |
| `ReceiverRole` | int64 | `1 … 10 000 000`. |
| `Data` | string | Required, smaller than `MAX_DATA_SIZE_KB`. |
| `UseContract` | string | Optional, 66 hex characters. Required when `ContractBase64` is set. |
| `ContractBase64` | bytes | Optional base64‑encoded WASM. |
| `ContractData` | Contract | Optional metadata (name, version, checksum, …). |
| `TimeStamp` | int64 | Unix milliseconds, inside the accepted window. |

Response (`AddDataResult`): `IsSuccess`, `Log`, `BlockHash`.

The client signature covers `db.MessageMaker(blockData)`, a length‑prefixed encoding of the record fields including `UseContract` and the contract metadata. When `ContractBase64` is present, the server decodes it, checks `ContractData.Checksum` against the SHA‑256 of the bytes, and stores the file as `<checksum>.wasm` inside `SMART_CONTRACT_DIR`.

After verifying the client signature, the node links the block to the sender's previous block and **signs the block itself** with its node key (`BlockMeta.NodeSignature`). The node signature covers the block ID and the authenticated metadata (`PreBlockHash`, `NodeUID`, `BlockMeta.TimeStamp`), so peers can verify which node produced the block and that the predecessor was not altered.

Retrying the exact same request is idempotent: the server looks the signature up and returns the existing block hash instead of creating a second block.

#### `gate.ReadData/readData`

Request (`ReadDataRequest`): `Limit` (1–100) and `Skip` are required, plus **at least one filter**:

| Filter | Type | Matches |
|---|---|---|
| `UID` | string | `senderUid == UID` **or** `receiverUid == UID`. |
| `SenderUID` | string | `blockData.senderUid`. |
| `SenderRole` | int64 | `blockData.senderRole`. |
| `SenderPubKey` | string | `blockData.senderPubKey`. |
| `ReceiverUID` | string | `blockData.receiverUid`. |
| `ReceiverRole` | int64 | `blockData.receiverRole`. |
| `BlockHash` | string | `_id`. |
| `PreBlockHash` | string | `blockMeta.preBlockHash`. |
| `NodeUID` | string | `blockMeta.nodeUid`. |
| `TimeStampFrom` / `TimeStampTo` | int64 | `blockData.timeStamp` range, **in milliseconds**. |
| `UseContract` | string | `blockData.useContract`. |

Blocks that fail full validation (client signature, node signature or hash) are silently skipped; `Boycott` blocks are never returned.

---

### WebSocket protocol

Peers connect to `ws://host:WP` with an `X-UID` header (a cheap pre-upgrade filter) and then complete a **challenge/response handshake** before any other message is accepted. Messages are JSON.

**Handshake**

1. Server → dialer: `{ "method": "challenge", "nonce": "…", "serverUid": "…", "serverPubKey": "…" }`.
2. Dialer → server: `{ "method": "auth", "uid": "…", "pubKey": "…", "signature": "…" }`, where the signature is over the nonce, UID and public key.
3. Server verifies the signature against the public key pinned for that UID in `dns_seed.json`. An unauthenticated connection cannot issue any other request.

**Requests**

```json
{ "method": "add", "block": { ... } }
{ "method": "inventory", "cursor": "" }
{ "method": "getblocks", "ids": ["…", "…"] }
```

**Methods**

| Method | Meaning | Reply |
|---|---|---|
| `get` | Look up one block by any combination of fields. | `WS_Res` |
| `add` | Replicate a block. The receiver validates structure, both signatures and the hash, stores it, and treats a duplicate as success. Invalid blocks are dropped, not quarantined. | `WS_Res` |
| `inventory` | Return one page of block IDs in ascending `_id` order, plus `nextCursor`/`hasMore`. | `WS_Inventory_Res` |
| `getblocks` | Return the blocks for the supplied IDs (bounded). | `WS_Blocks_Res` |

**Replies**

```json
{ "isSuccess": true, "block": { ... } }
{ "isSuccess": true, "blocks": [ { ... }, { ... } ] }
{ "isSuccess": true, "ids": ["…"], "nextCursor": "…", "hasMore": true }
```

**Why inventory sync is correct:** it never relies on timestamps. The requester pages the peer's complete, immutable `_id`-ordered inventory, asks explicitly for the IDs it lacks, validates them, and stores them in dependency order. Already-known IDs are ignored, so replication is idempotent and re-running a sync is a no-op. A peer that was offline, restarted, or received blocks out of order converges on the next round. The old `sync` message (timestamp-ordered) has been removed because it could permanently strand a block older than the requester's tip.

The node runs three background loops:

- `MonitorAndReconnectToServers` — pings peers every 10 s and re‑dials the dead ones.
- `FirstTimeSyncData` — blocks start‑up until every peer is up to date.
- `SyncData` — reconciles with peers every 5 s.

---

### Smart contracts (BVM)

Contracts are WebAssembly modules that export a function named `smart_contract`:

```wat
(module
  (func (export "smart_contract"))
)
```

They run on [wazero](https://wazero.io/) with three limits:

1. **CPU** — `EXECUTION_TIMEOUT_SECONDS` (default 10 s). The context deadline aborts the module, so infinite loops are killed.
2. **Memory** — `MAX_MEMORY_PAGES` (default 256 pages = 16 MiB). A module that asks for more fails to instantiate.
3. **API** — only functions registered with `bvm.AddHostFunction` are visible. The development helper `print(ptr, len)` is registered by `bvm.InitBVMInternalFunctions`.

```go
if err := bvm.RunBVM("smart_contracts/example.wasm"); err != nil {
	log.Fatal(err)
}
```

Uploaded contracts are stored by their SHA‑256 checksum, never by a client‑supplied filename, so a peer cannot write outside `SMART_CONTRACT_DIR`.

---

### Testing

The suite is hermetic: it uses an in‑memory MongoDB fake (`db/dbtest`) and a hand‑assembled WASM module, so **no MongoDB server, network or compiler is required**.

```bash
# everything
go test ./...

# verbose
go test -v ./...

# with coverage
go test -cover ./...

# a single package
go test ./utils/ -run TestSignVerifyRoundTrip -v
```

On machines with little free memory the Go toolchain may fail to link all test binaries at once (`fatal error: runtime: cannot allocate memory`). Run them one at a time:

```bash
go test -p 1 ./...
```

**Coverage by package**

| Package | What is covered |
|---|---|
| `utils` | Domain-separated SHA‑256 (domains, unambiguous encoding), educational Delium (determinism, malformed paths, entropy loss), ECDSA sign/verify (tampering, wrong key, bad encodings), hex/file helpers, validators. |
| `config` | Every loader, `.env` precedence, `Validate()`, hashing‑path syntax, genesis constant lengths. |
| `db` | Block hashing (field coverage), signature validation, genesis creation/idempotency, CRUD, pagination, empty‑result semantics. |
| `db/dbtest` | The in‑memory fake: operators, dotted paths, sort, skip/limit, concurrency. |
| `protos` | Request validation (every field), auth, rate limiting (burst, isolation, cleanup, interceptors), `AddData`/`ReadData` end‑to‑end, contract uploads. |
| `ws` | Origin checks, challenge/response authentication, `get`/`add`/`inventory`/`getblocks` handlers, rate limiting, dependency-aware sync, replication, reconnects. |
| `bvm` | Minimal module execution, missing file, bad WASM, missing export, timeout, memory limit, host functions. |
| `logger` | Initialisation and output redirection. |
| `main` | Fail‑fast on invalid configuration. |

---

### What changed in this revision

This revision is a correctness and safety pass. The following changes are **protocol‑visible** and require a fresh database:

1. **Fixed‑width signatures.** `r` and `s` are now zero‑padded to 32 bytes each. The old code concatenated the minimal big‑endian encodings, which produced a variable‑length signature that could not be split reliably.
2. **The block hash and the signed message now cover `UseContract` and the contract metadata.** Two blocks that differ only in contract data previously shared an ID.
3. **`ReadData` timestamp filters are milliseconds.** They were validated in seconds but compared against millisecond values, so the range never matched.
4. **Empty query results are no longer errors.** `FindMany*` returns an empty slice with `nil`; callers no longer special‑case `mongo.ErrNoDocuments`.
5. **Contract uploads verify the checksum against the uploaded bytes.** The old code checked a file that had to exist on disk first, so the first upload always failed.
6. **Genesis constants have the correct lengths** (the block pre‑hash was 62 characters and the signature 142, so the first block could never pass validation).
7. **WebSocket writes are serialised.** gorilla/websocket allows a single writer; replication, sync and the reconnect monitor used to race on the same connection.
8. **Sync excludes the tip block and preserves contract metadata.** The old loop re‑downloaded the block it already had and dropped `UseContract`/`ContractData`, which made synced blocks fail validation.
9. **The BVM now enforces the limits the documentation promised.** The runtime had no timeout and no memory cap.
10. **Configuration is validated at start‑up**, missing `.env` files are no longer fatal, and `log.Fatal` inside the hashing library was removed.

Other fixes: `AddData` no longer prints debug output, reports duplicate submissions idempotently, and logs success to the success logger; `ConnectToOneServer` no longer stores a nil connection after a failed dial; the rate limiter uses the same metadata keys as authentication.

**Phase 2 security-audit revision.** A full adversarial audit is recorded in
[`AUDIT_REPORT.md`](AUDIT_REPORT.md), with a prioritised checklist in
[`REMEDIATION_PLAN.md`](REMEDIATION_PLAN.md), the threat model in
[`SECURITY.md`](SECURITY.md) and the upgrade steps in
[`docs/MIGRATION.md`](docs/MIGRATION.md). This phase is **protocol-visible and
requires a fresh database**:

11. **ECDSA signatures are canonical low-S (anti-malleability).** `Sign` emits
    only the lower-half form of `s`; `decodeSignature` rejects high-S and
    out-of-range scalars. Previously `(r, s)` and `(r, N−s)` both verified, so
    one payload could produce two block IDs.
12. **Sender/receiver role bounds are consistent** across gRPC and replication
    (`[1, 10 000 000]`).
13. **Delium is no longer used for anything security-critical.** Block IDs,
    UIDs, message digests and node signatures use domain-separated SHA‑256
    (`utils/hash.go`). Delium remains only as an educational module; its
    `deleteChars` step lowers collision resistance to ≈2^86 (message) / 2^104
    (block), below SHA‑256.
14. **Signatures cover the raw 32-byte digest**, not 64 ASCII hex characters
    (previously only the first 128 bits reached ECDSA).
15. **Serialisation is unambiguous.** The signed message, contract root and
    block-hash root are length-prefixed, so different field tuples cannot
    encode identically.
16. **Sync is inventory-based, not timestamp-based.** A node that is offline,
    restarted, or receives blocks out of order converges; no record is stranded
    because it is older than the local tip. Replication is idempotent.
17. **Peers are authenticated by a challenge/response signature** against the
    key pinned in `dns_seed.json`. The `X-UID` header is no longer an
    authentication mechanism.
18. **Blocks carry a node signature** over the block ID and authenticated
    metadata; peers verify it. The predecessor link is resolved before a
    replicated block is stored.
19. **WebSocket denial-of-service protections**: per-peer rate limiting,
    inbound connection cap, bounded sync pages, bounded messages, and invalid
    blocks are dropped instead of quarantined.
20. **Secrets are out of source control** (`.env`, `api_key.json`,
    `node_keys.json` are gitignored). **The previously committed API key must
    still be rotated.**

> **Delium is not a stronger hash.** It is a configurable multi-round SHA-256/512
> pipeline kept for teaching. Its `deleteChars` step *lowers* collision
> resistance below SHA-256 (to roughly 2^86 for the message path, 2^104 for the
> block path). It is no longer referenced by the protocol — see
> `AUDIT_REPORT.md` finding DEL-1.

---

### Known limitations

- **No consensus.** Blocks are signed and chained, but nodes do not vote or agree on a canonical chain. A malicious majority can rewrite history.
- **No accounts or balances.** The chain stores arbitrary data; `SenderRole`/`ReceiverRole` are free integers.
- **No TLS.** Run the node behind a TLS‑terminating proxy.
- **Synchronous replication.** `AddData` waits for the peers before answering; unreachable peers slow the request down (each write is bounded by `WS_WRITE_TIMEOUT`).
- **No automatic data migration.** The v2 hash profile is incompatible with v1 databases; see `docs/MIGRATION.md` for the backup/reset procedure.
- **Node keys are an operator responsibility.** A node needs a P‑256 key (`config/node_keys.json` or `NODE_PRIVATE_KEY`) to produce blocks and to dial peers.
- **Single process, single collection.** There is no sharding, no compaction and no pruning.
- **The API key is a shared secret** stored in plain text.

---

### Security notes

- Put the node behind a reverse proxy that terminates TLS and applies network‑level restrictions.
- Keep `config/api_key.json`, `config/.env` and `config/node_keys.json` out of version control in real deployments. **Rotate any key that was ever committed.**
- The WASM sandbox is a resource limit, not a full isolation boundary; treat uploaded contracts as untrusted input.
- Inbound WebSocket peers are authenticated by a challenge/response signature against the key pinned in `dns_seed.json`; a spoofed `X-UID` header alone is rejected. Still bind the WebSocket port to a private network.
- Rate limiting is per API key and per method, with a 5‑minute idle timeout; WebSocket peers are also rate-limited per connection.
- **Delium is not a security primitive.** It is kept only as an educational example and is not used by the protocol.
  There is **no consensus** and **no TLS** — see `SECURITY.md` for the full model.

---

### License

This project is part of the **BLVchain** ecosystem. See the repository for license information.

---

## فارسی

### فهرست مطالب

1. [ویژگی‌ها](#ویژگی‌ها)
2. [معماری](#معماری)
3. [ساختار پروژه](#ساختار-پروژه)
4. [پیش‌نیازها](#پیش‌نیازها)
5. [راه‌اندازی سریع](#راه‌اندازی-سریع)
6. [پیکربندی](#پیکربندی)
7. [رابط gRPC](#رابط-grpc)
8. [پروتکل WebSocket](#پروتکل-websocket)
9. [قراردادهای هوشمند (BVM)](#قراردادهای-هوشمند-bvm)
10. [تست‌ها](#تست‌ها)
11. [تغییرات این نسخه](#تغییرات-این-نسخه)
12. [محدودیت‌های شناخته‌شده](#محدودیت‌های-شناخته‌شده)
13. [نکات امنیتی](#نکات-امنیتی)
14. [مجوز](#مجوز)

---

### ویژگی‌ها

| ویژگی | توضیح |
|---|---|
| **بلاک‌های زنجیره‌ای با هش** | شناسهٔ هر بلاک، هش SHA‑256 با جداسازی دامنه از هدر، داده، فرادادهٔ قرارداد و هش بلاک قبلی است. |
| **امضا** | ECDSA روی منحنی P‑256 (low‑S). شناسهٔ فرستنده از کلید عمومی ساخته می‌شود. هر بلاک همچنین امضای نود تولیدکننده را دارد. |
| **هش استاندارد** | `utils/hash.go` برای شناسهٔ بلاک، UID و پیام از SHA‑256 با جداسازی دامنه استفاده می‌کند. Delium فقط آموزشی است و در پروتکل استفاده نمی‌شود. |
| **رابط gRPC** | دو متد `AddData` و `ReadData` با احراز هویت کلید API و محدودکنندهٔ نرخ به ازای هر کلاینت. |
| **همگام‌سازی نظیرها** | نودها با WebSocket و تبادل موجودی (`inventory`/`getblocks`) و احراز هویت چالش/پاسخ همگام می‌شوند. |
| **قرارداد هوشمند** | ماژول‌های WASM **ذخیره** می‌شوند؛ اجرای `bvm` فقط یک helper توسعه است. |
| **گزارش‌گیری ساخت‌یافته** | فایل‌های چرخشی لاگ بر اساس نتیجه: gRPC، WebSocket، قرارداد، داخلی و امضا. |
| **مجموعهٔ تست** | بیش از ۱۹۰ تست برای همهٔ بسته‌ها، همراه با یک شبیه‌ساز MongoDB در حافظه. |

> ⚠️ این پروژه **آموزشی** است و یک بلاک‌چین تولیدی نیست: الگوریتم اجماع، مدل حساب/موجودی، TLS و ابزار مهاجرت داده ندارد.

---

### معماری

```
                       ┌──────────────────────────────────────────┐
                       │              نود BLVchain                │
                       │                                          │
  کلاینت‌های gRPC ────▶│  protos/ (AddData، ReadData)             │
                       │      │             │                   │
                       │      ▼             ▼                   │
                       │  utils/ (ECDSA،   db/ (MongoDB)          │
                       │   SHA‑256)         │                   │
                       │      │             │                   │
                       │      ▼             │                   │
                       │  bvm/ (WASM،       │                   │
                       │   dev helper)      │                   │
                       │                    │                   │
  نودهای نظیر ◀───────▶│ ws/ (inventory /   │                   │
  (چالش/پاسخ)          │  getblocks / add)  │                   │
                       └────────────────────┼───────────────────┘
                                            │
                                     ┌──────▼──────┐
                                     │  MongoDB    │
                                     └─────────────┘
```

**مسیر یک درخواست `AddData`**

1. `validateAuth` کلید API را از متادیتای gRPC می‌خواند.
2. `validateAddDataRequest` طول فیلدها، کدگذاری hex، اندازه و زمان را بررسی می‌کند.
3. `utils.Verify` شناسه را از کلید عمومی بازسازی و امضای `r‖s` را روی `db.MessageMaker` بررسی می‌کند.
4. بلاک قبلی فرستنده پیدا می‌شود تا `PreBlockHash` تنظیم شود.
5. اگر فایل WASM ارسال شده باشد، رمزگشایی، بازبینی چک‌سام و در `SMART_CONTRACT_DIR` ذخیره می‌شود.
6. بلاک هش می‌شود، در MongoDB درج و برای نظیرها ارسال می‌شود.

---

### ساختار پروژه

```
.
├── main.go                 # نقطه شروع: تنظیمات → DB → جنسیس → WS → gRPC
├── build.sh                # ساخت قابل حمل (با آپشن garble/upx)
├── run_dev.sh              # بارگذاری گرم با Air یا `go run .`
├── make_proto.sh           # بازتولید فایل‌های protobuf
├── s1.sh / s2.sh / s3.sh   # اجرای سه نود محلی
│
├── config/
│   ├── .env                # متغیرهای محیطی
│   ├── .env.example        # نمونهٔ مستندشده
│   ├── api_key.json        # کلیدهای API مجاز
│   ├── delium_config.json  # تعریف خط لولهٔ هش
│   ├── dns_seed.json       # فهرست کشف نظیرها
│   ├── constants.go        # مقادیر جنسیس، حدود و پیش‌فرض‌ها
│   ├── lib.go              # بارگذارها و Validate()
│   ├── structs.go          # ساختارهای فایل‌های تنظیمات
│   └── vars.go             # تنظیمات زمان اجرا
│
├── db/
│   ├── model.go            # Block / BlockMeta / BlockData / Contract
│   ├── lib.go              # هش، امضا، اعتبارسنجی، جنسیس
│   ├── mongo.go            # واسط Collection و CRUD
│   └── dbtest/
│       └── memory.go       # شبیه‌ساز MongoDB در حافظه برای تست
│
├── logger/
│   └── logger.go           # لاگ‌های چرخشی و SetOutput()
│
├── protos/
│   ├── gate.proto          # تعریف سرویس
│   ├── gate.pb.go          # پیام‌های تولیدشده
│   ├── gate_grpc.pb.go     # استاب‌های gRPC
│   ├── gate.service.go     # پیاده‌سازی AddData / ReadData
│   ├── lib.go              # اعتبارسنجی درخواست و احراز هویت
│   ├── ratelimit.go        # سهمیهٔ توکن به ازای متد و کلاینت
│   └── struct.go           # انواع سرویس
│
├── utils/
│   ├── delium.go           # D256 / D512 / D256C / D512C
│   ├── elliptic.go         # امضا و بررسی ECDSA P‑256
│   └── lib.go              # قالب‌بندی، hex، فایل و اعتبارسنج‌ها
│
├── ws/
│   ├── client.go           # اتصال خروجی، همگام‌سازی و تکثیر
│   ├── server.go           # هندلر ورودی (get / add / sync)
│   ├── conn.go             # پوشش امن WebSocket
│   ├── struct.go           # انواع پیام
│   └── var.go              # Upgrader و مدیرها
│
└── bvm/
    └── bvm.go              # زمان‌اجرا wazero، محدودیت‌ها و توابع میزبان
```

---

### پیش‌نیازها

- **Go نسخهٔ ۱.۲۴ یا جدیدتر** (`go.mod` مقدار `go 1.24.0` را اعلام کرده است).
- **MongoDB نسخهٔ ۵.۰ به بالا** در دسترس در `MONGO_URI`.
- اختیاری: `protoc` به همراه `protoc-gen-go` و `protoc-gen-go-grpc` برای بازتولید کد protobuf.
- اختیاری: [`air`](https://github.com/air-verse/air) برای بارگذاری گرم، و [`garble`](https://github.com/burrowers/garble) و [`upx`](https://upx.github.io/) برای ساخت محافظت‌شده.

---

### راه‌اندازی سریع

#### ۱. MongoDB را اجرا کنید

```bash
docker run -d --name blvchain-mongo -p 27017:27017 mongo:7
```

#### ۲. تنظیمات نود

```bash
cp config/.env.example config/.env
```

حداقل این مقادیر را در `config/.env` تنظیم کنید:

```env
MONGO_URI=mongodb://localhost:27017/
SELF_UID=node_1_729ea2f23d456b626df2f27183047912
```

`SELF_UID` باید **۳۲ کاراکتر hex** باشد و در حالت چندنودی باید در `config/dns_seed.json` هم وجود داشته باشد.

کلید API مجاز در `config/api_key.json` قرار دارد. نسخهٔ ارسالی یک کلید فعال دارد:

```json
{
  "384f98b6-ff7e-42c7-97b3-95a8dad59963": true
}
```

#### ۳. اجرا

```bash
# از ریشهٔ مخزن
go run .

# یا ساخت باینری
./build.sh
./core
```

در اولین اجرا نود:

1. تنظیمات را اعتبارسنجی می‌کند،
2. به MongoDB وصل می‌شود و ایندکس‌ها را می‌سازد،
3. اگر کلکسیون خالی باشد بلاک جنسیس را می‌نویسد،
4. سرور WebSocket و همگام‌سازی اولیه را راه‌اندازی می‌کند،
5. سرور gRPC را اجرا می‌کند.

با `Ctrl+C` فرآیند متوقف می‌شود.

#### ۴. سه نود محلی

```bash
./s1.sh   # SELF_UID=node_1_...  DB=BLVchain_1  WP=:8081  GP=:50051
./s2.sh   # SELF_UID=node_2_...  DB=BLVchain_2  WP=:8082  GP=:50052
./s3.sh   # SELF_UID=node_3_...  DB=BLVchain_3  WP=:8083  GP=:50053
```

هر اسکریپت فقط هویت و پورت‌ها را تنظیم می‌کند؛ `MONGO_URI` همچنان از `config/.env` خوانده می‌شود. سه UID با `config/dns_seed.json` مطابقت دارند، بنابراین نودها خودکار یکدیگر را پیدا می‌کنند.

#### ۵. فراخوانی API

با [`grpcurl`](https://github.com/fullstorydev/grpcurl):

```bash
grpcurl -plaintext localhost:50051 list
```

نمونهٔ `ReadData`:

```bash
grpcurl -plaintext \
  -H 'authorization: 384f98b6-ff7e-42c7-97b3-95a8dad59963' \
  -d '{"Limit": 10, "Skip": 0, "SenderUID": "0123456789abcdef0123456789abcdef"}' \
  localhost:50051 gate.ReadData/readData
```

برای `AddData` به امضای واقعی نیاز است، پس ساده‌تر است از Go استفاده کنید:

```go
package main

import (
	"context"
	"fmt"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/protos"
	"blvchain/core/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	config.SetConfigDir("./config")
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()

	privHex := "..." // کلید خصوصی ۳۲ بایتی به hex
	pubHex := "..."  // کلید عمومی فشرده ۳۳ بایتی به hex
	uid := utils.Make_UID(pubHex)

	blockData := db.BlockData{
		SenderUID:    uid,
		SenderRole:   1,
		SenderPubKey: pubHex,
		ReceiverUID:  "0123456789abcdef0123456789abcdef",
		ReceiverRole: 1,
		Data:         "hello chain",
		TimeStamp:    utils.NowTimeInt64UnixMilli(),
	}

	sig, err := utils.Sign(privHex, db.MessageMaker(blockData))
	if err != nil {
		panic(err)
	}

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"authorization", "384f98b6-ff7e-42c7-97b3-95a8dad59963")

	res, err := protos.NewAddDataClient(conn).AddData(ctx, &protos.BlockData{
		SenderUID: uid, SenderRole: 1, SenderPubKey: pubHex, Signature: sig,
		ReceiverUID: blockData.ReceiverUID, ReceiverRole: 1,
		Data: blockData.Data, TimeStamp: blockData.TimeStamp,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("success=%v hash=%s log=%s\n", res.IsSuccess, res.BlockHash, res.Log)
}
```

> کلید خصوصی نمونه را واقعی بسازید و هرگز آن را در مخزن قرار ندهید.

---

### پیکربندی

همهٔ تنظیمات از `config/.env` خوانده می‌شوند (متغیرهای واقعی محیط ارجحیت دارند).

| متغیر | پیش‌فرض | توضیح |
|---|---|---|
| `MONGO_URI` | — **الزامی** | رشتهٔ اتصال MongoDB. |
| `DB` | `BLVchain` | نام پایگاه داده. |
| `SELF_UID` | — **الزامی** | شناسهٔ این نود (یک برچسب). |
| `NODE_PRIVATE_KEY` | — | کلید خصوصی P‑256 این نود (hex). در صورت نبودن مقدار، از `config/node_keys.json` خوانده می‌شود. برای ساخت بلاک و اتصال به نظیرها لازم است. |
| `NODE_PUBLIC_KEY` | — | اختیاری؛ باید با کلید مشتق‌شده از `NODE_PRIVATE_KEY` یکسان باشد. |
| `TIMESTAMP_SKEW_MS` | `86400000` | بیشترین اختلاف مجاز ساعت نود و زمان‌سندای کلاینت (۲۴ ساعت). |
| `WP` | `:8080` | آدرس WebSocket. |
| `GP` | `:50051` | آدرس gRPC. |
| `READ_DATA_R` | `0.1` | نرخ مجاز `ReadData`. |
| `READ_DATA_BURST` | `30` | حجم جهش `ReadData`. |
| `ADD_DATA_R` | `0.1` | نرخ مجاز `AddData`. |
| `ADD_DATA_BURST` | `5` | حجم جهش `AddData`. |
| `MAX_DATA_SIZE_KB` | `2048` | بیشترین حجم مجاز دادهٔ بلاک. |
| `SMART_CONTRACT_DIR` | `smart_contracts` | پوشهٔ ذخیرهٔ فایل‌های WASM. |
| `SMART_CONTRACT_PATH` | — | مسیر فایل WASM برای اجرای توسعه. |
| `EXECUTION_TIMEOUT_SECONDS` | `10` | کرانل زمانی اجرای قرارداد. |
| `MAX_MEMORY_PAGES` | `256` | سقف حافظهٔ WASM (۲۵۶ صفحه = ۱۶ مگابایت). |
| `MAX_WASM_SIZE_KB` | `1024` | بیشترین حجم آپلود قرارداد. |
| `DB_OP_TIMEOUT_SECONDS` | `10` | کرانل هر عملیات MongoDB. |
| `BLVCHAIN_CONFIG_DIR` | — | تغییر پوشهٔ فایل‌های تنظیمات. |

اگر مقدار الزامی ناموجود یا نامعتبر باشد، نود اجرا نمی‌شود و فهرست خطاها چاپ می‌شود.

**`config/api_key.json`** (در `.gitignore` — از `config/api_key.example.json` کپی کنید) — کلیدی با مقدار `false` وجود دارد اما همیشه رد می‌شود. حداقل یک کلید فعال لازم است. **هر کلیدی که قبلاً commit شده باید بچرخد.**

**`config/dns_seed.json`** — هر ورودی که `UID` آن با `SELF_UID` خود نود متفاوت باشد، شماره‌گیری می‌شود. `PubKey` کلید عمومی P‑256 نود است و لنگر اعتماد احراز هویت نظیر و امضای نود است. هدر `X-UID` به‌تنهایی احراز هویت نیست.

**`config/delium_config.json`** — فقط آموزشی. Delium در پروتکل استفاده نمی‌شود؛ به‌دلیل مرحلهٔ `deleteChars` مقاومت برخورد آن کمتر از SHA‑256 است. پروتکل از SHA‑256 با جداسازی دامنه استفاده می‌کند.

---

### رابط gRPC

دو سرویس، هر دو با کلید API در متادیتا (`authorization`، `x-api-key` یا `auth`).

#### `gate.AddData/addData`

فیلدهای `BlockData`:

| فیلد | نوع | قاعده |
|---|---|---|
| `SenderUID` | string | ۳۲ کاراکتر hex و برابر با `Make_UID(SenderPubKey)`. |
| `SenderRole` | int64 | بین ۱ تا ۱۰٬۰۰۰٬۰۰۰. |
| `SenderPubKey` | string | ۶۶ کاراکتر hex (P‑256 فشرده). |
| `Signature` | string | ۱۲۸ کاراکتر hex (`r‖s`، هر کدام ۳۲ بایت). |
| `ReceiverUID` | string | ۳۲ کاراکتر hex. |
| `ReceiverRole` | int64 | بین ۱ تا ۱۰٬۰۰۰٬۰۰۰. |
| `Data` | string | الزامی و کوچک‌تر از `MAX_DATA_SIZE_KB`. |
| `UseContract` | string | اختیاری، ۶۶ کاراکتر hex. همراه با `ContractBase64` الزامی است. |
| `ContractBase64` | bytes | WASM در قالب base64 (اختیاری). |
| `ContractData` | Contract | فرادادهٔ قرارداد (اختیاری). |
| `TimeStamp` | int64 | زمان Unix بر حسب میلی‌ثانیه. |

پاسخ: `IsSuccess`، `Log`، `BlockHash`.

امضا روی `db.MessageMaker` گرفته می‌شود که `UseContract` و فرادادهٔ قرارداد را هم شامل می‌شود. اگر `ContractBase64` وجود داشته باشد، سرور آن را رمزگشایی می‌کند، `ContractData.Checksum` را با SHA‑256 بایت‌ها مقایسه می‌کند و فایل را با نام `<checksum>.wasm` ذخیره می‌کند.

ارسال مجدد همان درخواست مخرب نیست (idempotent): سرور امضا را پیدا می‌کند و هش بلاک موجود را برمی‌گرداند.

#### `gate.ReadData/readData`

`Limit` (۱ تا ۱۰۰) و `Skip` الزامی هستند و **حداقل یک فیلتر** باید داده شود:

| فیلتر | نوع | تطبیق |
|---|---|---|
| `UID` | string | `senderUid` یا `receiverUid` برابر UID. |
| `SenderUID` | string | `blockData.senderUid`. |
| `SenderRole` | int64 | `blockData.senderRole`. |
| `SenderPubKey` | string | `blockData.senderPubKey`. |
| `ReceiverUID` | string | `blockData.receiverUid`. |
| `ReceiverRole` | int64 | `blockData.receiverRole`. |
| `BlockHash` | string | `_id`. |
| `PreBlockHash` | string | `blockMeta.preBlockHash`. |
| `NodeUID` | string | `blockMeta.nodeUid`. |
| `TimeStampFrom` / `TimeStampTo` | int64 | بازهٔ `blockData.timeStamp` **بر حسب میلی‌ثانیه**. |
| `UseContract` | string | `blockData.useContract`. |

بلاک‌هایی که اعتبارسنجی کامل آن‌ها (امضای کلاینت، امضای نود یا هش) ناموفق است نادیده گرفته می‌شوند و بلاک‌های `Boycott` هرگز برگردانده نمی‌شوند.

---

### پروتکل WebSocket

نظیرها با هدر `X-UID` (فقط یک فیلتر سریع پیش از ارتقا) به `ws://host:WP` وصل می‌شوند و سپس یک **چالش/پاسخ امضایی** را کامل می‌کنند. پیام‌ها JSON هستند.

**متدها**

| متد | معنی | پاسخ |
|---|---|---|
| `get` | یافتن یک بلاک بر اساس ترکیبی از فیلدها. | `WS_Res` |
| `add` | تکثیر یک بلاک؛ گیرنده ساختار، هر دو امضا و هش را بررسی می‌کند، آن را ذخیره می‌کند و تکراری را موفق می‌شمارد. بلاک نامعتبر دور ریخته می‌شود (نه قرنطینه). | `WS_Res` |
| `inventory` | یک صفحه از شناسه‌های بلاک به ترتیب صعودی `_id` به‌همراه `nextCursor`/`hasMore`. | `WS_Inventory_Res` |
| `getblocks` | دریافت بلاک‌های مربوط به شناسه‌های خواسته‌شده (محدود). | `WS_Blocks_Res` |

همگام‌سازی مبتنی بر موجودی (inventory) است و هرگز به زمان‌سندکا تکیه نمی‌کند؛ بنابراین نظری که آفلاین بوده، ری‌استارت شده یا بلاک‌ها را نامرتب دریافت کرده، در دور بعد همگام می‌شود. `sync` قدیمی حذف شد.

سه حلقهٔ پس‌زمینه اجرا می‌شود: پینگ و اتصال مجدد هر ۱۰ ثانیه، همگام‌سازی اولیه تا رسیدن به «چیزی جدید نیست»، و همگام‌سازی دوره‌ای هر ۵ ثانیه.

---

### قراردادهای هوشمند (BVM)

قراردادها ماژول‌های WebAssembly هستند که تابعی با نام `smart_contract` را export می‌کنند:

```wat
(module
  (func (export "smart_contract"))
)
```

سه محدودیت اعمال می‌شود:

1. **CPU** — `EXECUTION_TIMEOUT_SECONDS` (پیش‌فرض ۱۰ ثانیه)؛ حلقه‌های بی‌نهایت متوقف می‌شوند.
2. **حافظه** — `MAX_MEMORY_PAGES` (پیش‌فرض ۲۵۶ صفحه = ۱۶ مگابایت).
3. **API** — فقط توابع ثبت‌شده با `bvm.AddHostFunction` در دسترس‌اند.

```go
if err := bvm.RunBVM("smart_contracts/example.wasm"); err != nil {
	log.Fatal(err)
}
```

فایل‌های آپلودی با چک‌سام SHA‑256 ذخیره می‌شوند، نه با نام انتخابی کلاینت، بنابراین امکان نوشتن بیرون از `SMART_CONTRACT_DIR` وجود ندارد.

---

### تست‌ها

مجموعهٔ تست خودبسنده است: از شبیه‌ساز MongoDB در حافظه (`db/dbtest`) و یک ماژول WASM دست‌ساز استفاده می‌کند، بنابراین **به سرور MongoDB، شبکه یا کامپایلر نیازی نیست**.

```bash
# همه
go test ./...

# verbose
go test -v ./...

# پوشش
go test -cover ./...

# یک بسته
go test ./utils/ -run TestSignVerifyRoundTrip -v
```

اگر رم ماشین کم باشد، ممکن است Go در ساخت هم‌زمان همهٔ باینری‌های تست با خطای `fatal error: runtime: cannot allocate memory` مواجه شود. در این حالت:

```bash
go test -p 1 ./...
```

**پوشش بسته‌ها**

| بسته | چه چیزی پوشش داده می‌شود |
|---|---|
| `utils` | هش SHA‑256 با جداسازی دامنه، Delium آموزشی، امضای ECDSA (دستکاری، کلید اشتباه، کدگذاری نامعتبر)، توابع hex/فایل و اعتبارسنج‌ها. |
| `config` | همهٔ بارگذارها، اولویت `.env`، `Validate()`، قواعد مسیر هش و طول ثابتهای جنسیس. |
| `db` | هش بلاک، اعتبارسنجی امضا، ساخت و تکرار جنسیس، CRUD، صفحه‌بندی و نتیجهٔ خالی. |
| `db/dbtest` | شبیه‌ساز: عملگرها، مسیرهای نقطه‌دار، مرتب‌سازی، skip/limit و هم‌زمانی. |
| `protos` | اعتبارسنجی هر فیلد، احراز هویت، محدودکنندهٔ نرخ، AddData/ReadData و آپلود قرارداد. |
| `ws` | بررسی مبدأ، احراز هویت چالش/پاسخ، هندلرهای get/add/inventory/getblocks، محدودکنندهٔ نرخ، همگام‌سازی مبتنی بر موجودی و تکثیر. |
| `bvm` | اجرای ماژول، فایل ناموجود، WASM نامعتبر، نبود export، کرانل زمانی و سقف حافظه. |
| `logger` | مقداردهی اولیه و تغییر خروجی. |
| `main` | توقف سریع روی تنظیمات نامعتبر. |

---

### تغییرات این نسخه

این نسخه یک بازبینی صحت و ایمنی است. تغییرات زیر **برای پروتکل قابل مشاهده‌اند** و به پایگاه دادهٔ تازه نیاز دارند:

1. **امضای با عرض ثابت.** `r` و `s` هر کدام با صفر به ۳۲ بایت رسیده می‌شوند. کد قبلی آن‌ها را با طول متغیر الحاق می‌کرد و تقسیم امضا همیشه ممکن نبود.
2. **هش بلاک و پیام امضا‌شده اکنون `UseContract` و فرادادهٔ قرارداد را هم شامل می‌شوند.**
3. **فیلترهای زمانی `ReadData` میلی‌ثانیه‌ای هستند** (قبلاً با ثانیه اعتبارسنجی ولی با میلی‌ثانیه مقایسه می‌شد و هیچ نتیجه‌ای نداشت).
4. **نتیجهٔ خالی دیگر خطا نیست** و `mongo.ErrNoDocuments` حذف شده است.
5. **آپلود قرارداد، چک‌سام را با خود بایت‌ها مقایسه می‌کند** (قبلاً فایلی که باید از قبل روی دیسک وجود داشت بررسی می‌شد و اولین آپلود همیشه شکست می‌خورد).
6. **طول ثابتهای جنسیس اصلاح شد** (هش پیشین ۶۲ و امضا ۱۴۲ کاراکتر بود و بلاک اول هرگز معتبر نمی‌شد).
7. **نوشتن‌های WebSocket سریالایز شدند** (gorilla/websocket فقط یک نویسنده را مجاز می‌داند).
8. **همگام‌سازی بلاک نوک را رد می‌کند و فرادادهٔ قرارداد را حفظ می‌کند.**
9. **BVM اکنون محدودیت‌های مستندشده را اعمال می‌کند** (قبلاً کرانل زمانی و سقف حافظه نداشت).
10. **تنظیمات در شروع، اعتبارسنجی می‌شوند** و `log.Fatal` از کتابخانهٔ هش حذف شد.

سایر اصلاحات: حذف چاپ دیباگ در `AddData`، پاسخ idempotent به درخواست تکراری، اصلاح لاگ موفقیت، و جلوگیری از ذخیرهٔ اتصال nil در `ConnectToOneServer`.

---

### محدودیت‌های شناخته‌شده

- **بدون اجماع.** بلاک‌ها امضا و زنجیره می‌شوند اما نودها رأی نمی‌دهند؛ اکثریت مخرب می‌تواند تاریخچه را بازنویسی کند.
- **بدون حساب و موجودی.**
- **بدون TLS.** از پروکسی معکوس استفاده کنید.
- **تکثیر هم‌زمان.** `AddData` منتظر پاسخ نظیرها می‌ماند.
- **بدون مهاجرت خودکار داده.** نمایهٔ هش v2 با پایگاه دادهٔ v1 ناسازگار است؛ برای پشتیبان‌گیری/بازنشانی به `docs/MIGRATION.md` مراجعه کنید.
- **کلید نود به عهدهٔ اپراتور است.** هر نود برای ساخت بلاک و شماره‌گیری به نظیرها به یک کلید P‑256 نیاز دارد.
- **کلید API یک راز مشترک متنی است.**

---

### نکات امنیتی

- نود را پشت پروکسی معکوس با TLS قرار دهید.
- `config/api_key.json`، `config/.env` و `config/node_keys.json` را در استقرار واقعی از نسخه‌گیری خارج کنید. **هر کلیدی که قبلاً commit شده باید بچرخد.**
- سندباکس WASM یک محدودکنندهٔ منابع است، نه مرز کامل جداسازی؛ قراردادهای آپلودی را نامعتبر فرض کنید.
- احراز هویت نظیرهای ورودی با چالش/پاسخ امضایی بر اساس کلید ثبت‌شده در `dns_seed.json` است؛ هدر `X-UID` جعلی به‌تنهایی پذیرفته نمی‌شود. پورت WebSocket را فقط به شبکهٔ خصوصی محدود کنید.
- محدودکنندهٔ نرخ به ازای هر کلید API و هر متد است و پس از ۵ دقیقه بیکاری پاک می‌شود؛ نظیرهای WebSocket نیز به ازای هر اتصال محدود شده‌اند.

---

### مجوز

این پروژه بخشی از اکوسیستم **BLVchain** است. برای اطلاعات مجوز به مخزن مراجعه کنید.

---

*برای جزئیات بیشتر، کد هر بسته را مطالعه کنید.*