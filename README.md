# BLVchain Core

A **blockchain-based data integrity and verification system** built in Go, designed for secure, immutable data storage with support for smart contracts, verifiable credentials, and a distributed node network.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Modules](#modules)
  - [Config (`config/`)](#config-config)
  - [Database Layer (`db/`)](#database-layer-db)
  - [gRPC & API Layer (`proto/`)](#grpc--api-layer-proto)
  - [ACPT — Merkle Tree State (`acpt/`)](#acpt--merkle-tree-state-acpt)
  - [BVM — Blockchain Virtual Machine (`bvm/`)](#bvm--blockchain-virtual-machine-bvm)
  - [Utilities (`utils/`)](#utilities-utils)
  - [Logger (`logger/`)](#logger-logger)
- [Data Flow](#data-flow)
- [Configuration](#configuration)
- [Running the Node](#running-the-node)
- [API Reference](#api-reference)
- [Security & Rate Limiting](#security--rate-limiting)
- [Development](#development)
- [License](#license)

---

## Overview

**BLVchain** is a blockchain node implementation that provides:

- **Immutable Block Storage** — Each block is cryptographically linked to its predecessor via a hash chain, ensuring data integrity.
- **Merkle Tree State Management** — A persistent, verifiable key-value state stored as a binary Merkle tree (ACPT) in MongoDB.
- **Smart Contract Execution** — A WebAssembly (WASM) sandbox (BVM) that runs user-uploaded smart contracts with strict CPU, memory, and API limits.
- **Verifiable Credentials** — Support for storing and verifying credential data on-chain.
- **gRPC API** — Two primary services (`AddData` / `ReadData`) exposed over gRPC with per-client rate limiting.
- **Write-Ahead Logging (WAL)** — Crash-safe persistence for state mutations before they are committed to the database.
- **DNS Seed Discovery** — Node-to-node discovery via a DNS seed list for decentralized network bootstrapping.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         BLVchain Node                        │
│                                                               │
│  ┌──────────┐    ┌──────────────┐    ┌───────────────────┐ │
│  │  Config   │───▶│   Database    │◀───│   gRPC Server    │ │
│  │  (.env)   │    │  (MongoDB)    │    │  (proto/*)       │ │
│  └──────────┘    └──────┬───────┘    └─────────┬─────────┘ │
│                          │                      │           │
│                          ▼                      ▼           │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Core Pipeline                        │  │
│  │                                                         │  │
│  │  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌──────┐  │  │
│  │  │ WAL  │──▶│  ACPT    │──▶│   BVM    │──▶│Utils │  │  │
│  │  │(acpt)│   │(Merkle)  │   │(WASM)   │   │(crypto)│  │  │
│  │  └──────┘   └──────────┘   └──────────┘   └──────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Logger (lumberjack)                  │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │  │
│  │  │ gRPC     │ │ WebSocket│ │ Internal  │ │ Smart    │ │  │
│  │  │ success/ │ │ success/  │ │           │ │ Contract │ │  │
│  │  │ fail      │ │ fail      │ │           │ │ success/ │ │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
.
├── main.go                  # Node entry point — bootstraps DB, gRPC, indexes
├── go.mod                   # Go module definition (blvchain/core)
├── go.sum                   # Dependency checksums
├── .air.toml                # Air live-reload config (development)
├── build.sh                 # Build script
├── run_dev.sh               # Development runner
├── make_proto.sh            # Protobuf code generation
├── s1.sh / s2.sh / s3.sh    # Utility scripts
├── note.todo                 # Project notes / todos
│
├── config/                   # Environment & file-based configuration
│   ├── .env                 # Environment variables
│   ├── lib.go               # Config loader (godotenv, JSON files)
│   ├── structs.go           # Config struct definitions
│   ├── var.go               # Global runtime variables
│   ├── api_key.json         # Allowed API keys
│   ├── delium_config.json   # Hashing path configuration
│   └── dns_seed.json        # DNS seed list for node discovery
│
├── db/                       # MongoDB data layer
│   ├── lib.go               # Block hashing & genesis logic
│   ├── model.go             # Block / BlockMeta / BlockData structs
│   └── mongo.go             # MongoDB connection & CRUD operations
│
├── proto/                    # gRPC service definitions & implementations
│   ├── client.proto         # Protobuf schema (AddData / ReadData)
│   ├── node.proto           # Node-level protobuf schema
│   ├── client.pb.go         # Generated protobuf Go code
│   ├── client_grpc.pb.go   # Generated gRPC server/client code
│   ├── client.service.go    # gRPC service implementations
│   ├── lib.go               # Request validation & auth
│   ├── ratelimit.go         # Per-client rate limiter
│   └── struct.go            # Service structs
│
├── acpt/                    # Authenticated Chained Persistent Tree
│   ├── berkle.go            # Merkle tree insert / update logic
│   ├── manager.go           # TreeManager — WAL + DB orchestration
│   ├── models.go            # DBNode / KeyValue / GlobalState models
│   └── wal.go               # Write-Ahead Log (WAL) implementation
│
├── bvm/                     # Blockchain Virtual Machine
│   ├── bvm.go               # WASM runtime — wazero sandbox
│   └── bvm.service.go       # Host functions exposed to WASM
│
├── utils/                   # Cryptographic & utility helpers
│   ├── lib.go               # General utilities (time, JSON, validation)
│   ├── delium.go            # Delium hashing (D256C / D512C)
│   └── elliptic.go         # ECDSA P-256 signature verification
│
├── logger/                  # Structured logging (lumberjack)
│   └── logger.go           # Multi-file, rotated loggers
│
└── wal_logs/               # Write-Ahead Log files
    └── wal_000001.log      # WAL persistence
```

---

## Modules

### Config (`config/`)

The [`config`](./config/) package centralizes all node configuration:

- **Environment variables** — Loaded via [`godotenv`](./config/lib.go:19) from [`config/.env`](./config/.env).
- **JSON config files** — [`delium_config.json`](./config/delium_config.json) defines hashing paths for block integrity; [`dns_seed.json`](./config/dns_seed.json) provides seed nodes for peer discovery; [`api_key.json`](./config/api_key.json) maps allowed client API keys.
- **Global variables** — [`var.go`](./config/var.go) initializes MongoDB collection references, rate-limit constants, genesis block placeholders, and runtime parameters (execution timeout, memory pages, etc.).

### Database Layer (`db/`)

The [`db`](./db/) package manages all MongoDB interactions:

- **Connection** — [`ConnectToMongoDB()`](./db/mongo.go:14) establishes a client using `MONGO_URI` from config.
- **Block Model** — [`Block`](./db/model.go:41) is the core data structure with `BlockMeta` (pre-hash, node UID, timestamp) and `BlockData` (sender/receiver, signature, contract data, verifiable credentials).
- **Genesis** — [`Genesis_check()`](./db/lib.go:14) creates the first genesis block if none exists, using zeroed-out placeholder values.
- **Block Hashing** — [`BlockHashMaker()`](./db/lib.go:51) constructs a deterministic hash from all block fields (including contract metadata and VC data) using SHA-256.
- **CRUD** — Functions for single/multiple block insertion and paginated queries with ascending/descending sort.

### gRPC & API Layer (`proto/`)

The [`proto`](./proto/) package implements the node's external API:

- **Protobuf Schema** — Defined in [`client.proto`](./proto/client.proto) and [`node.proto`](./proto/node.proto), with two services:
  - **`AddData`** — [`/gate.AddData/addData`](./proto/client.service.go:18) accepts a `BlockData`, validates the signature via ECDSA P-256, checks the sender's last block for chain linking, and optionally handles WASM smart contract uploads.
  - **`ReadData`** — [`/gate.ReadData/readData`](./proto/client.service.go) returns paginated blocks with optional filters (sender UID, receiver UID, block hash, timestamp range).
- **Validation** — [`validateAddDataRequest()`](./proto/lib.go:25) and [`validateReadDataRequest()`](./proto/lib.go:71) enforce field lengths, timestamp ranges, and data size limits using `go-playground/validator`.
- **Rate Limiting** — [`RateLimiter`](./proto/ratelimit.go:24) provides per-method, per-client token-bucket rate limiting with TTL-based cleanup for idle entries.

### ACPT — Merkle Tree State (`acpt/`)

The [`acpt`](./acpt/) package implements an **Authenticated Chained Persistent Tree** — a binary Merkle tree stored in MongoDB:

- **Tree Structure** — [`DBNode`](./acpt/models.go:12) stores key, value hash, left/right child hashes, and height (for AVL balancing).
- **Insert/Update** — [`ApplyChanges()`](./acpt/berkle.go:22) takes the current root hash and a batch of `KeyValue` pairs, recursively traverses the tree (BST comparison), and returns the new root plus a map of nodes to persist.
- **WAL** — [`WAL`](./acpt/wal.go:9) writes each batch to a binary log file before committing to MongoDB, ensuring crash recovery.
- **TreeManager** — [`NewTreeManager()`](./acpt/manager.go:25) orchestrates the full flow: WAL append → Merkle calculation → DB flush.

### BVM — Blockchain Virtual Machine (`bvm/`)

The [`bvm`](./bvm/) package provides a **WebAssembly sandbox** for smart contract execution:

- **Runtime** — Uses [`wazero`](./bvm/bvm.go:9) (zero-dependency WASM runtime) with three safety walls:
  1. **CPU** — Context timeout (`EXECUTION_TIMEOUT`, default 10s).
  2. **Memory** — Page limit (`MAX_MEMORY_PAGES`, default 256 pages = 16 MB).
  3. **API** — Only registered host functions are exposed to the WASM module.
- **Host Functions** — [`AddHostFunction()`](./bvm/bvm.go:25) registers Go functions (e.g., [`getOneBlockByHash`](./bvm/bvm.service.go:13) for reading blocks from the database) that the WASM contract can call.
- **Execution** — [`RunSmartContract()`](./bvm/bvm.go:38) loads a `.wasm` file, instantiates it, and calls the exported `"smart_contract"` function.

### Utilities (`utils/`)

The [`utils`](./utils/) package provides cryptographic and general helpers:

- **Delium Hashing** — [`D256C()`](./utils/delium.go:19) and [`D512C()`](./utils/delium.go:69) implement a configurable multi-step hashing pipeline (SHA-256/SHA-512 with additive and truncation steps).
- **ECDSA Verification** — [`Verify()`](./utils/elliptic.go:13) validates ECDSA P-256 signatures by decompressing the public key, computing the UID from the message, and verifying `r || s`.
- **UID Checker** — [`NodeUidChecker()`](./utils/lib.go:75) validates node UIDs against the DNS seed list.
- **Helpers** — Time conversion, JSON marshaling, string/int/float64 parsing, and URL query parameter building.

### Logger (`logger/`)

The [`logger`](./logger/) package initializes **rotating log files** via `lumberjack`:

- **Log categories** — gRPC (success/fail), WebSocket (success/fail), internal, smart contract (success/fail), WAL (success/fail), verifiable credential (success/fail), and signature.
- **Each logger** — Max 2 MB per file, with automatic rotation and local timestamps.

---

## Data Flow

1. **Client** sends a `BlockData` via gRPC `AddData`.
2. **Validation** — [`validateAddDataRequest()`](./proto/lib.go:25) checks field lengths and timestamp bounds.
3. **Authentication** — [`validateAuth()`](./proto/client.service.go) extracts the API key from gRPC metadata.
4. **Signature Verification** — [`Verify()`](./utils/elliptic.go:13) validates the ECDSA P-256 signature.
5. **Chain Linking** — Finds the sender's last block to set `PreBlockHash`.
6. **Block Construction** — Creates a new `Block` with the current node's UID and timestamp.
7. **Smart Contract** — If `ContractBase64` is present, decodes and saves the WASM file.
8. **Storage** — Inserts the block into MongoDB via [`InsertOneBlock()`](./db/mongo.go:30).
9. **Response** — Returns `AddDataResult` with success status and block hash.

---

## Configuration

All configuration is managed through [`config/`](./config/):

| Variable | Source | Default | Description |
|---|---|---|---|
| `MONGO_URI` | `.env` | — | MongoDB connection string |
| `DB` | `.env` | `BLVchain` | Database name |
| `GP` | `.env` | `:50051` | gRPC listen port |
| `SELF_UID` | `.env` | — | This node's unique identifier |
| `MAX_DATA_SIZE_KB` | `.env` | `2048` | Max block data size (KB) |
| `READ_DATA_R` | `.env` | `0.1` | Read rate limit (tokens/sec) |
| `READ_DATA_BURST` | `.env` | `30` | Read burst limit |
| `ADD_DATA_R` | `.env` | `0.1` | Add rate limit (tokens/sec) |
| `ADD_DATA_BURST` | `.env` | `5` | Add burst limit |
| `EXECUTION_TIMEOUT` | `var.go` | `10s` | WASM execution timeout |
| `MAX_MEMORY_PAGES` | `var.go` | `256` | WASM memory limit (16 MB) |

---

## Running the Node

### Prerequisites

- Go 1.24+
- MongoDB instance
- Go module dependencies (see `go.mod`)

### Quick Start

```bash
# 1. Clone the repository
git clone <repo-url> && cd blvchain/core

# 2. Set environment variables
cp config/.env.example config/.env
# Edit config/.env with your MONGO_URI and other settings

# 3. Build
./build.sh

# 4. Run (development)
./run_dev.sh

# 5. Or run directly
go run main.go
```

### Environment Variables

Set these in `config/.env`:

```env
MONGO_URI=mongodb://localhost:27017
DB=BLVchain
GP=:50051
SELF_UID=<your-32-hex-char-node-uid>
```

### gRPC Endpoints

| Service | Method | Endpoint |
|---|---|---|
| `AddData` | `addData` | `/gate.AddData/addData` |
| `ReadData` | `readData` | `/gate.ReadData/readData` |

---

## API Reference

### AddData

**`/gate.AddData/addData`**

Request:
```protobuf
message BlockData {
    bytes SenderUID = 1;      // 16 bytes
    bytes SenderPubKey = 2;   // 33 bytes (compressed P-256)
    bytes Signature = 3;       // 64 bytes (r || s)
    bytes ReceiverUID = 4;     // 16 bytes
    bytes Data = 5;
    bytes ContractBase64 = 6;  // Optional: WASM binary (base64)
    bytes UseContract = 7;     // Optional: 33 bytes
    Contract ContractData = 8;
    int64 TimeStamp = 9;
}
```

Response:
```protobuf
message AddDataResult {
    bool IsSuccess = 1;
    string Log = 2;
    bytes BlockHash = 3;
}
```

### ReadData

**`/gate.ReadData/readData`**

Request:
```protobuf
message ReadDataRequest {
    int64 Limit = 1;           // 1–100
    int64 Skip = 2;
    bytes SenderUID = 3;      // Optional filter
    bytes SenderPubKey = 4;
    bytes ReceiverUID = 5;
    bytes BlockHash = 6;
    bytes PreBlockHash = 7;
    bytes NodeUID = 8;
    int64 TimeStampFrom = 9;
    int64 TimeStampTo = 10;
    bytes UseContract = 11;
    bytes UID = 12;
}
```

Response:
```protobuf
message ReadDataResult {
    bool IsSuccess = 1;
    string Log = 2;
    repeated Block Data = 3;
}
```

---

## Security & Rate Limiting

### Authentication

API keys are validated from gRPC metadata:
- `authorization` header
- `x-api-key` header
- Falls back to peer IP

### Rate Limiting

The [`RateLimiter`](./proto/ratelimit.go:24) enforces per-client token-bucket limits:

| Method | Rate | Burst |
|---|---|---|
| `addData` | 0.1/s | 5 |
| `readData` | 0.1/s | 30 |
| Default | 5/s | 10 |

Idle entries are cleaned up after 5 minutes of inactivity.

### Smart Contract Sandbox

The BVM enforces three safety walls:

1. **CPU** — Context timeout kills infinite loops after 10 seconds.
2. **Memory** — WASM memory is capped at 16 MB (256 pages × 64 KB).
3. **API** — Only explicitly registered host functions are exposed to the contract.

---

## Development

### Generating Protobuf Code

```bash
./make_proto.sh
```

This regenerates `*.pb.go` and `*_grpc.pb.go` from the `.proto` files in `proto/`.

### Live Reload

The project uses [Air](https://github.com/air-verse/air) for hot-reloading during development:

```bash
# air is configured in .air.toml
air
```

### Adding a New Host Function

1. Define the function in [`bvm/bvm.service.go`](./bvm/bvm.service.go).
2. Register it with [`AddHostFunction()`](./bvm/bvm.go:25) in `init()`.
3. The WASM contract can now call it by name.

---

## License

This project is part of the **BLVchain** ecosystem. See license information in the repository root.

---

*For more details, refer to the source code in each package directory or the project's [`note.todo`](./note.todo) file.*
