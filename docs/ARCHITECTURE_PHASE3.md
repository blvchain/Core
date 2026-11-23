# BLVChain Core — Phase 3 Architecture (CometBFT Consensus)

> **Status:** DIRECTION + IMPLEMENTATION PLAN APPROVED (Rev 4) — IMPLEMENTATION
> NOT YET AUTHORIZED. The architecture and the phased implementation plan (§16)
> are approved. **No source code, dependency, configuration file, or database
> content may change until the user explicitly authorizes Phase 0**, which itself
> may not begin until **Phase S (dependency & security review, §17) is completed
> and signed off**. This document records the approved decisions and specifies the
> target architecture, data model, transaction lifecycle, migration,
> failure/recovery model, acceptance criteria, and implementation plan.
>
> **Author of record:** architecture pass, 2026-11-22.
> **Revision 4 — 2026-11-23:** plan approved subject to 8 conditions. Adds
> **Phase S** (dependency & security review) as a hard pre-Phase-0 gate and expands
> §17 with SBOM/vulnerability-scanning, justified version selection, upgrade +
> security-patch policy, toolchain uniformity, and a coordinated-consensus-upgrade
> definition; strengthens Phase 0 (independently verified cross-platform golden
> vectors), Phase 2 (hard durability gate blocking Phase 3), Phase 3 (real
> network/failure-domain testing), and Phase 6 (full staging rehearsal); makes
> review sign-off mandatory to advance any phase.
> **Revision 3 — 2026-11-23:** incorporates the direction approval and its seven
> decisions. Contract payload kept self-contained with explicit tested size/
> resource limits (§3.5); `AddData` wrapper gets a configurable timeout that is
> never treated as failure (§7.7); chain ID enters the tx/TxID replay-protection
> domain and `ConsensusMeta` (§3.4, §6.1, §10.5); AppHash divergence triggers
> automatic fail-closed isolation with no auto-rejoin (§9.2); legacy migration
> refusal + auditable operator resolution confirmed (§12); validators-only Phase 3
> deployment (§4.5.2); MongoDB Replica Set production requirement with per-
> validator independent instances (§15.1). Adds the size-limit tests (§14.11) and
> AddData-timeout / chain-id tests (§14.5, §14.9). **Direction APPROVED; still NOT
> authorized for implementation** — see the status line above.
> **Revision 2 — 2026-11-23:** incorporates the design review. Corrects the
> AppHash guarantee (§9), separates ordinary receipts from verifiable finality
> proofs (§7.3), adds the P2P identity/authorization model (§4.5), rewrites the
> legacy-migration algorithm to detect and refuse conflicting sender tips
> (§12), makes the ABCI crash-recovery state machine exact (§11), makes the
> Phase 3 validator set immutable (§10), adopts the remaining technical defaults
> (§15), adds canonical transaction serialization (§3.4), and expands the
> acceptance tests (§14) and CometBFT source references (Appendix B).
> **Supersedes:** the "Architecture" section of `README.md` for consensus,
> ordering, finality, networking, and the client API. The README narrative
> (per-sender gossip chains, WS mesh, synchronous AddData) describes the
> pre-Phase-3 system and will be reconciled only after approval.

---

## 0. Scope and the one-paragraph summary

Phase 3 replaces BLVChain's leaderless, eventually-consistent gossip model with
**CometBFT** as the single authoritative source of global block ordering and
finality. Today there is *no* consensus: each node creates a block as a side
effect of each client `AddData` gRPC call, links it to that sender's previous
block via `PreBlockHash`, signs it, writes it to MongoDB, and gossips it to a
static full-mesh of WebSocket peers who accept any structurally valid,
correctly signed block whose predecessor they already hold. There is no total
order, no fork choice, no proposer, no validator set, and no finality.

Phase 3 keeps the **client-facing cryptographic identity and record format
unchanged** (P-256 key-derived UIDs, low-S ECDSA signatures, domain-separated
SHA-256 content IDs) and keeps contracts **storage-and-validation-only** (no
WASM execution in consensus or finalization). It introduces an **in-process
CometBFT node** whose consensus produces an authoritative, totally ordered,
BFT-final log; the existing gRPC "gate" becomes an **asynchronous transaction
submission API with receipts**, and CometBFT's own P2P + block sync **replace**
the WS gossip/inventory layer entirely.

---

## 1. Approved decisions (Phase 3 baseline)

These are the approved decisions this design is built on. They are normative.

### 1.1 Validator set
- **4 validators** for the initial production configuration, **equal voting power**.
- Documented **BFT assumption n ≥ 3f + 1**: with n = 4 the tolerated fault
  count is **f = 1**. See §13 for the precise safety/liveness consequences.
- Quorum = **⌈2n/3⌉ = 3** of 4. Liveness requires ≥ 3 correct, connected
  validators; safety holds as long as ≤ 1 validator is Byzantine.
- Each validator MUST hold its **consensus key independently** and SHOULD run on
  **independent infrastructure / failure domains**. Four processes on one host
  are **one** security participant, not four, and MUST NOT be treated as a
  4-node deployment for the purposes of the n ≥ 3f + 1 claim.

### 1.2 AddData API
- **Asynchronous** transaction submission. Submission returns **acceptance**,
  not finalization.
- A transaction **status** model and a **`GetReceipt`** RPC are introduced.
- Backward compatibility is preserved via a synchronous `AddData` convenience
  (see §7.7) or documented as a breaking change where it cannot be.
- A transaction is **never** reported finalized before CometBFT commits the
  block that contains it.
- Duplicate submissions, retries, timeouts, and client disconnects are defined
  in §7.6.
- Whether an accepted-but-uncommitted transaction can be lost after restart is
  answered in §7.6 / §11.

### 1.3 Validator-set management
- **Genesis-based** validator configuration, **immutable after genesis** for the
  life of a Phase 3 chain. Adding/removing/rotating validators or changing power
  during normal operation is **unsupported**; the only way to change the set is
  the exceptional **chain-migration** procedure (§10.5), **not** an in-place
  `genesis.json` edit + restart (§10.6).
- The internal architecture is designed so **on-chain validator-set updates**
  (add/remove, key rotation, voting-power change) can be introduced in a later
  phase **without redesigning the consensus integration** (§10). The ABCI
  validator-update return path exists from day one and is empty in Phase 3.

### 1.4 Smart contracts
- **Storage and structural validation only.** No WASM execution and no
  arbitrary user-code execution during consensus or block finalization in
  Phase 3 (§6.4).

### 1.5 Additional mandatory requirements (all addressed)
1. CometBFT is the single authority for ordering + finality; no competing
   chain-selection survives (§5).
2. The per-sender `PreBlockHash` relationship is preserved inside globally
   ordered blocks; sender continuity, duplicates, out-of-order submissions, and
   invalid sender references are defined (§8).
3. CometBFT block hashes, BLVChain record IDs, transaction IDs, and client
   signatures are distinct and explicitly defined; existing identities are not
   redefined (§3).
4. Exact CometBFT version, Go compatibility, embedding API, and integration
   approach are verified (§4).
5. Crash recovery and atomicity between consensus commits and MongoDB
   persistence are defined; a committed block is never permanently absent from
   application state (§11).
6. Deterministic application behavior across validators (§9).
7. Migration strategy that does not falsely claim historical per-sender chains
   became globally finalized history (§12).
8. Explicit safety and liveness acceptance criteria, including partition,
   Byzantine, restart, duplicate, and conflicting-proposal tests (§14).

---

## 2. Terminology

This document uses the following terms precisely. The overloading of "block" in
the existing code (a BLVChain record is called a `Block`) is the single largest
source of confusion, so the mapping is fixed here.

| Term | Meaning in Phase 3 |
|---|---|
| **Record** | One client submission. The existing `db.Block` / `db.BlockData` structure. Still called a "block" in legacy code; referred to as a **record** here to disambiguate. Content-addressed by its **Record ID**. |
| **Record ID** | The existing `db.Block.ID` — domain-separated SHA-256 over the record's content fields (§3). **Unchanged.** |
| **Transaction (tx)** | The serialized, client-signed `BlockData` submitted to the mempool. One record ⇒ one tx. |
| **Tx ID** | Hash of the canonical tx bytes. The CometBFT mempool/consensus identity of the submission. |
| **CometBFT block** | A consensus batch of ordered txs at a given **height**, with an AppHash and commit signatures. |
| **CometBFT block hash** | Hash of the CometBFT block header. The consensus/finality identity. |
| **Client signature** | The submitter's low-S P-256 ECDSA signature over `MessageMaker(BlockData)`. Unchanged. |
| **Finalized / committed** | The record's tx is included in a CometBFT block that has been committed by > 2/3 voting power and persisted by the ABCI `Commit` call. |

---

## 3. Cryptographic identity model (requirement 3)

Phase 3 introduces CometBFT, which brings its own consensus-level identities
(validator keys, block hashes). The existing BLVChain identities MUST NOT be
silently redefined. There are four distinct identities; each is produced by a
different key, covers a different message, and plays a different role.

### 3.1 The four identities

| # | Identity | Key / algorithm | Covers | Produced by | Where stored |
|---|---|---|---|---|---|
| A | **Client signature** | submitter P-256 key (secp256r1), low-S ECDSA, 128 hex | `MessageMaker(BlockData)` under domain `blvchain/v2/message` | client | `BlockData.Signature` |
| B | **Record ID** | none (hash) | all content fields incl. `PreBlockHash`, `NodeUID`, `BlockMeta.TimeStamp`, `BlockData` | the ABCI app, at execution | `db.Block.ID` (`_id`) |
| C | **Tx ID** | none (hash) | canonical tx bytes (= serialized signed `BlockData`) | client/node, at submission | CometBFT mempool; `Receipt.TxID` |
| D | **CometBFT block hash** + **validator (consensus) keys** | **ed25519** (CometBFT default) | CometBFT block header + vote commit signatures | CometBFT consensus | CometBFT block store; `Receipt.BlockHash` |

### 3.2 What changes and what does not

- **Unchanged:** identity **A** (client signature) and the UID derivation
  `Make_UID(pubkey) = first 32 hex of SHA-256("blvchain/v2/uid"‖0x00‖pubkey)`.
  Clients keep using P-256. The existing record content hash (**B**,
  `BlockHashMaker`) is **byte-for-byte unchanged**, so existing Record IDs
  remain reproducible and historical records remain verifiable.
- **New:** identity **D**. CometBFT validators sign consensus messages with
  **ed25519** keys. This is a *new, separate* key per validator, used **only**
  for consensus. It does not replace or reinterpret any P-256 key.
- **Demoted:** the legacy **node signature** (`BlockMeta.NodeSignature`, the
  P-256 `NodeMessage` signature over block ID + metadata). Its job —
  attesting that an authorized node produced/relayed this record — is now done
  by CometBFT commit signatures (identity D). See §3.3.

### 3.3 Fate of the legacy node signature (`NodeSignature`)

Today each record carries a P-256 `NodeSignature` over
`NodeMessage = "blvchain/node/v2"‖PreBlockHash‖NodeUID‖TimeStamp‖ID`, verified
by peers against the pinned `dns_seed.json` key. In Phase 3:

- `BlockMeta.NodeSignature` is **retained as a field but no longer produced or
  required** for new records. It is excluded from the Record ID (it always was),
  so dropping its production does not change Record IDs.
- The authorization it provided — "an authorized participant emitted this into
  the ordered log" — is now provided by CometBFT: a record only exists in a
  committed block if > 2/3 of the validator set (identity D) signed that block's
  commit. This is a *stronger* guarantee than a single node signature.
- `BlockValidator` loses its node-signature step (step 4) and keeps steps
  1–3 (struct shape, Record ID recomputation, client signature). The node-key
  trust anchor (`dns_seed.json`) is retired along with the WS mesh (§5.4).
- **Compatibility rule:** when replaying/verifying *legacy* records during
  migration (§12), the node-signature check is still applied to legacy data so
  historical records remain validated exactly as before. The check is simply
  not applied to records committed at `ConsensusHeight > 0`.

> **Key-type note:** CometBFT supports ed25519 and secp256k1 consensus keys
> (and `secp256k1eth` from v0.40). It does **not** support P-256 for consensus
> keys. We therefore standardize validators on **ed25519** and keep P-256 for
> the client/record layer. Mixing is intentional and keeps the two identity
> domains independent.

### 3.4 Canonical transaction encoding and TxID (requirement 8)

The **TxID** MUST be computed over a canonical, versioned, deterministic byte
encoding — **never** over "whatever protobuf happens to emit." Protobuf wire
output is not canonical across implementations: field ordering is not guaranteed,
unknown fields may be retained or dropped, and packed-vs-unpacked and default-
value handling vary by language/library/version. Two clients that produce the
same logical `BlockData` must derive the **same TxID**, or dedup, receipts, and
cross-implementation verification break.

**Definition.**

```
canonicalTx(chain) = "BLVT1" ‖ 0x00 ‖ canon(fields in fixed order)
TxID(chain)        = hex( SHA256( "blvchain/v2/txid" ‖ 0x00 ‖ canonicalTx(chain) ) )
```

`canon(...)` is the **existing `utils.EncodeFields` framing** reused for bytes:
each field is emitted as `"<byte-len>:<raw-bytes>;"` concatenated in a **fixed,
field-number order**. Using one canonical primitive for the signed message, the
Record ID, and the tx encoding keeps a single audited code path. (The 4-byte
`"BLVT1"` magic + `0x00` separator + the distinct `"blvchain/v2/txid"` domain tag
ensure a tx encoding can never be reinterpreted as a record or a signed message.)

**Field order (fixed, v1).** `ChainID` (node-supplied; see below), then the
submitted record: `SenderUID, SenderRole, SenderPubKey, Signature, ReceiverUID,
ReceiverRole, Data, UseContract, ContractData{Name,Version,Language,Compiler,
Description,Checksum,Author,License}, BlockData.TimeStamp, ContractBase64(raw
bytes, possibly empty)`. Integers use a fixed 8-byte big-endian encoding inside
the length-prefixed field (not varint, to remove a cross-impl variable);
`SenderRole`/`ReceiverRole` are signed 64-bit. Strings are UTF-8. Lengths are
decimal byte counts as the existing encoder already emits.

**Chain ID is in the tx domain (replay protection).** The canonical tx begins
with the CometBFT **chain ID**, so the TxID is **chain-scoped**: the same signed
`BlockData` submitted on two different chains yields two different TxIDs. This is
the replay-protection boundary the validator-replacement migration relies on
(§10.5): after a migration to a new chain ID, a transaction from the old chain
cannot be silently replayed into the new one's receipts/mempool, and the
per-chain `(senderUid, signature)` dedup (§7.6) — seeded with the migrated
history — rejects any cross-chain replay of the same record. The chain ID is
**node-supplied** (the client does not sign it); it is bound into the **TxID**
and stamped into each committed record's `ConsensusMeta.ChainID` (§6.1). The
client's signature still covers only `MessageMaker(BlockData)` (unchanged), so no
existing client key or Record ID is redefined.

**Deterministic serialization rules (normative):**
- No Go/proto maps anywhere in the tx; all fields are fixed and ordered.
- Integers are fixed-width big-endian; no platform-dependent `int`.
- The client signature is itself low-S canonical (§3, anti-malleability), so one
  `(message,key)` pair yields exactly one tx byte string.
- The contract payload is the **raw decoded bytes** (not the base64 wire form),
  length-prefixed; empty ⇒ zero-length field. This makes the record
  reconstructable from the tx alone and keeps storage-only contracts
  deterministic.

**Unknown-field handling.** The canonical form is produced from a **typed,
strict v1 schema**, not from raw protobuf. On decode:
- Any protobuf field **not** in the v1 field set is **rejected** (the tx is
  invalid), not silently dropped. This closes the malleability gap where two
  byte strings decode to the same tx under one implementation and differ under
  another.
- Missing required fields, wrong types, or non-canonical integer encodings are
  rejected.
- The decoder does **not** re-encode-and-compare as the only guard; canonical
  encoding is the single source of the TxID, and clients submit the typed record
  (gRPC), from which the node computes the canonical bytes once.

**Compatibility across client implementations.**
- The TxID is a pure function of the canonical bytes; any client that implements
  §3.4 derives the identical TxID without trusting the server's protobuf.
- If multi-language clients are supported, **golden test vectors** for the
  canonical encoding + TxID MUST be published and asserted by ≥ 2 independent
  implementations (Appendix C; acceptance test C1, §14.9).
- A future format change bumps the `"BLVT1"`/version marker; old and new
  encodings never collide by domain/magic.

> This encoding is used **only** for the CometBFT tx bytes / TxID. The **Record
> ID** continues to be computed by the unchanged `BlockHashMaker` over the record
> fields (§3, §6.1) and is a distinct identity.

### 3.5 Transaction, block, mempool, and resource limits (approved with contract payload)

Contract bytes stay **self-contained inside the tx** (§3.4). That is retained
**only because** the size/resource limits below are defined and tested (decision
1). Limits are enforced at four layers that MUST be set **consistently**:

| Layer | Limit | Value / rule |
|---|---|---|
| App: `Data` | `MAX_DATA_SIZE_KB` | 2048 KB (existing config) |
| App: contract bytes | `MAX_WASM_SIZE_KB` | 1024 KB (existing config) |
| **App: max canonical tx** | `MAX_TX_BYTES` | defined as the largest canonical tx the above can produce **plus encoding overhead + headroom** — set to **4 MiB** (must be ≥ the true max; verified by test) |
| Mempool: `max_tx_bytes` | CometBFT `mempool.max_tx_bytes` | **≥ `MAX_TX_BYTES`** (rejects oversize at admission) |
| Mempool: size (count) | CometBFT mempool caps | bounded; `CheckTx` cheaply rejects |
| Consensus: block | CometBFT `consensus.max_block_bytes` | **≥ one `MAX_TX_BYTES`** and sized for target throughput; `PrepareProposal` bounds total ≤ this |
| P2P frame | CometBFT `max_packet_msg_payload_size` | **≥ `consensus.max_block_bytes`** so blocks/txs are not fragmented below the block limit |

**Resource cost.** Phase 3 has **no WASM execution**, so per-tx cost is dominated
by one ECDSA verify + domain-separated SHA-256 + one idempotent DB upsert — all
bounded and cheap. **No gas metering is needed in Phase 3**; the binding
constraints are **size and count**, not compute. (A future execution phase would
add metering.)

**Normative rules.**
1. `MAX_TX_BYTES` is computed from the app limits, not guessed; a build-time
   assertion fails if a configured `MAX_DATA_SIZE_KB`/`MAX_WASM_SIZE_KB` would
   exceed `MAX_TX_BYTES`.
2. `mempool.max_tx_bytes ≤ consensus.max_block_bytes ≤ max_packet_msg_payload_size`
   so no layer rejects what a higher layer would have allowed.
3. `CheckTx` rejects any tx whose canonical bytes exceed `MAX_TX_BYTES`;
   `PrepareProposal` never proposes a block exceeding `consensus.max_block_bytes`.
4. All of the above are **pinned in CometBFT config and tested** (acceptance
   OS-series, §14.11).

---

## 4. CometBFT integration strategy (requirement 4)

### 4.1 Version selection and Go compatibility (verified, requirement 10)

- **Pin: CometBFT `v1.0.1`** — the latest v1.0.x patch tag (verified; no v1.0.2+
  exists at time of writing; the repo also carries a parallel `v0.40.0` line that
  is **not** used here). The v1.0.x line carries the formal versioning guarantees
  (PBTS, Data Companion API). Source: <https://github.com/cometbft/cometbft/releases/tag/v1.0.1>.
- **Go compatibility:** CometBFT v1.0.1's `go.mod` declares **`go 1.23.5`**;
  this project's `go.mod` declares **Go 1.24.0**, which satisfies it. Source:
  <https://github.com/cometbft/cometbft/blob/v1.0.1/go.mod>.
- **Default ports (v1.0.1):** P2P `tcp://0.0.0.0:26656`, RPC
  `tcp://127.0.0.1:26657`, ABCI app proxy `tcp://127.0.0.1:26658`. Source:
  `config/config.go` @ v1.0.1. (RPC/ABCI bind to localhost by default — keep it
  that way; only the gRPC gate faces clients. See §4.5.)
- **Package layout note (v1.0.x):** consensus code lives under
  **`internal/consensus/`** (not `consensus/`); execution/state live under
  top-level **`state/`**. This matters when reconciling call sites.
- **API-reconciliation (implementation step):** the `nm.NewNode(...)` /
  `proxy.NewLocalClientCreator(...)` wiring below is correct in shape; reconcile
  the exact argument list to the v1.0.1 signature when coding. (This is an
  implementation mechanic, Appendix A step 2, not a design change.)

### 4.2 In-process (built-in) embedding — verified as supported

In-process Go embedding is officially supported (this is exactly how the Cosmos
SDK runs CometBFT). The node and the ABCI app run in the same OS process; ABCI
calls are delivered via a local client. Concurrency note from the CometBFT docs:
the default local ABCI client serializes ABCI calls under a lock, so the app
**may assume `FinalizeBlock`/`Commit` are not called concurrently** and must not
expose mutable app state to the gRPC layer without its own synchronization.

Target wiring (shape; reconcile to pinned version):

```go
// pseudocode — reconcile signatures to the pinned CometBFT tag
app := blvapp.New(deps)                         // our ABCI 2.0 Application
node, err := nm.NewNode(
    cfg,                                       // *cfg.Config (CometBFT config)
    privValidator,                             // this node's ed25519 FilePV
    nodeKey,                                   // node identity key
    proxy.NewLocalClientCreator(app),          // in-process ABCI wiring
    nm.DefaultGenesisDocProviderFunc(cfg),     // serves genesis.json
    nm.DefaultDBProvider,                       // CometBFT block/state store
    nm.DefaultMetricsProvider(cfg.Instrumentation),
    logger,
)
node.Start()
```

- The ABCI app lives in a **new package** (e.g. `consensus/app`), separate from
  `protos` (gRPC), `db` (Mongo), and `ws` (to be removed). It depends on `db`
  (for persistence) and `utils` (for hashing/verification).
- `nm` = CometBFT `node` package. The legacy `core` binary at the repo root is a
  stale compiled artifact and is unrelated.

### 4.3 ABCI 2.0 method surface to implement

Our `Application` implements the ABCI 2.0 interface. Responsibilities:

| Method | Phase 3 behavior |
|---|---|
| `Info` | Return app version, `LastBlockHeight` = highest height durably committed to Mongo, and the matching `AppHash`. Used by CometBFT on restart to decide replay (§11). |
| `InitChain` | Load genesis `app_state`: validator set (for visibility/validation), `sender_tails` map, legacy anchor `FIRST_BLOCK_HASH`, chain params. Return initial validator updates (empty in Phase 3 — updates come only from genesis). |
| `CheckTx` | Stateless + stateful admission: shape/length checks, client signature + UID binding, size limits, and dedup (reject a tx already committed). This is the mempool gate. |
| `PrepareProposal` | (Proposer only.) Select/reorder txs from the mempool for the proposal. Phase 3: include valid txs in arrival order, bounded by `MaxBytes`; **deterministic** tie-breaking. Optional in Phase 3 (CometBFT default txs selection may be used); if implemented, see §9. |
| `ProcessProposal` | Validate a received proposal: all txs pass `CheckTx`, none duplicate committed state, byte/gas bounds respected. Reject otherwise. Deterministic — all validators reach the same accept/reject. |
| `FinalizeBlock` | **Execute the ordered txs deterministically** against committed state, assign each record its `PreBlockHash`/`NodeUID`/meta-time, compute the new state, and return the **`app_hash`** (+ empty `validator_updates` in Phase 3). MUST NOT make state durably visible. **(Verified: v1.0 returns `app_hash` here, not in `Commit`.)** |
| `Commit` | **Durably persist** the staged writes for this height to Mongo and advance the committed-height watermark. Returns only `retain_height` (pruning hint) — **not** the app hash (verified v1.0 `CommitResponse`). See §11. |

- **Verified against v1.0.1:** `FinalizeBlockResponse` carries
  `tx_results`, `validator_updates` ("validator set at height + 2"),
  `consensus_param_updates`, and **`app_hash`**; `CommitResponse` carries only
  `retain_height`. The full `Application` interface also includes
  `ExtendVote`/`VerifyVoteExtension` (vote extensions — not used in Phase 3;
  return empty) and the state-sync set
  `ListSnapshots`/`OfferSnapshot`/`LoadSnapshotChunk`/`ApplySnapshotChunk`
  (used by CometBFT state-sync; the app must implement them correctly so a
  syncing node can reconstruct state — see §11.7 and acceptance L5). Sources:
  `abci/types/application.go` and `proto/cometbft/abci/v1/types.proto` @ v1.0.1.
- **`Info` drives restart recovery:** it returns `LastBlockHeight` and
  `LastBlockAppHash` from Mongo's watermark; CometBFT's handshake compares these
  to its own store to decide replay (§11.3). This is the hinge of crash
  recovery and must report **durable** values only.

- **Mempool:** Phase 3 uses CometBFT's **default mempool** (CList + gossip)
  with `CheckTx` doing validation. The **app-mempool** (Krakatoa) model —
  where the app owns tx storage/ordering via `InsertTx`/`ReapTxs` — is a
  documented future option, not required for Phase 3.

### 4.4 Networking: CometBFT P2P replaces the WS mesh (requirement 1)

- CometBFT runs its **own P2P** (encrypted secret-connection, peer exchange,
  block-sync/state-sync). This becomes the inter-node transport.
- The existing WebSocket layer (`ws/`) — static full-mesh, challenge/response
  auth, `inventory`/`getblocks` sync — is **removed from the data path**. Its
  ordering role is fully subsumed by CometBFT; retaining it would be a
  "competing independent chain-selection mechanism," which is forbidden by
  requirement 1.
- **Client-facing API stays on gRPC.** CometBFT's RPC (`127.0.0.1:26657`) is
  used **internally** by the gate (tx broadcast, block queries) and is bound to
  localhost only. BLVChain clients never see CometBFT RPC. The ABCI proxy
  (`127.0.0.1:26658`) is loopback only. Only the **gRPC gate** faces clients.
- Consensus identity is **ed25519** (validator + node keys, §4.5), not the
  retired P-256 `dns_seed.json` (which survives only to verify legacy-record
  node signatures during migration, §5.4/§12).

### 4.5 P2P identity and authorization (requirement 3)

**Authenticated ≠ authorized.** CometBFT's p2p transport authenticates a
*node*, not a *validator*. The two are different keys with different jobs, and
neither implies the other. Verified against v1.0.1 (`p2p/key.go`,
`privval/file.go`, `p2p/conn/secret_connection.go`).

#### 4.5.1 Two keys, distinct purposes (both ed25519)

| Key | File | Purpose | Identity |
|---|---|---|---|
| **Node key** | `config/node_key.json` | Authenticates this node on the p2p transport (secret connection). Signs nothing in consensus. | **Node ID** = `hex(pubkey.Address())` |
| **Validator (consensus) key** | `config/priv_validator_key.json` | Signs consensus votes (precommit). Holds voting power. | Validator address in the validator set |

- The **secret connection** verifies the remote's **node key** (requires ed25519;
  X25519 ECDH + ChaCha20-Poly1305). This authenticates *who you are talking to*,
  **not** whether they have voting power.
- **Voting power** comes **only** from the validator set (genesis or future ABCI
  updates), proven by **signed votes** verified against validator keys. An
  authenticated peer with no validator entry has **zero** consensus weight.
- A peer can be fully authenticated and still be a non-validator; a validator's
  votes are checked against validator keys regardless of which node relayed them.

#### 4.5.2 Who may connect — permissioned deployment posture

Phase 3 runs a **permissioned** validator set and a controlled full-node set.
Although CometBFT p2p is permissionless by default, BLVChain configures it to be
effectively closed:

- **Persistent peers:** `persistent_peers` lists every known validator/full-node
  (node IDs + addresses). Validators keep persistent links to each other.
- **Private peers:** `private_peer_ids` lists validator/full-node node IDs so
  their addresses are **not gossiped** to others (reduces discovery surface).
- **Strict address book:** `addr_book_strict = true` (default) — only routable
  addresses in the book are dialed; combined with a curated book this keeps the
  peer set known.
- **Peer exchange:** `pex` is **disabled** (or `seeds` limited to a trusted set)
  so the network does not freely discover and admit unknown peers.
- **Admission hook:** CometBFT's `FilterPeers`/node filters are used to
  **reject connections whose node ID is not on BLVChain's allowlist** at connect
  time (defense-in-depth on top of the firewall). The ABCI app's peer filter
  returns reject for unknown node IDs.
- **Validators-only for Phase 3 (approved, decision 6).** The initial Phase 3
  deployment admits **only the 4 validators** to the p2p network. **Read-only
  full nodes and all other non-validator roles are deferred to a later phase.**
  (The architecture already supports them — a node with a node key but no
  validator key syncs via CometBFT block/state-sync and serves `ReadData`/
  `GetReceipt` with zero voting power — but they are not enabled in Phase 3 and
  are not part of the Phase 3 threat/failure model.)

#### 4.5.3 Node-key provisioning and rotation

- Each node's **node key** is generated at setup (`node_key.json`, ed25519). The
  node ID (hex of its pubkey) is what appears in `persistent_peers`/
  `private_peer_ids`/the allowlist.
- **Rotation:** replace `node_key.json` and update every peer's
  `persistent_peers`/`private_peer_ids`/allowlist to the new node ID, then
  restart. Rotation changes **only** the p2p identity; it does **not** touch the
  validator key or voting power.
- **Validator-key rotation** is a separate, consensus-affecting operation handled
  only via the exceptional validator-set procedure (§10.5) — **never** by editing
  `priv_validator_key.json` under a running set (that would equivocate). Node-key
  and validator-key rotation are therefore independent (acceptance P2).

#### 4.5.4 Sybil / unauthorized-peer / DoS protection

- **Authentication is not authorization:** an attacker cannot gain influence by
  authenticating with a self-generated node key — it is not a validator and not
  on the allowlist.
- **Connection caps:** `max_num_inbound_peers` / `max_num_outbound_peers` bound
  total connections; `allow_duplicate_ip = false` and per-IP limits blunt
  connection-flood/Sybil attempts.
- **Firewall / exposure (normative assumptions):** the p2p port (`26656`) is
  reachable **only** by known validator/full-node IPs (allowlist at the firewall
  / security group); the RPC (`26657`) and ABCI (`26658`) ports are bound to
  loopback and **never** exposed. The **only** internet-facing port is the gRPC
  gate. Operators MUST NOT publish `26656`/`26657` to `0.0.0.0` on public
  interfaces without the allowlist in place.
- **Unauthenticated flooding** of the gate is separately bounded by the gRPC
  rate limiter (§7.6, with the pre-auth-key fix in §15.6).

#### 4.5.5 Residual

- CometBFT's p2p is not a full BFT-DoS-proof boundary by itself; the
  defense-in-depth is **firewall allowlist + private_peer_ids + FilterPeers
  allowlist + connection caps**. Validator **misbehavior** (equivocation) at the
  consensus layer is handled by CometBFT evidence (§10.4/§13), which is a
  different axis from p2p admission.

---

## 5. Target architecture overview

### 5.1 Component view

```
                    CLIENT (unchanged: P-256 key, API key)
                       │  gRPC  (gate)
                       ▼
        ┌──────────────────────────────────────────────┐
        │  gRPC Gate (protos/)                           │
        │   SubmitTx → tx bytes, mempool broadcast         │
        │   GetReceipt(txID) → status + finality proof     │
        │   ReadData / AddData (compat)                    │
        │   rate limit, API-key auth (unchanged)           │
        └───────────────┬────────────────────────────────┘
                        │  (in-process, same OS process)
                        ▼
        ┌──────────────────────────────────────────────┐
        │  In-process CometBFT node (nm.NewNode)          │
        │   consensus + mempool + P2P + block store       │
        │   ed25519 validator key (this node)             │
        └───────────────┬────────────────────────────────┘
                        │  ABCI 2.0 (local client)
                        ▼
        ┌──────────────────────────────────────────────┐
        │  ABCI App (consensus/app)                        │
        │   CheckTx / ProcessProposal / FinalizeBlock /    │
        │   Commit / Info / InitChain                      │
        │   deterministic; contracts = storage only        │
        └───────────────┬────────────────────────────────┘
                        ▼
        ┌──────────────────────────────────────────────┐
        │  Persistence (db/) → MongoDB                     │
        │   Block collection (records) + legacy collection │
        │   height watermark, sender-tails, receipts       │
        └──────────────────────────────────────────────┘
```

### 5.2 Node startup sequence (replaces `main.go` bootstrap)

1. Load config; **remove** the WS mesh bootstrap and the inventory-sync gate.
2. Connect to MongoDB; open the `Block` collection(s); create indexes.
3. Construct the ABCI app with its dependencies (db handles, params, logger).
4. Load this node's **ed25519** consensus key (CometBFT `FilePV`) and node key.
5. Construct and `Start()` the in-process CometBFT node.
6. CometBFT's **handshake** (`Info`) compares the app's reported
   `LastBlockHeight`/`LastBlockAppHash` (from Mongo's durable watermark) to its
   store and decides replay per §11.3/§11.4: if the block store is ahead of Mongo
   the missing heights are re-applied via `FinalizeBlock`+`Commit`; an AppHash
   mismatch panics (safe halt); an app ahead of the store refuses. This completes
   **before** the gate opens.
7. Start the gRPC gate (submit/receipt/read). The gate opens for reads only after
   reconciliation completes and the node's locally computed AppHash matches the
   authoritative committed AppHash (§9.2) — otherwise it comes up **isolated**
   (not serving trusted reads).

### 5.3 What is removed

- `ws/` package in its entirety (mesh server, client, auth, inventory sync,
  `AddBlockToServers`). Block propagation is now CometBFT consensus.
- The "first-time sync must succeed before gRPC starts" gate.
- `BlockMeta.NodeSignature` production/verification for new records (§3.3).
- The BVM execution surface (`RunBVM`, `RunBVMWithOptions`, host functions,
  `SMART_CONTRACT_FUNCTION_NAME`, `EXECUTION_TIMEOUT`, `MAX_MEMORY_PAGES`) and
  the unused `wazero` + `participle` dependencies. These are already dead code
  (no non-test callers). Contract *upload + checksum storage* is retained (§6.4).

### 5.4 Retired trust anchors

- `config/dns_seed.json` (P-256 peer keys / WS mesh membership) is retired as a
  networking construct. Its only remaining *transitional* use is verifying the
  node signature on **legacy** records during migration (§12); after the legacy
  corpus is sealed, it can be dropped entirely.
- The validator set now lives in **CometBFT `genesis.json`** (ed25519 keys +
  voting power).

---

## 6. Data model

### 6.1 The record (unchanged content type, extended envelope)

The existing `db.Block` is the **record** type and its content-addressed
identity is preserved. Phase 3 adds a *consensus envelope* that is **excluded**
from the Record ID (exactly as `NodeSignature` is today), plus two fields that
are *included* in the ID and are now **app-assigned deterministically** rather
than by a gossip node.

Existing `Block` / `BlockMeta` / `BlockData` / `Contract` are unchanged. Added:

```go
// NEW — consensus envelope, EXCLUDED from BlockHashMaker (not part of Record ID)
type ConsensusMeta struct {
    ChainID     string `bson:"chainId"`     // CometBFT chain-id (replay-protection scope, §3.4/§10.5)
    Height      int64  `bson:"height"`      // CometBFT block height (authoritative global order)
    TxIndex     int32  `bson:"txIndex"`     // index within the CometBFT block
    CometBlock  string `bson:"cometBlock"`  // CometBFT block hash (finality identity)
    CommittedAt int64  `bson:"committedAt"` // CometBFT block time (ms)
}

// Added to db.Block (bson-optional, excluded from the ID hash):
//   Consensus ConsensusMeta `bson:"consensus,omitempty"`
//   Era       string        `bson:"era"`   // "consensus" (Phase 3) | "legacy" (migrated)
```

Fields in the ID that change *provenance* (not meaning):
- `BlockMeta.PreBlockHash` — now assigned by the app from committed state (§8).
- `BlockMeta.NodeUID` — now the **proposer identity for this height** (§8.2).
- `BlockMeta.TimeStamp` — now the **CometBFT block time** for this height,
  identical for every record in the block and across all validators (§9).
- `BlockData.TimeStamp` — **unchanged**: still the client-supplied timestamp,
  still inside the client signature and the ID.

Because `BlockHashMaker`'s input order and encoding are untouched, and the
changed fields carry the *same kinds* of values (a 64-hex predecessor, a UID
label, a millisecond timestamp), the Record ID scheme is preserved — only the
*source* of those values moves from "the gossiping node" to "deterministic
consensus execution."

### 6.2 Collections (MongoDB)

- **`Block`** — Phase 3 records (`Era: "consensus"`) and, after migration,
  sealed legacy records (`Era: "legacy"`, `Consensus.Height = 0`).
- **`_consensus_state`** (new, singleton-ish docs) — protocol bookkeeping:
  - `{ _id: "height", h: <last committed height> }` — the committed-height
    watermark (§11).
  - `{ _id: "sender_tail", senderUid: ..., recordId: ... }` — per-sender tail
    pointer (the last committed Record ID per sender), the authoritative source
    for `PreBlockHash` assignment (§8). Equivalent to a materialized index of
    the latest record per sender.
  - `{ _id: "receipt", txId: ..., ... }` — tx → receipt index (§7), or receipts
    may be derived on demand from `Block.Consensus` + the CometBFT store. See
    §7.3 for the recommended choice.
- **Indexes:** keep the existing timestamp/sender/receiver indexes for
  `ReadData`; add `{ "consensus.height": 1, "consensus.txIndex": 1 }` (global
  order) and a unique `{ "blockData.signature": 1 }` partial index (or
  `{senderUid, signature}`) to make dedup enforceable rather than check-then-
  insert (§7.6).

### 6.3 Global ordering is now explicit

The authoritative order is `(Consensus.Height, Consensus.TxIndex)`. All
consensus-ordered reads use this. `BlockMeta.TimeStamp` / `BlockData.TimeStamp`
remain as **secondary, presentation** sorts and client-time filters only; they
no longer define any ordering authority. This directly satisfies the removal of
timestamp-as-order that the pre-Phase-3 system relied on.

### 6.4 Contracts (storage + structural validation only)

- `db.Contract` and the `UseContract` reference stay **exactly** as they are,
  because both are inside `MessageMaker` and `BlockHashMaker`; changing them
  would break every Record ID and client signature.
- Contract **upload** (`storeContract`) remains: base64-decode, enforce
  `MAX_WASM_SIZE`, verify `SHA256(bytes) == ContractData.Checksum`, store at
  `SMART_CONTRACT_DIR/<checksum>.wasm`. This is **file storage**, not execution.
- **No WASM is ever instantiated, imported, or called** during `CheckTx`,
  `ProcessProposal`, `FinalizeBlock`, or `Commit`. The BVM package and the
  `wazero`/`participle` dependencies are removed (§5.3).
- **Determinism note:** because contract handling is pure byte storage keyed by
  content hash, it is trivially deterministic and cannot diverge across
  validators. The upload side effect (writing a `.wasm` file) is local and
  non-consensus; the *consensus* artifact is only the checksum already inside
  the signed Record, which any validator can re-verify against the stored bytes
  if/when a future execution phase reads them.

---

## 7. Transaction lifecycle (requirement 1.2, requirement 2)

### 7.1 The happy path

```
Client                Gate (gRPC)            CometBFT mempool      Consensus        ABCI App / Mongo
  │  SubmitTx(BlockData) │                        │                  │                    │
  ├─────────────────────►│                        │                  │                    │
  │                      │ 1. API-key auth       │                  │                    │
  │                      │ 2. shape validation   │                  │                    │
  │                      │ 3. CheckTx-equivalent  │                  │                    │
  │                      │ 4. build tx bytes ────►│  (broadcast)     │                    │
  │  ◄── TxID, ACCEPTED ─┤  (NOT finalized)      │                  │                    │
  │        (pending)     │                        │  propose block   │                    │
  │                      │                        ├─────────────────►│  ProcessProposal   │
  │                      │                        │                  ├───────────────────►│ (validate)
  │                      │                        │                  │  FinalizeBlock ───►│ (execute, AppHash)
  │                      │                        │                  │  Commit ──────────►│ (persist, watermark)
  │  GetReceipt(TxID)    │                        │                  │                    │
  ├─────────────────────►│  look up status ───────────────────────────  COMMITTED + proof  │
  ◄── COMMITTED, height, blockHash, recordId, index ─┤                  │                    │
```

`SubmitTx` returns as soon as the tx is **accepted into the mempool**. The
client learns finality only by polling `GetReceipt` (or via the synchronous
`AddData` wrapper, §7.7).

### 7.2 Transaction states

| State | Meaning | Terminal? |
|---|---|---|
| `UNKNOWN` | No record of this TxID (too new, wrong node, or never submitted). | no |
| `CHECKTX_FAILED` | Rejected at admission (bad shape, bad signature, over size). | yes (negative) |
| `PENDING` | Accepted into the mempool; not yet in a committed block. | no |
| `PROPOSED` | Included in a block being decided (optional observability state). | no |
| `COMMITTED` | Included in a CometBFT block committed by > 2/3 and persisted by `Commit`. **Only this state is "finalized."** | yes (positive) |
| `DROPPED` | Evicted from the mempool (expired, evicted under mempool pressure) before inclusion. Resubmittable. | yes (negative, soft) |

A tx in `PENDING`/`PROPOSED` may still become `DROPPED` or `COMMITTED`. Only
`COMMITTED` is finality.

### 7.3 Receipts — ordinary API receipt vs. verifiable finality proof (requirement 2)

There are **two different things** a "receipt" could be, and conflating them is
unsafe. Phase 3 implements only (a). It MUST NOT label (a) as (b).

#### (a) Ordinary API receipt — Phase 3 (implemented)

A **trusted-node assertion**: a receipt says "the node you asked reports this tx
was committed at height H." It is correct only insofar as you trust that node.
It is **not** independently verifiable by a third party and MUST be documented as
such.

New RPC (gRPC, same gate):

```proto
rpc GetReceipt(GetReceiptRequest) returns (GetReceiptResult);
message GetReceiptRequest  { string TxId = 1; }      // canonical TxID (§3.4) or RecordId
message GetReceiptResult {
  enum Status { UNKNOWN=0; PENDING=1; COMMITTED=2; CHECKTX_FAILED=3; DROPPED=4; }
  Status Status        = 1;
  string Log           = 2;
  bool   Verifiable    = 3;   // ALWAYS false in Phase 3 (reserved for proof bundles)
  int64  Height        = 4;   // set iff COMMITTED
  string CometBlock    = 5;   // CometBFT block hash iff COMMITTED
  int32  TxIndex       = 6;
  string RecordId      = 7;   // BLVChain Record ID iff COMMITTED
  string ChainId       = 8;
  string AssertingNode= 9;   // node id that answered (for audit)
}
```

Receipt source of truth: a **persistent `txId → (height, index)` index** written
during `Commit` (adopted default, §15). The gate answers `GetReceipt` from this
durable index (not from CometBFT's prunable RPC). It returns `COMMITTED` **only**
when the watermark (§11) is `≥ Height` **and** the record is durably in Mongo.
Otherwise it returns the honest non-final status. A receipt is **never**
synthesized ahead of durable commit. `Verifiable` is always `false` in Phase 3.

#### (b) Cryptographically verifiable finality proof — SPECIFIED, not implemented

For a party that does **not** trust the queried node, a *verifiable* proof lets
it confirm finality itself against a known validator set (light-client style).
Phase 3 defines the bundle and reserves the fields but does **not** populate
them; this is a Phase 4+ deliverable. The required components of a future proof
for tx at height `H`:

1. **CometBFT header for `H`** — the signed block header: `height`, `chain_id`,
   `time`, `app_hash`, `data_hash` (tx Merkle root), `last_commit_hash`,
   `validators_hash`, `next_validators_hash`, `proposer_address`.
2. **Commit signatures for `H`** — the +2/3 precommit signatures over block `H`
   (in CometBFT the commit that finalized `H` is carried as `LastCommit` in the
   header of `H+1`, and is also retrievable via the commit RPC). The verifier
   sums the voting power of the valid signatures and checks it is `> 2/3` of the
   validator set's total power.
3. **Validator set at `H` and the set-transition chain** — the verifier must know
   which keys had voting power at `H`. Either it is given a **weak-subjectivity
   checkpoint** (a trusted header at a recent height with its `validators_hash`),
   or it follows the `validators_hash → next_validators_hash` linkage block-by-
   block back to a trusted root (genesis or checkpoint).
4. **Transaction inclusion proof** — a Merkle proof that the tx's leaf hash is in
   the block's `data_hash` (tx Merkle root), together with the tx bytes. (Tx
   results/events have their own Merkle root if those are proven too.)
5. **Trusted starting point + verification procedure** — begin at a trusted
   header `H0` with a known validator set (genesis or weak-subjectivity
   checkpoint) → verify the commit chain `H0..H` (each header's signatures valid
   under the previous set; `next_validators_hash` continuity) → verify the commit
   for `H` carries `> 2/3` → verify the tx inclusion proof against `H`'s
   `data_hash` → conclude final. No step relies on trusting the serving node.

**Phase 3 rule:** receipts returned by `GetReceipt` are ordinary receipts
(`Verifiable=false`). They MUST NOT be described in client-facing docs as
cryptographic finality proofs, and the proof fields above remain reserved.

### 7.4 What "finalized" means, precisely

A record is finalized iff:
1. its tx is in a CometBFT block at height `H`, and
2. that block has been committed by > 2/3 voting power (CometBFT consensus
   guarantees this before `FinalizeBlock`), and
3. the ABCI `Commit` for `H` has durably persisted the record to Mongo and the
   watermark is `≥ H`.

Condition 3 is the only one BLVChain controls, and §11 guarantees it is atomic
and recoverable. Until all three hold, `GetReceipt` MUST NOT report `COMMITTED`.

### 7.5 Determinism of the acceptance path

`CheckTx` must be deterministic in its *accept/reject* verdict across
validators (it is executed by all nodes independently). It may use wall-clock
skew for the client timestamp sanity bound, but the skew check must be
**advisory at admission only** and re-checked identically in `ProcessProposal`
against the **block time** so all validators agree. Non-deterministic inputs
(local clock beyond the documented skew window, local map iteration) MUST NOT
affect inclusion.

### 7.6 Duplicates, retries, timeouts, disconnects

- **Duplicate submission (same bytes ⇒ same TxID):**
  - Mempool dedups by TxID (CometBFT never includes the same tx twice).
  - `CheckTx` rejects a tx whose `(senderUid, signature)` is already committed
    (state lookup) — returns a non-fatal "already committed" code.
  - `SubmitTx` is **idempotent**: if the TxID is already known, it returns the
    existing TxID immediately (same as today's duplicate-returns-existing-
    BlockHash behavior). The client cannot create two records from one signed
    payload because the Record ID is content-addressed and the DB upsert is
    keyed on it (and a unique index enforces it).
- **Retries:** safe by construction — resubmitting the identical signed
  `BlockData` is a no-op that converges to the same TxID/Record ID.
  Resubmitting after `DROPPED` re-admits the same tx.
- **Timeouts:** if `GetReceipt` stays `PENDING`/`DROPPED` past a client-chosen
  horizon, the client MAY resubmit (idempotent) or treat as not-yet-final. The
  gate does not invent a timeout-final status.
- **Client disconnects:** submission is decoupled from the connection. Once
  `SubmitTx` returns a TxID the tx is owned by the mempool; disconnecting does
  not cancel it. (This is strictly better than today, where `AddData` blocks on
  peer fan-out and ties latency to the connection.)
- **Rate limiting:** the existing per-key gRPC limiter stays, but must be
  applied to `SubmitTx`/`GetReceipt`; `GetReceipt` should be cheap and lightly
  limited. Fix the pre-auth bucket issue noted in §14 (limit after identity is
  known, or key on connection IP).

### 7.7 Backward compatibility and breaking changes

- **New async API:** `SubmitTx` + `GetReceipt` (the approved model).
- **Compatibility `AddData`:** retained as a **synchronous wrapper** around
  `SubmitTx` that submits the tx then polls `GetReceipt` until `COMMITTED` (or
  until a **configurable timeout** elapses), returning the historical
  `AddDataResult{IsSuccess, Log, BlockHash}` where `BlockHash` is the Record ID.
  This preserves the old *signature* while strengthening semantics: "returned
  `IsSuccess=true` ⇒ finalized." `SubmitTx` remains the **preferred** API;
  `AddData` exists only for legacy clients.
  - **Configurable timeout:** `ADD_DATA_FINALITY_TIMEOUT` (per-call, with a
    server-side cap). On expiry the wrapper returns
    `AddDataResult{IsSuccess:false, Log:"timeout: outcome unknown — query
    GetReceipt with the returned TxId"}`. The `TxId` is always returned so the
    client can resolve the true outcome.
  - **A timeout is NEVER proof of failure.** It means only "not finalized within
    the deadline." The tx may still commit later. The wrapper MUST NOT report a
    timeout as a definitive failure, and MUST NOT suggest resubmission as the
    first remedy (resubmission is safe/idempotent, but the correct first step is
    `GetReceipt`). This is a normative correctness rule (acceptance D6).
- **Documented breaking changes:**
  1. `AddData` latency increases (it now waits for consensus finality, not a
     local insert + best-effort gossip). Clients needing low latency should
     migrate to `SubmitTx`.
  2. `BlockHash` in `AddDataResult` is the Record ID, which still exists and is
     content-addressed, but it is no longer "the thing peers gossiped" — peers
     never see it out of band; they get it via consensus.
  3. The WS protocol is gone; any client/peer tooling depending on it breaks.
  4. `ReadData` results gain `Consensus` fields; legacy blocks carry `Era:
     "legacy"`.

### 7.8 Can an accepted-but-uncommitted tx be lost after restart?

Yes, a `PENDING` (accepted-but-uncommitted) tx **can be lost** on restart, and
that is correct:
- Mempool contents are **not** part of consensus state and need not survive a
  restart. CometBFT may reload a mempool WAL (capped, best-effort); a tx still
  in `PENDING` has no finality guarantee.
- **Committed** txs are **never** lost: they are in CometBFT's block store
  (replicated across > 2/3 of validators) and in Mongo (watermark + records).
  If this node's Mongo is behind on restart, it **replays** from the block
  store until caught up (§11) — so a committed record is always reconstructable.
- Clients treat `PENDING` as "not yet final" and resubmit idempotently; a lost
  `PENDING` tx simply gets resubmitted. This is the defined contract.

---

## 8. Preserving the per-sender `PreBlockHash` chain inside global order (requirement 2)

### 8.1 The key insight

Today `PreBlockHash` = "the previous block by the same sender," assigned by the
node at gossip time and baked into the Record ID. Under CometBFT the global
order is authoritative, so the per-sender chain becomes a **derived view of
committed state**, assigned **deterministically at execution time** by the ABCI
app — not guessed at admission time. Because every validator executes the same
ordered tx sequence from the same committed state, they all derive the **same**
`PreBlockHash` for each record, so the Record ID is identical everywhere.

### 8.2 Assignment rule (in `FinalizeBlock`, per tx, in tx order)

The app maintains a **sender-tail map** `tail[senderUid] -> last committed
Record ID` (persisted in `_consensus_state`, initialized from genesis). For each
tx in the block, in `TxIndex` order:

1. **Sender continuity:**
   `PreBlockHash = tail[senderUid]` if present, else `FIRST_BLOCK_HASH`
   (genesis anchor, carried through migration, §12).
2. **NodeUID:** `NodeUID = proposer identity for this height` (see §8.4).
3. **BlockMeta.TimeStamp:** `= CometBFT block time` for this height (same for all
   txs in the block; see §9).
4. Compute the Record ID with the unchanged `BlockHashMaker`.
5. Stage the record write.
6. **Advance the tail:** `tail[senderUid] = this Record ID`.

Because steps run in `TxIndex` order and the tail is updated after each tx,
**two txs from the same sender in the same block chain correctly**: the second
links to the first. This is exactly the per-sender chain semantics, now with a
deterministic total order.

### 8.3 Handling the four cases

- **Sender continuity:** preserved by the tail map (above). A sender's records
  form a `PreBlockHash`-linked subchain even though they are interleaved with
  every other sender's records in the global order.
- **Duplicate records:** impossible to fork the sender chain. A duplicate tx has
  the same TxID and is deduped at the mempool/`CheckTx`; if it somehow reached
  `FinalizeBlock`, `tail[senderUid]` would equal the already-committed Record ID
  only for the *original* — the duplicate produces a byte-identical `BlockData`
  whose recomputed Record ID already exists, so the upsert is a no-op and the
  tail is not corrupted. The unique `(senderUid, signature)` index enforces
  this at the DB.
- **Out-of-order submissions:** CometBFT provides a single total order, so
  "out of order" only means "a later tx is in an earlier position than the
  sender submitted it." That is fine — `PreBlockHash` reflects *consensus*
  order, not submission order. If strict per-sender submission ordering is ever
  required, the app's `PrepareProposal`/`CheckTx` can enforce a per-sender nonce
  (future option); Phase 3 does not require it and simply records true order.
- **Invalid sender references:** **structurally impossible.** The client never
  supplies `PreBlockHash` (it is not in `MessageMaker`), so a client cannot
  reference a non-existent or attacker-chosen predecessor. The only values the
  app ever assigns are `FIRST_BLOCK_HASH` or an already-committed Record ID from
  its own tail map. There is no "unknown predecessor" failure mode, unlike the
  pre-Phase-3 gossip path (which had to defer/retry blocks with unresolved
  predecessors). This is a strict safety improvement.

### 8.4 What `NodeUID` means now

Today `NodeUID` is "the gossip node that handled the request." In Phase 3 it is
"the validator that **proposed** the CometBFT block at this height," derived
from the proposer address via a deterministic, documented mapping (e.g. a
registered label per validator in genesis `app_state`, keyed by ed25519
address). It is per-CometBFT-block, so all records in one block share the
proposer's `NodeUID`. This is deterministic and known to all validators in
advance (the proposer for height `H` is fixed by the round-robin/PBTS schedule),
so it does not threaten determinism. `NodeUID` remains a 64-hex-compatible label
as before; it is **no longer a key-derived or signed field** (its trust now comes
from CometBFT commit signatures, §3.3).

---

## 9. Determinism (requirement 6)

Every validator MUST compute the identical application state and the identical
AppHash for the same committed block sequence. Rules:

1. **No wall-clock in execution.** `BlockMeta.TimeStamp` and receipt timestamps
   come from the **CometBFT block time** (BFT Time), which is identical on all
   validators for a given height. Local `time.Now()` is forbidden in
   `FinalizeBlock`/`ProcessProposal`.
2. **No unordered iteration.** Any map iteration that affects state (the sender-
   tail map, dedup checks) is avoided or sorted by key. State transitions are a
   pure function of (committed state, ordered txs, block time, proposer).
3. **No randomness, no floating point, no environment reads** in execution.
4. **Idempotent, content-addressed writes.** Record IDs are content hashes, so
   re-execution and replay converge (§11).
5. **Contracts are inert bytes** (§6.4) — no execution nondeterminism.
6. **`PrepareProposal`/`ProcessProposal` are deterministic.** If `PrepareProposal`
   selects/reorders txs, the selection rule is deterministic given the mempool
   view; `ProcessProposal` only *validates* (accept/reject) and never mutates
   state, so validators agree on the verdict. (Phase 3 may defer custom proposal
   logic to CometBFT defaults; if so, this is moot.)

### 9.1 AppHash definition (normative)

The AppHash committed at height `H` is a deterministic commitment to the full
application state resulting from executing block `H` (§9.3 enumerates that
state). Define it as a rolling hash chain committing both the record log and the
sender-tail state:

```
AppHash[H] = SHA256(
    "blvchain/v2/apphash" ‖ 0x00 ‖
    EncodeFields(
        Int64ToStr(H),
        "blvchain-apphash-v1",             // app-hash scheme version (domain)
        AppHash[H-1],                       // previous AppHash (genesis: 32 zero bytes)
        RecordIDsRoot(block H),             // ordered Record IDs in this block, Merkleized (Appendix D)
        SenderTailsRoot(after block H)      // Merkle root of the updated sender-tail map (Appendix D)
    )
)
```

- `RecordIDsRoot` = Merkle root (Appendix D) of the 32-byte Record IDs in
  `(TxIndex)` order; empty block ⇒ the fixed empty-root constant.
- `SenderTailsRoot` = Merkle root over `sorted(senderUid → RecordId)` pairs after
  applying the block.
- `EncodeFields` length-prefixing keeps the preimage unambiguous; the
  `"blvchain/v2/apphash"` tag keeps an AppHash from ever colliding with a tx ID
  or Record ID. The Merkle rules are fixed in Appendix D and locked by golden
  vectors (Appendix C).

### 9.2 What CometBFT guarantees about the AppHash — and what it does NOT (requirement 1, corrected)

> **Correction of an earlier overclaim.** CometBFT does **not** gossip app
> hashes between validators to detect nondeterminism, and it has **no dedicated
> "app-hash divergence → halt with diagnosis" mechanism**. BLVChain therefore
> **must** implement its own divergence detection. Do not rely on CometBFT to
> tell you a validator diverged.

**What CometBFT guarantees (verified, v1.0.1):**
- `FinalizeBlock` returns an `app_hash`; CometBFT writes it into the **next**
  block's header (`header.AppHash` at height `H+1` = the app hash resulting from
  executing `H`).
- During block validation, each node runs a **local, per-node check**
  (`state/validation.go`, `validateBlock`):
  ```go
  if !bytes.Equal(block.AppHash, state.AppHash) {
      return fmt.Errorf("wrong Block.Header.AppHash. Expected %X, got %v. Check ABCI app for non-determinism", ...)
  }
  ```
  i.e. a node compares the incoming block's `header.AppHash` against **its own**
  locally stored `state.AppHash` (the value *it* computed by running its own
  app). On mismatch the node **rejects that block** and withholds its vote.
- On restart, the handshake cross-checks the app's reported `LastBlockAppHash`
  against CometBFT's stored state and **panics** on mismatch
  ("Did you reset CometBFT without resetting your application's data?"); during
  replay it cross-checks each block's produced app hash against the next block's
  `header.AppHash`.

**What it does NOT guarantee:**
- It does **not** compare app hashes **across different validators** — there is
  no cross-node app-hash agreement message. The check is purely local: "does the
  proposer's claimed prior-app-hash match *mine*?"
- The chain-level effect of nondeterminism is **indirect and lossy**: validators
  whose local AppHash diverges reject the next block and stop voting; if **≥ 1/3**
  of voting power diverges, no block reaches 2/3 and the chain **halts** — which
  looks like an ordinary liveness failure. It does **not** name the divergent
  node or the cause, and a **≤ 1/3** divergent minority can simply fall behind
  silently.
- Safety is preserved (a divergent minority cannot force a conflicting commit),
  but **detection, attribution, and isolation are BLVChain's responsibility.**

**How BLVChain detects divergence (normative):**
1. **Authoritative committed AppHash.** For every committed height `H`, the
   canonical `AppHash(H)` is the value in `header.AppHash` of the committed block
   `H+1`. BLVChain persists this per-height authoritative AppHash (from the
   committed header) alongside the height watermark.
2. **Per-node local AppHash exposure.** Each node exposes, via a **local admin /
   metrics endpoint (not the consensus RPC)**, its **locally computed**
   `LastBlockAppHash` and committed height.
3. **Reconciliation monitor.** A monitoring component polls all validators and,
   for each height, asserts: (a) every node's locally-computed `AppHash(H)`
   equals the authoritative committed `AppHash(H)` from the header; and (b) all
   nodes agree with each other. Any mismatch at any height ⇒ **divergence
   alarm**, attributed to the specific node(s). (Acceptance S2, S6, S7.)
4. **Self-check on commit.** In addition, each node itself compares the AppHash
   it just committed to the AppHash in the next committed header it observes; a
   local mismatch triggers self-isolation (below) immediately, not only via the
   monitor.

**How a divergent validator is isolated — automatic FAIL-CLOSED (approved):**
On detecting that its locally computed AppHash ≠ the authoritative committed
header AppHash, a node **automatically fails closed** and, with no operator in the
loop, immediately:
1. **Stops serving trusted reads** — the gate returns an explicit "node not in
   consensus / isolated" status for `ReadData`/`GetReceipt`; it never serves
   possibly-divergent data.
2. **Stops accepting new submissions** — `SubmitTx`/`AddData` are refused (the
   node will not add txs to a state it can no longer vouch for).
3. **Stops validator signing/voting** — the consensus key is fenced (the node
   stops prevoting/precommitting; CometBFT would reject its blocks anyway, but the
   app also refuses to participate), so it cannot influence the chain.
4. **Preserves forensic data** — it snapshots its CometBFT state + Mongo app state
   and the exact AppHash preimage at the divergence height, and writes a
   tamper-evident incident record, **before** any remediation.

**No automatic recovery.** The isolated node does **NOT** auto-resynchronize and
does **NOT** auto-rejoin. Recovery is an **explicit operator procedure** (§9.2
recovery steps): root-cause, fix, reprovision uniform binary/toolchain, wipe
local state, state-sync from honest peers, verify, then re-enable serving/voting.
The reconciliation monitor marks the node divergent and removes it from the
trusted read-serving pool (load balancers / client discovery stop routing to it);
**no other node trusts its data** once flagged. (Acceptance S6 asserts all four
fail-closed actions and the absence of auto-rejoin.)

**How divergence is investigated and recovered:**
1. **Freeze** the divergent node out of the serving pool (above).
2. **Capture forensics:** snapshot its CometBFT state + Mongo app state at the
   divergence height; record its app hash inputs.
3. **Diff** against a known-good node at the same height (records, sender-tails,
   and the exact AppHash preimage) to localize the nondeterminism source
   (typical culprits: Go map iteration order, wall-clock, float, `int` width, or
   a Go toolchain / dependency version skew between nodes).
4. **Fix** the nondeterminism (code/config), reprovision **all** nodes with the
   **same** binary/toolchain/dependency versions.
5. **Resync:** wipe the divergent node's app+consensus state and **state-sync
   from honest peers** (which verifies the AppHash/merkle), then rejoin. A
   state-synced node is known-good by construction.
6. **Post-incident:** add a regression test that would have caught the specific
   nondeterminism (negative AppHash test), and require uniform toolchain pins in
   CI.

**How AppHash consistency is tested after every committed height:**
- Acceptance **S2** runs a multi-node cluster and, **after every committed
  height**, asserts all validators' locally-computed AppHash equals the
  authoritative committed header AppHash (a per-height golden chain).
- Acceptance **S6** injects divergence on one node and asserts detection +
  isolation; **S7** mutates a validation-relevant field on one node and asserts
  the AppHash changes and divergence is caught (proving §9.3 completeness).

### 9.3 Complete application state committed by the AppHash (requirement 1)

The AppHash MUST commit to **everything that influences future transaction
validation or execution**. In Phase 3 that is exactly:

1. **Chain position and lineage:** `H`, the app-hash scheme version
   (`blvchain-apphash-v1`), and `AppHash[H-1]` — binds each height to its
   predecessor (a single altered historical state changes all later AppHashes).
2. **The committed record log:** `RecordIDsRoot(block H)` over the ordered Record
   IDs. Because **each Record ID is a SHA-256 over the record's full content**
   (incl. the client signature, `SenderUID`, `UseContract`, and
   `ContractData.Checksum` — §3, §6.1), committing the ordered Record IDs
   transitively commits the **entire record content**, and therefore also:
   - the **admission/dedup state** used by `CheckTx`/`ProcessProposal` (the set
     of committed `(senderUid, signature)` pairs is a pure function of the
     committed records), and
   - the **contract checksums** referenced by records (storage-only; no bytes
     are executed, so no execution state to commit — §6.4).
3. **Sender continuity state:** `SenderTailsRoot` over the sender-tail map,
   because the next record's `PreBlockHash` for each sender is read from it
   (§8). A divergent tail map changes future Record IDs and must be caught.
4. **Validation constants are NOT per-state values but a hard determinism
   requirement:** all app-level validation bounds (max data size, role ranges,
   skew window, etc.) are **compile-time constants identical across every
   validator**, and CometBFT consensus params (block/gas bounds) are committed by
   CometBFT itself. Therefore no separate per-state param root is needed in
   Phase 3 — **but** any future **on-chain** parameter (governance, dynamic
   limits, an on-chain validator set) that affects validation MUST be added as
   app state and folded into this AppHash before it is introduced.
5. **Genesis anchors:** `AppHash[0]` is fixed in `InitChain` from genesis
   `app_state` (seeded sender-tails + `FIRST_BLOCK_HASH`), so the genesis state
   is committed and identical on all nodes (§12).

> Anything **not** in this list MUST NOT influence validation/execution. If it
> does, it is a determinism bug (it would make the AppHash under-commit and let
> divergent state pass undetected). This is the invariant behind S7.

---

## 10. Validator-set management (requirement 1.3)

### 10.1 Phase 3: genesis-defined and IMMUTABLE after genesis

- The validator set is defined once in **CometBFT `genesis.json`**: 4 validators,
  each `{ ed25519 pubkey, power: equal }`, plus `app_state` carrying the
  `sender_tails` initial map, the legacy anchor `FIRST_BLOCK_HASH`, and the
  proposer-label map (§8.4).
- **The validator set is immutable for the entire life of a Phase 3 chain.**
  Adding, removing, or rotating validators, or changing voting power, is
  **unsupported during normal operation.** There is **no** in-protocol update and
  **no** supported "edit `genesis.json` and restart the existing chain" path
  (§10.6 explains why that is unsafe and is explicitly forbidden).
- `FinalizeBlock` returns **empty** `ValidatorUpdates` at every height; the set
  committed at genesis is the set forever (acceptance V1). This keeps the
  validators_hash / AppHash continuity intact and the BFT math (n = 4, f = 1)
  fixed and auditable.
- The **only** way to change the validator set in Phase 3 is the exceptional,
  **coordinated chain-migration** procedure in §10.5 — a planned, operator-driven
  event, not a routine operation.

> **Correction of an earlier draft.** An earlier draft described "stop all
> validators, edit `genesis.json`, restart from the same height." That is **not**
> a valid general-purpose validator-set change and MUST NOT be used: a chain's
> committed history is bound to its original validator set via the
> `validators_hash` chain, so swapping the set under an existing chain breaks
> continuity and forks/rejects rather than updating. See §10.6.

### 10.2 Designed for later on-chain updates (no redesign)

The ABCI integration reserves the **validator-update return path** from day one:

- `InitChain` returns `ValidatorUpdates` (the genesis set in Phase 3).
- `FinalizeBlock` returns `ValidatorUpdates`; in Phase 3 this is **always
  empty**, but the field is wired through to CometBFT, which already knows how
  to apply validator-set changes returned by the app.
- A future phase introduces a **governance/proposer-authorized state transition**
  (a special record type or a privileged tx) whose deterministic execution
  returns a non-empty `ValidatorUpdates` (add/remove/change power/rotate key).
  Because the return path already exists, this requires **new transition logic,
  not a new consensus integration**.

### 10.3 Documented operations (to be enabled in the on-chain phase)

- **Validator addition:** a new ed25519 key with `power > 0` enters the set via
  a `ValidatorUpdates` entry; it must satisfy the n ≥ 3f + 1 re-check for the
  new `n`. Pending-sync nodes catch up via CometBFT block/state sync.
- **Validator removal:** a `ValidatorUpdates` entry with `power: 0`. The set
  size must remain ≥ the BFT minimum after removal or the transition is
  rejected deterministically (all validators reject identically).
- **Key rotation:** a validator submits a signed transition replacing its
  ed25519 key; enforced to be authorized by the *outgoing* key, applied at a
  fixed height so all nodes switch atomically. The P-256 client identity is
  independent and unaffected.
- **Voting-power change:** a `ValidatorUpdates` power change, subject to the
  same BFT re-check. Equal power is the Phase 3 invariant; any change is a
  later, explicitly-governed decision.

### 10.4 Documented BFT assumption (normative)

- Safety holds if **at most f = 1** of the 4 validators is Byzantine
  (double-signing, equivocation). CometBFT detects equivocation and records
  evidence; with no slashing module in Phase 3, evidence is logged and surfaced
  in receipts/monitoring, not financially penalized.
- Liveness holds if **at least 3** validators are correct and connected.
- **A 2-2 network partition halts progress** (no quorum) but **preserves
  safety** (no two conflicting blocks can both commit). See §13/§14.

### 10.5 Exceptional validator replacement — a planned chain migration (the only supported way in Phase 3)

When a validator must genuinely be replaced (key compromise, operator
decommission), Phase 3 uses a **coordinated chain migration**, not an in-place
edit. This is an exceptional, scheduled operation with an operator runbook and a
dry run in staging (acceptance V2):

1. **Announce & halt.** Schedule downtime; let the chain commit a final height
   `H`, then stop all validators (and full nodes) cleanly.
2. **Export state (not replay).** From a trusted node, deterministically export
   the application state at `H`: the committed record set, the sender-tail map,
   the height watermark `H`, and `AppHash(H)`. This is a **state export**, not a
   re-execution of old blocks (which would re-impose the old validator set).
3. **Build a new genesis.** Create a **new `genesis.json` with a new chain-id**
   and the **replacement validator set** (ed25519 keys, equal power; re-check
   `n ≥ 3f + 1` for the new `n`). Its `app_state` is the exported state (seeded
   sender-tails, `FIRST_BLOCK_HASH`, and the carried record set as the initial
   committed state at `H`).
4. **Verify & start.** All nodes verify the new genesis hash matches the signed
   plan (guard), then start the **new chain** from the new genesis; the chain
   resumes at `H+1` with the new validator set.
5. **Continuity & records.** Receipts/records keep their content IDs; the chain-id
   changes, so a receipt records both the historical chain-id/height and the new
   one. Clients and monitoring are updated to the new chain-id. The migration is
   explicitly labeled; nothing silently rewrites history.

> This is the same state-export machinery as the §12 legacy cutover. It is a
> **new chain with a new identity**, chosen deliberately so validator-set changes
> never masquerade as in-place edits of the running chain.
>
> **Replay protection (approved, decision 3):** because the chain ID is part of
> the canonical tx / TxID domain (§3.4) and is stamped into each record's
> `ConsensusMeta.ChainID` (§6.1), a transaction signed for the old chain cannot be
> replayed into the new chain's mempool/receipts under the same TxID, and the
> per-chain `(senderUid, signature)` dedup (seeded with the migrated state)
> rejects any attempted cross-chain replay of the same record.

### 10.6 Why "edit genesis.json and restart from the old DB" is forbidden

Changing the validator set is **not** a configuration overlay on an existing
chain:
- CometBFT binds each block to its validators: the header carries
  `validators_hash`/`next_validators_hash`, and block `H+1`'s `LastCommit` is a
  set of precommits signed by **`H`'s** validators. A chain's committed history
  is therefore cryptographically tied to the validator set that produced it.
- If you alter `genesis.json` (or the validator keys) but restart a node against
  the **existing** block store / DB at height `H`, the new set will not match the
  `validators_hash` committed in that history. The node will either **reject its
  own history** (handshake AppHash/validators mismatch → the documented panic) or,
  if only some operators adopt the change, **fork** the fleet. Neither is safe nor
  deterministic across nodes.
- Therefore the genesis file defines a chain **from its start**; it cannot change
  the validator set of an already-committed chain. Validator-set changes in
  Phase 3 are **only** the §10.5 chain migration, and on-chain updates are a
  **future phase** (§10.2–10.3).

---

## 11. Crash recovery and atomicity (requirement 5)

**Invariant:** a record committed by CometBFT (> 2/3 signed the block) is **never
permanently absent** from application state. At worst it is temporarily behind
and is reconstructed deterministically on restart.

### 11.1 The verified commit ordering (the fact the design is built on)

CometBFT v1.0.1 `finalizeCommit` (`internal/consensus/state.go`) then
`applyBlock` (`state/execution.go`) orders the durable events like this
(verified against the v1.0.1 source):

```
ValidateBlock(H)
→ blockStore.SaveBlock(H)            // block is on disk FIRST
→ WAL: EndHeightMessage (fsync)
→ app.FinalizeBlock(H)               // returns app_hash (NOT persisted by CometBFT yet)
→ SaveFinalizeBlockResponse          // ABCI response saved (for possible mock replay)
→ updateState (in-memory)
→ app.Commit(H)                      // the app persists to Mongo HERE
→ stateStore.Save(state, AppHash)    // consensus state + AppHash persisted AFTER Commit
→ fireEvents
```

Two facts follow directly and the whole design relies on them:
1. **The block is in CometBFT's block store BEFORE the app's `Commit` runs.**
   Therefore `blockStoreHeight >= app-watermark` **always**. If the app ever
   reports a watermark above the block store, CometBFT refuses to start with
   `ErrAppBlockHeightTooHigh` (a safe halt; that situation indicates tampering).
2. **CometBFT persists its consensus state (incl. the AppHash) only AFTER
   `app.Commit` returns.** So a real window exists ("`Commit` done, consensus
   state not yet saved") that the handshake is specifically built to recover.

The block store is therefore the durable consensus record; Mongo is a
**deterministic materialized view** of committed blocks, always rebuildable from
the block store by replay.

### 11.2 `FinalizeBlock` (compute) vs `Commit` (persist), and why `Commit` must be atomic

- `FinalizeBlock` computes new state + the `app_hash` **in memory / staged** and
  returns it. It MUST NOT make state durably visible.
- `Commit` is the single point that makes height `H` visible, and MUST be
  **atomic**: in **one MongoDB transaction** (replica set required, §15.1) it
  writes all of `H`'s records, the `txId` to `(height,index)` receipt entries,
  the updated `sender_tail` docs, the per-height authoritative AppHash, and
  **finally** the `height` watermark = `H`. The watermark is written **last, in
  the same transaction**, as the commit marker; if the transaction fails, none of
  `H` is visible and the watermark stays `H-1`.
- Because `Commit` is atomic and the watermark is the last write, the app's
  reported height (`Info`) is **always truthful** — this is what lets CometBFT's
  handshake make the right replay decision in every crash window (§11.4).
- Every write is **idempotent** (content-addressed `_id`, `txId`-keyed receipts,
  deterministic sender-tail values), so re-running `Commit(H)` converges to the
  same state. The transaction is the primary guarantee; idempotency is the net.

### 11.3 `Info` / watermark is the recovery anchor; handshake behavior (verified)

On startup the ABCI app's `Info` returns `LastBlockHeight = watermark` (the last
durably committed height in Mongo) and `LastBlockAppHash = AppHash[watermark]`.
CometBFT's handshake (`internal/consensus/replay.go`) compares these to its store
and behaves per v1.0.1:

- **app height == 0** (fresh): `InitChain` runs; genesis state/validators saved.
- **app ahead of store** (`storeH < appH`): CometBFT returns
  `ErrAppBlockHeightTooHigh` and refuses ("the app should never be ahead of the
  store"). Cannot happen given §11.1 fact 1; a safe guard against tampering.
- **app behind store** (`appH < storeH`): CometBFT **replays** every missing
  height by **re-invoking `FinalizeBlock` AND `Commit`** for each (via
  `ExecCommitBlock`). This re-materializes Mongo from the block store.
- **app == store, AppHash matches:** clean start, no replay.
- **app == store, AppHash MISMATCH:** CometBFT **panics** ("state.AppHash does
  not match… Did you reset CometBFT without resetting your application's data?").
  This is the DB-tampered / reset-without-reset signal: a safe halt.
- **store == state + 1** (block saved, consensus state not yet updated — exactly
  the "`Commit` done, state save pending" window): if the app is **behind**
  (`appH == stateH`) CometBFT **replays the last block with the real app**
  (re-runs FinalizeBlock+Commit); if the app is **already at** `appH == storeH` it
  **replays with a mock app** built from the saved `FinalizeBlockResponse`,
  advancing CometBFT's state to `H` **without** re-invoking our app (no
  double-apply).

During replay, each block's produced app hash is cross-checked against the next
block's `header.AppHash`; a mismatch panics. The gate opens for reads only after
reconciliation completes (§5.2).

### 11.4 The exact crash-recovery state machine (requirement 5)

Each row is a crash point in the §11.1 pipeline for a single height `H` (a block
already decided by consensus, so final); the only question is whether the app has
materialized it. "Block store" = CometBFT block store; "cons. state" = CometBFT
consensus state store (AppHash); "Mongo" = app watermark + records. **Re-run**
= CometBFT re-invokes the app for `H` on restart; **mock** = CometBFT advances
itself from the saved `FinalizeBlockResponse` without re-invoking our app.

| Crash point (for height `H`) | Block store | Cons. state | Mongo | Re-run `H`? | Recovery height from `Info` | AppHash restored/verified by | Duplicate prevention |
|---|---|---|---|---|---|---|---|
| **C0** before `H` decided | `H-1` | `H-1` | `H-1` | n/a (consensus re-decides `H` from WAL) | `H-1` | n/a | n/a — `H` not yet final |
| **C1** after `SaveBlock(H)`, before WAL EndHeight | `H` | `H-1` | `H-1` | **Yes** (real app) | `H-1` | recompute `AppHash(H)`, checked vs `H+1` header | idempotent `Commit(H)` |
| **C2** after WAL EndHeight, during `FinalizeBlock(H)` | `H` | `H-1` | `H-1` | **Yes** (real app) | `H-1` | recompute `AppHash(H)` | idempotent |
| **C3** during `Commit(H)` (Mongo txn in flight, rolls back) | `H` | `H-1` | `H-1` (atomic rollback) | **Yes** (real app) | `H-1` | recompute `AppHash(H)` | idempotent; rolled-back partial overwritten |
| **C4** after `Commit(H)` done, **before** `stateStore.Save` | `H` | `H-1` | **`H`** (watermark+records durable) | **No — mock** (app already at `H`) | `H` | CometBFT takes AppHash from saved `FinalizeBlockResponse`, cross-checked | app not re-invoked; even if it were, idempotent |
| **C5** after `stateStore.Save` | `H` | `H` | `H` | n/a | `H` | AppHash matches (clean) | settled |

Reading the critical rows:
- **C1–C3 (crash before `Commit` completes):** Mongo is still `H-1` (the atomic
  transaction means `H` never half-appears); the app truthfully reports `H-1`;
  CometBFT sees `storeH == stateH+1` with `appH == stateH` and **replays `H` with
  the real app** (re-runs `FinalizeBlock(H)`+`Commit(H)`). Determinism makes the
  recomputed `AppHash(H)` identical; idempotency makes the Mongo re-apply
  converge. No loss, no duplication.
- **C4 (crash after Mongo `Commit`, before CometBFT state save):** the exact
  "between Mongo commit and CometBFT persistence" window from the review. Mongo
  durably has `H`; the app truthfully reports `H`; CometBFT sees
  `storeH == stateH+1` with `appH == storeH` and **replays with a mock app** from
  the saved `FinalizeBlockResponse`, advancing to `H` **without** re-calling our
  app — so `H` is **neither double-applied nor lost**. That response was persisted
  by CometBFT *before* `Commit` (§11.1), so it is available for this mock replay.
- **App ahead of store is impossible** by §11.1 fact 1; if it ever occurred
  (e.g. direct Mongo tampering) CometBFT halts with `ErrAppBlockHeightTooHigh`.

**Invariants the app must uphold for this table (normative):**
1. `Commit(H)` is **atomic** and writes the watermark **last** (so `Info` never
   over-reports). (§11.2)
2. `FinalizeBlock`/`Commit` are **deterministic and idempotent** (re-runnable to
   the same state). (§9, §11.6)
3. `Info` reports the **durable** watermark and its AppHash, never a staged value.
4. All validators run the **same binary/toolchain** so recomputed AppHashes match
   (C1–C3); a mismatch is caught by the handshake cross-check (panic) and the
   §9.2 monitor.

**Executable tests:** every row C0–C5 is a deterministic crash-injection test
(kill at the exact point) asserting the post-restart invariants — consistent
height, matching AppHash, no duplicated/missing records, correct `GetReceipt`.
See acceptance R1–R6 (§14.4), especially R5 (the C4 window) and R6 (full matrix).

### 11.5 Periodic reconciliation (defense in depth)

A low-frequency background check compares CometBFT's committed height to the
Mongo watermark and triggers the same replay if they diverge. This bounds any
window where a committed block is visible in consensus but not yet in Mongo to
"until next reconciliation," never "permanently." (The handshake covers restart;
this covers a *running* node whose watermark somehow lags.)

### 11.6 Idempotency and duplicate-persistence prevention

Application of a block is **at-least-once** (replayable) and **idempotent**.
Duplicate persistence is prevented by four composing mechanisms:
1. **Atomic `Commit` + watermark-last:** a height either fully appears (watermark
   advances) or not at all; re-running converges.
2. **Content-addressed records:** `_id` = Record ID, so re-insert is a no-op.
3. **`txId`-keyed receipts:** idempotent upsert of `(txId → height,index)`.
4. **Deterministic sender-tails:** set to the deterministic value, not
   incremented, so re-apply is identical.

### 11.7 What about Mongo loss? (and malicious state-sync)

If Mongo is lost entirely (disk failure), the node rebuilds it by **CometBFT
state-sync from peers** + replay into a fresh Mongo, as a new node bootstraps;
the watermark and sender-tails are re-derived by replay, not trusted from a backup
alone. State-sync snapshots are **cryptographically verified** (app hash + merkle)
before acceptance, so a **malicious/forged snapshot is rejected** and the node
falls back to block replay / another peer (acceptance L5). A synced node is
known-good by construction.

---

## 12. Migration design (requirement 7)

### 12.1 The honesty constraint

The legacy dataset is a **DAG of per-sender chains** with no global total order
and no finality. We MUST NOT present it as if it were globally finalized
CometBFT history, and we MUST NOT manufacture a false total order over it.

**Decision: clean cutover, legacy corpus sealed and clearly labeled.** We do
**not** replay legacy records as CometBFT transactions (that would fabricate a
total order and retroactively "finalize" data that was never ordered). Instead:

### 12.2 Migration tool: validate → detect → report → (only if clean) seal

The legacy dataset is **not guaranteed to be well-formed**. The pre-Phase-3
system accepted any structurally-valid, correctly-signed block whose predecessor
a node happened to hold, and never enforced a single tip per sender — so a
sender can legitimately have **forked histories, multiple valid tips, dangling
predecessors, or invalid records**. The migration MUST assume this and handle it
explicitly. It is a one-time, **offline, deterministic** tool
(`cmd/legacy_migrate`) that reads the legacy DB, **produces a reviewable
report**, and only emits genesis `app_state` if the data is clean or explicitly
resolved. **It never silently selects, drops, or discards a conflicting
history.**

#### Step 1 — Per-record validation
For every legacy record:
- `BlockStructValidator` (shape, hex, size, timestamp window).
- Recompute the Record ID with `BlockHashMaker` and require it equals `_id`.
- Verify the **client signature**: `Verify(senderPubKey, senderUid,
  MessageMaker(BlockData), Signature)` (also re-derives UID from key).
- Verify the **legacy node signature**: `VerifySignature(dns_seed PubKey(NodeUID),
  NodeMessage(block), NodeSignature)` (genesis exempt) — the frozen
  `dns_seed.json` is used here only, for legacy validation (§5.4).
Invalid records are **flagged** (`status: INVALID`, reason recorded), never
silently dropped.

#### Step 2 — Per-sender predecessor graph and anomaly detection
Group valid records by `senderUid`. For each sender `S`:
- **Resolve predecessors:** each record's `PreBlockHash` must equal either
  `FIRST_BLOCK_HASH` (for a first record) or the `_id` of another **valid**
  record by the same sender `S`. A reference that resolves to a missing record,
  an invalid record, or a record by a *different* sender is a **dangling/invalid
  predecessor** — flagged.
- **Edges:** parent (`PreBlockHash`) → child (the referencing record).
- **Tips:** records not referenced as a parent by any other record of `S`.
- **Fork:** a record referenced as a parent by **≥ 2** distinct children of `S`.
- **Classify `S`:**
  - `CLEAN` — exactly **one** tip, **no** fork, all predecessors resolve, all
    records valid.
  - `CONFLICTED` — multiple tips, and/or a fork, and/or a dangling predecessor.
  - `INVALID` — contains one or more invalid records.

#### Step 3 — Deterministic, reviewable report
Emit a report (committed as an audit artifact alongside the migration), sorted by
`senderUid` then Record ID, with per sender: record count, tip(s), classification,
and for non-clean senders the specific anomaly(s):
- multiple tips → list of tip Record IDs;
- fork → the fork parent Record ID + its children;
- dangling predecessor → the referencing Record ID + the missing/invalid ref;
- invalid record → Record ID + reason.
Plus global totals and a preview of the resulting genesis hash. The report is
byte-for-byte reproducible from the same input (deterministic ordering).

#### Step 4 — Refuse-on-conflict (mandatory)
If **any** sender is `CONFLICTED` or `INVALID`, the tool:
- still emits the full report;
- does **NOT** emit a usable `app_state.sender_tails` for those senders;
- exits **non-zero** with the migration marked **BLOCKED**, pending explicit
  operator resolution.

It MUST NOT auto-select one tip, MUST NOT drop the losing fork branch, and MUST
NOT invent continuity it cannot justify.

#### Step 5 — Explicit operator resolution (manual, recorded)
For each conflicted sender an operator decides **out of band**, recorded in an
input **resolution manifest** (`senderUid → chosen tip Record ID`, or
`senderUid → no-continuity`). The tool consumes this manifest and **never**
chooses on its own. Options per sender:
- select the canonical tip (operator asserts which history is authoritative);
- `no-continuity` — the sender is deliberately seeded to `FIRST_BLOCK_HASH`
  (its next record starts a fresh chain). This is an explicit, audited decision,
  not a silent default. Invalid records are **quarantined (flagged), not
  deleted**; the migration never destroys legacy bytes.

#### Step 6 — Seal and seed (only when clean / fully resolved)
- Tag all validated legacy records `Era: "legacy"` (read-only history,
  not-finalized, §12.3).
- Write `sender_tails` = the operator-chosen tip per sender (or omit senders
  marked `no-continuity`).
- Carry `FIRST_BLOCK_HASH` into genesis `app_state`.
A sender's first post-cutover record then links `PreBlockHash` to its chosen
legacy tip (or genesis), preserving **only** the continuity the operator
authorized — never a manufactured global order.

### 12.3 What is explicitly NOT claimed

- Legacy per-sender chains are **not** re-declared as globally finalized blocks.
- There is **no** retroactive height assigned to legacy records
  (`Consensus.Height = 0`, `Era = "legacy"`), and they are excluded from the
  `AppHash` record-log commitment (the AppHash covers only `Era:"consensus"`
  records from height ≥ 1). The genesis AppHash commits to the seeded
  `sender_tails` and `FIRST_BLOCK_HASH`, not to a replayed legacy order.
- `ReadData` can filter `Era` so clients can distinguish "legacy, non-final"
  from "consensus, finalized."

### 12.4 Genesis guard continuity

The existing `db.Genesis_check` / `ErrLegacyDatabase` guard (which rejects a DB
whose genesis hash differs from this build's) is **retained and extended**: the
Phase 3 genesis carries a **format/version marker** so a pre-Phase-3 DB cannot
be silently reinterpreted as a Phase-3 chain. A pre-Phase-3 DB triggers the same
"operator must migrate" path; the migration above is that path.

### 12.5 Migration procedure (operational)

1. Quiesce the old network at a known state (all nodes stopped, DB consistent).
2. **Back up** Mongo (existing `docs/MIGRATION.md` discipline).
3. Run `cmd/legacy_migrate` in **report-only** mode (§12.2 Steps 1–3): validate
   records, build per-sender graphs, emit the deterministic report. **Do not**
   mutate data yet.
4. **Review the report.** If any sender is `CONFLICTED`/`INVALID`, STOP and
   obtain operator resolutions (§12.2 Step 5). Re-run until clean/resolved;
   the tool must exit zero before proceeding.
5. Run the tool in **apply** mode: tag `Era:"legacy"`, write `sender_tails` from
   the (clean or resolved) tips, carry `FIRST_BLOCK_HASH`.
6. Generate CometBFT `genesis.json` (4 ed25519 validators, equal power) with the
   exported `app_state`.
7. Stand up the Phase-3 nodes; each verifies its genesis hash matches (guard),
   starts CometBFT, reconciles Mongo (§11), and opens the gate.
8. New submissions flow through CometBFT; legacy records remain read-only
   history with an explicit "not finalized" label.

### 12.6 Rollback

Because the cutover only *adds* `Era:"legacy"` tags and a new genesis, rollback =
restore the Mongo backup and restart the pre-Phase-3 binary. No legacy record
content is mutated (invalid records are flagged/quarantined, never deleted), so
rollback is lossless.

---

## 13. Failure and recovery model

Assumption: n = 4, equal power, f = 1 (n ≥ 3f + 1). Quorum = 3.

| Failure | Effect | Recovery / behavior |
|---|---|---|
| 1 validator crashes | 3 remain ⇒ **liveness + safety preserved**. | Restarted node state-syncs + replays (§11). |
| 2 validators crash | only 2 < quorum ⇒ **chain halts** (no new commits). **Safety preserved.** | Restart a 3rd node ⇒ progress resumes. No conflicting block can commit. |
| 2-2 network partition | neither side has 3 ⇒ **both sides halt**, no commits. **Safety preserved** (no double-spend/equivocation across the partition). | Heal partition ⇒ single chain resumes from the higher committed height. |
| 1 Byzantine (equivocation / double-sign) | ≤ f ⇒ **safety preserved**; CometBFT records **evidence**; chain continues with 3 honest. | Evidence logged/surfaced (no slashing in Phase 3). A repeat offender is removed only via the §10.5 chain-migration procedure (the Phase 3 set is immutable, §10.1). |
| 1 Byzantine + 1 crash (2 faulty) | 2 of 4 faulty exceeds f for safety-relevant thresholds ⇒ **halt rather than fork**; safety not violated as long as the 2 are not both equivocating with > 1/3. | Manual intervention / coordinated restart. Documented limitation of f=1. |
| Node disk/Mongo loss | that node's app state rebuildable. | State-sync + replay from peers (§11.7). |
| Crash between FinalizeBlock and Commit (C1–C3) | block in store, not yet visible in Mongo. | Handshake replays `H` with the real app; re-materialized (§11.4). **Not permanently lost.** |
| Crash after Commit, before CometBFT state save (C4) | Mongo has `H`; CometBFT state at `H-1`. | Handshake advances CometBFT via the saved `FinalizeBlockResponse` (mock), **no double-apply** (§11.4). |
| App reports height above CometBFT store | cannot happen (§11.1); tampering indicator. | CometBFT refuses to start (`ErrAppBlockHeightTooHigh`) — safe halt. |
| Duplicate tx / replay attack | same TxID | deduped at mempool + `CheckTx` + unique index; idempotent (§7.6). |
| Conflicting proposals (Byzantine proposer) | honest validators run `ProcessProposal` | invalid/duplicate-laden proposal **rejected** identically by all; consensus moves to next proposer. |
| Mempool flooding / DoS | resource pressure | `CheckTx` bounds + rate limiters + CometBFT mempool size caps; invalid txs cheaply rejected. |
| Accepted-but-uncommitted tx lost on restart | `PENDING` tx gone | Expected; client resubmits idempotently (§7.8). |

**Key guarantee:** safety is never sacrificed for liveness. When quorum is
unavailable the chain **halts** rather than forking. This is the correct BFT
trade-off and is asserted by the tests in §14.

---

## 14. Acceptance criteria (requirement 8)

Phase 3 is accepted only when **all** of the following pass in CI / a staging
cluster. Each is a concrete, automatable test.

### 14.1 Safety
- **S1 — No two conflicting commits:** under a Byzantine proposer and adversarial
  tx ordering, no two different blocks commit at the same height across correct
  validators. (Deterministic simulation + multi-node run.)
- **S2 — AppHash determinism:** every validator computes an identical AppHash
  for the same committed block sequence; a deliberately injected nondeterminism
  (e.g. map-order iteration) causes a detectable mismatch (negative test).
- **S3 — Record ID stability:** a given signed `BlockData` yields the **same
  Record ID** pre- and post-Phase-3 (golden vectors from the existing suite).
- **S4 — Signature/identity preservation:** existing client-signature and
  UID-derivation test vectors still pass unchanged (P-256, low-S, domain tags).
- **S5 — PreBlockHash correctness:** per-sender chains are correctly formed under
  global order, including multiple same-sender txs in one block and across the
  legacy cutover boundary (sender's first new record links to legacy tip).
- **S6 — Different-AppHash detection + fail-closed isolation:** with a
  deliberately divergent validator (e.g. injected map-order nondeterminism on one
  node), the cross-validator AppHash monitor (§9.2) **detects the divergence at
  that height** and the node **automatically fails closed** — asserts all four
  actions (stops serving trusted reads, stops accepting submissions, stops
  validator signing/voting) **and** that it **does not auto-resync or auto-rejoin**
  (recovery requires the explicit procedure), **and** that forensic state is
  preserved. (Addresses the review: CometBFT does not report this for us.)
- **S7 — Complete-state commitment:** mutate a field that influences future
  validation (sender-tail, committed-signature dedup set) on one node and assert
  the AppHash changes and divergence is caught — proves the AppHash commits the
  *full* validation-relevant state (§9.3), not just the record log.

### 14.2 Liveness & consensus
- **L1 — 4/4 happy path:** tx submitted → committed within a bounded number of
  heights; receipt transitions PENDING → COMMITTED.
- **L2 — 3/4 (one crash):** progress continues.
- **L3 — 2/4 (two crash):** chain halts cleanly, no commits, resumes when a
  third node returns; **no conflicting block** ever appears.
- **L4 — Partition (2-2):** both sides halt; on heal, converge to one chain
  without reorg of committed blocks.
- **L5 — Malicious state-sync snapshot rejected:** a peer serves a forged/invalid
  state-sync snapshot (bad app hash, truncated chunk, non-matching merkle); the
  syncing node **rejects** it and falls back to block replay / another peer,
  never trusting the bad snapshot. Assert the AppHash/merkle verification on
  state-sync (§14 state-sync note).

### 14.3 Byzantine
- **B1 — Equivocation evidence:** a double-signing validator produces evidence
  that is logged and visible; honest chain continues.
- **B2 — Conflicting proposal rejected:** a Byzantine proposer's invalid proposal
  is rejected by `ProcessProposal` on all honest validators; consensus proceeds.
- **B3 — Invalid tx rejected:** malformed / bad-signature / oversized /
  duplicate txs are rejected at `CheckTx`/`ProcessProposal` and never commit.

### 14.4 Restart & durability (the requirement-5 tests)
- **R1 — Commit durability:** a committed record survives `kill -9` of a node
  at every crash window (during FinalizeBlock, during Commit, after Commit);
  on restart `GetReceipt` returns COMMITTED and the record is present. **(Primary
  acceptance test for §11.)**
- **R2 — Mongo behind CometBFT:** simulate CometBFT ahead of Mongo; restart
  replays and reconciles until watermark == CometBFT height; no committed record
  missing.
- **R3 — Full Mongo loss:** node rebuilds from state-sync + replay and serves
  correct reads.
- **R4 — Watermark integrity:** Mongo is never ahead of CometBFT (negative test:
  tamper detection).
- **R5 — Crash between Mongo `Commit` and CometBFT state persistence:** kill the
  process in the window where the app has persisted height `H` to Mongo but
  CometBFT has not yet written block `H` to its block store / advanced its state.
  On restart the handshake/replay (§11.3) must reconcile to a single consistent
  height with **no duplicated or missing records** and a **matching AppHash**;
  if the app is momentarily ahead, the node resyncs/trusts CometBFT's height as
  authoritative and re-derives. **This is the exact window the review flagged;
  it maps to §11 state-machine rows and MUST be a crash-injection test.**
- **R6 — Crash-injection matrix:** every crash window enumerated in §11.4 is
  exercised by an automated test that kills the process at that exact point
  (deterministic fault injection), then asserts the post-restart invariants
  (consistent height, AppHash match, no dup/missing records, correct
  `GetReceipt`).

### 14.5 Duplicates & API
- **D1 — Idempotent resubmission:** N identical submissions ⇒ exactly one
  committed record; all callers get the same TxID/Record ID.
- **D2 — Duplicate returns existing:** `SubmitTx` on a known TxID returns it
  without creating a second record.
- **D3 — Lost PENDING tx:** a PENDING tx lost on restart ⇒ resubmission
  succeeds; no corruption.
- **D4 — Backward-compatible AddData:** legacy `AddData` wrapper returns only
  after finality (or documented deadline), with correct Record ID in `BlockHash`.
- **D5 — Receipt matches the CometBFT commit:** for a `COMMITTED` receipt, assert
  its `(Height, CometBlock, TxIndex, RecordId)` matches the CometBFT block store
  (the tx is at that index in that block, the header hash equals `CometBlock`,
  and the record's Record ID recomputes). A receipt for an uncommitted tx never
  reports `COMMITTED`. Verifies the Phase-3 receipt is faithful to consensus
  (and that it is **not** claimed to be a standalone proof: `Verifiable=false`).
- **D6 — AddData timeout is not failure:** force `ADD_DATA_FINALITY_TIMEOUT` to
  expire before commit; assert the wrapper returns `IsSuccess=false` with an
  "outcome unknown — query GetReceipt" log **and** a `TxId`, that the tx still
  commits afterwards, and that `GetReceipt(TxId)` then reports `COMMITTED`. A
  timeout is **never** reported as definitive failure (decision 2).
- **D7 — Chain-ID replay protection:** a tx submitted on chain-id `A`, after a
  §10.5 migration to chain-id `B`, cannot create a second record on `B` (TxID is
  chain-scoped and the `(senderUid,signature)` dedup, seeded with migrated
  history, rejects it) (decision 3).

### 14.6 Migration
- **M1 — Seal + label:** all pre-existing records are `Era:"legacy"`, readable,
  and flagged not-finalized; legacy signatures still verify.
- **M2 — Continuity across cutover:** a sender whose only history is legacy links
  its first new record to its legacy tip.
- **M3 — Guard:** a pre-Phase-3 DB is rejected with a clear migrate-first error
  (no silent reinterpretation).
- **M4 — Rollback:** restoring the backup + old binary is lossless.
- **M5 — Multiple legacy tips / forks detected:** a fixture where a sender has
  **≥ 2 tips**, a **fork** (one parent, two children), a **dangling predecessor**,
  and an **invalid record**; the migration report classifies each correctly and
  the tool **refuses** (non-zero, BLOCKED) without selecting a tip. **(Review
  requirement: never silently discard/select conflicting history.)**
- **M6 — Operator resolution honored:** given a resolution manifest, the tool
  seeds exactly the chosen tips (and honors `no-continuity`), never the others,
  and the applied `app_state` matches the manifest. Invalid records remain
  flagged/quarantined, not deleted.

### 14.7 Non-functional
- **N1 — Throughput/latency:** documented baseline for `SubmitTx`→COMMITTED at
  target load; `AddData` latency increase quantified and within agreed bound.
- **N2 — Rate limiting:** per-key limits enforced; `GetReceipt` cheap; the
  pre-auth bucket issue is fixed (limit after identity/IP known).
- **N3 — `go build ./...`, `go vet ./...`, `go test ./...`, and `-race` pass**;
  a multi-node staging run (not just unit tests) is executed.

### 14.8 P2P security (§4.5)
- **P1 — Unauthorized peer rejected:** a node whose **node key** is not in the
  configured allowlist (`private_peer_ids` / `addr_book_strict` / firewall) is
  **not** accepted as a persistent/trusted peer and cannot obtain consensus data
  it is not entitled to; a flood of such connection attempts is bounded and does
  not degrade honest-peers connectivity. (Addresses: authenticated ≠ authorized.)
- **P2 — Node key vs. validator key separation:** removing a validator's
  **consensus (validator) key** from the set does **not** change its **node
  key**, and rotating a **node key** does **not** change consensus voting power;
  the two are provisioned/stored/rotated independently (§4.5). Test both.
- **P3 — Validators-only admission (Phase 3):** only the 4 validators' node IDs
  are accepted as p2p peers; any other node ID is rejected at the allowlist/
  firewall. (Read-only full nodes are a **later phase**, not enabled here —
  decision 6.)

### 14.9 Canonical transaction serialization (§3.4)
- **C1 — Cross-language canonical TxID:** golden vectors (tx bytes → TxID) are
  asserted by **≥ 2 independent implementations** (e.g. a Go and a non-Go
  reference); all produce identical TxIDs. **(Mandatory if multi-language
  clients are supported.)**
- **C2 — Unknown-field rejection:** a tx with an extra/unknown field or a
  non-canonical integer encoding is **rejected**, not silently accepted, and two
  byte-strings that differ only by a non-canonical encoding do **not** map to the
  same accepted tx.
- **C3 — Stability:** the same logical `BlockData` yields the same TxID before
  and after any library upgrade (no protobuf-wire dependence).
- **C4 — Chain-ID scoping:** the same `BlockData` under two different chain-ids
  yields two different TxIDs (chain-id is in the canonical domain, §3.4); golden
  vectors cover both.

### 14.10 Validator-set lifecycle (§10)
- **V1 — Immutable set in Phase 3:** the app's `FinalizeBlock` returns **empty**
  `ValidatorUpdates`; any attempt to change the set in normal operation is a
  no-op/rejected. The set is fixed at genesis for the chain's life in Phase 3.
- **V2 — Exceptional replacement procedure:** the documented coordinated
  replacement (export state → new genesis with the replacement validator → all
  nodes re-verify genesis hash → restart) is executed in a staging cluster and
  results in a single consistent chain with the new validator, no double-sign,
  and correct `app_state` continuity. (This is an operational migration, **not**
  an in-protocol update.)

### 14.11 Size / resource limits (§3.5)
- **OS1 — Oversize tx rejected:** a tx whose canonical bytes exceed
  `MAX_TX_BYTES` is rejected at `CheckTx` (and never admitted to the mempool).
- **OS2 — Limit-layering:** assert `mempool.max_tx_bytes ≤ consensus.max_block_bytes
  ≤ max_packet_msg_payload_size`; a block built under `PrepareProposal` never
  exceeds `consensus.max_block_bytes`.
- **OS3 — Build-time limit assertion:** if configured `MAX_DATA_SIZE_KB`/
  `MAX_WASM_SIZE_KB` would exceed `MAX_TX_BYTES`, the build/startup assertion
  fails.
- **OS4 — Self-contained contract tx:** a max-size (1024 KB) contract payload round
  trips inside one tx and one committed record without truncation, and the stored
  bytes hash to `ContractData.Checksum`.

---

## 15. Resolved technical decisions (APPROVED)

The following are **approved** (Revision 2 direction approved 2026-11-23, with
decisions 1–7 from the approval incorporated below and in the referenced
sections). They are normative for Phase 3:
- Contract payload is **self-contained in the tx**, retained together with the
  explicit, tested size/resource limits (§3.5) — decision 1.
- `AddData` is a backward-compatible synchronous wrapper with a **configurable
  timeout that is never treated as failure** (§7.7) — decision 2.
- Validator replacement uses a **new chain ID**, and the chain ID is in the tx
  signing/replay-protection domain (§3.4, §10.5) — decision 3.
- AppHash divergence triggers **automatic fail-closed isolation** with no
  auto-rejoin (§9.2) — decision 4.
- Legacy migration **refuses ambiguous histories** and requires **explicit,
  auditable** operator resolution; no-continuity is allowed but recorded (§12) —
  decision 5.
- **Validators-only** for the initial Phase 3 p2p deployment; full nodes later
  (§4.5.2) — decision 6.
- **MongoDB Replica Set is a production requirement**; each validator runs its own
  independent MongoDB; single-node RS is acceptable for transactions but is not
  DB HA (item 1 below) — decision 7.

The detailed, normative statements follow.

1. **MongoDB Replica Set — MANDATORY for production (approved, decision 7).** The
   atomic `Commit` (§11) uses multi-document transactions, which require a replica
   set. Standalone `mongod` is **not** supported for production; a node connected
   to a standalone server MUST refuse to start in production mode. **Each
   validator has its own independent MongoDB instance** (no shared database — a
   shared DB would couple validators' failure domains and break independence).
   A **single-node replica set is acceptable** for transaction support, but it
   provides **no database HA** — it only enables transactions, not redundancy.
   Production HA (a multi-node replica set per validator) is recommended
   operationally but is an infrastructure choice, not a consensus requirement.
   (Development/test may use the in-memory fake.)
2. **Mempool — CometBFT default mempool** (CList + gossip) with `CheckTx`
   validation. The app-mempool (Krakatoa / `InsertTx`/`ReapTxs`) model is a
   future option, not Phase 3.
3. **Receipt storage — persistent `txId → (height, index)` index**, written in
   the `Commit` transaction (§7.3, §11.2). Receipts are served from this durable
   index, not from CometBFT's prunable RPC.
4. **Timestamp validation — deterministic, against CometBFT block time.**
   `BlockMeta.TimeStamp` and any validation that depends on time use the BFT
   block time. The client-time skew bound (`TIMESTAMP_SKEW_MS`) is applied
   **advisory at admission (`CheckTx`)** and **re-checked deterministically in
   `ProcessProposal` against block time**, so all validators agree. **No local
   wall-clock dependency exists in consensus execution** (§9).
5. **Legacy data — sealed, read-only, explicitly non-finalized** (`Era:
   "legacy"`), per §12. Migration refuses conflicting senders and requires
   explicit operator resolution (§12.2).
6. **Rate limiting — pre-authentication bucket issue fixed.** The limiter is
   keyed/applied **after** identity is established (or keyed on connection IP for
   unauthenticated callers) so an unauthenticated caller cannot pre-poison a
   victim's API-key bucket (§7.6; acceptance N2).
7. **Merkle construction — fully specified** (RFC 6962 leaf/node domain
   separation, ordering, empty-root constant, power-of-two split rule with no
   odd-node ambiguity) in **Appendix D**, with golden test vectors (Appendix C;
   acceptance S2, C1).

**Residual items that are implementation mechanics, not open design choices**
(to be executed, not decided): CometBFT is pinned to **v1.0.1** (§4.1) and
Appendix B cites the verified v1.0.1 source; the remaining mechanic is
reconciling the `nm.NewNode`/`proxy.NewLocalClientCreator` argument lists to the
v1.0.1 signatures when coding. This does not change the architecture and is an
Appendix A implementation step.

---

## 16. Implementation phases (plan APPROVED — implementation NOT yet authorized)

The plan below is **approved** (Rev 4). The following **binding conditions** govern
execution:
1. **Phases S and 0–6** and their gates are approved as described. **No phase
   begins until the preceding phase's acceptance criteria are met AND the gate
   evidence is reviewed and signed off.** A green test run is necessary but not
   sufficient — review sign-off is required to advance.
2. **A dependency & security review (Phase S) MUST complete before Phase 0.** The
   CometBFT pin is **not** grandfathered in merely because it was chosen during
   design; Phase S re-justifies it (see §17).
3. Every **direct and critical transitive** dependency is reviewed (CometBFT,
   MongoDB driver, gRPC, protobuf, Go toolchain), with vulnerability scanning and
   an **auditable dependency inventory (SBOM)**.
4. Exact toolchain/dependency pins are committed, with a documented **upgrade
   policy, security-patch procedure, and definition of a coordinated consensus
   upgrade** (§17).
5. **Phase 0** produces **independently verified** golden vectors (TxID, canonical
   serialization, Merkle roots, AppHash) plus **deterministic cross-platform**
   tests.
6. **Phase 2's crash-recovery matrix is a HARD GATE.** Multi-validator work
   (Phase 3) MUST NOT begin until durability and recovery (R1–R6) are proven and
   reviewed.
7. **Phase 3** includes **real network / failure-domain testing** (multiple hosts,
   real partitions), not only local multi-process tests.
8. **Phase 6** includes a complete **staging rehearsal**: bootstrap, transaction
   finality, validator failure/recovery, database loss/rebuild, and migration.

| Phase | Scope | Gate (met AND reviewed to advance) |
|---|---|---|
| **S — Dependency & security review** (hard gate before Phase 0) | Re-justify the CometBFT pin (current stable + LTS lines, advisories, fixes, API/protocol/Go-compat); review all direct + critical transitive deps; vuln scanning; SBOM/inventory; upgrade + patch + coordinated-upgrade policy (§17) | Signed security-review report + justified final versions + committed SBOM + CI vuln-scan integrated |
| **0 — Foundations** | Confirmed pins; remove `wazero`/`participle`/`ws`; canonical tx encoding (§3.4) + **independently verified** golden vectors (App. C) + Merkle (App. D) + limits (§3.5); uniform toolchain; cross-platform deterministic tests; CI green | TxID/canonical/Merkle/AppHash golden vectors verified by ≥2 impls (C1–C4); cross-platform determinism; build/vet/test |
| **1 — ABCI app (single node, in-memory)** | `Info`/`InitChain`/`CheckTx`/`FinalizeBlock`/`Commit` vs in-memory Mongo; deterministic exec; AppHash (§9.1/D); PreBlockHash via sender-tails (§8) | Single-node commits; AppHash golden chain (S2/S3/S4/S5) |
| **2 — Durable persistence + recovery** (HARD GATE) | Atomic watermark-last `Commit` on a Mongo replica set; `Info`/handshake replay; crash-injection harness | **R1–R6 crash matrix + D-series proven and reviewed. Phase 3 is blocked until this passes.** |
| **3 — Multi-node consensus** | 4-validator genesis (ed25519, equal power); validators-only p2p allowlist (§4.5); wire `SubmitTx`/`GetReceipt`/`ReadData`/`AddData`; **real multi-host + failure-domain + partition testing** | L1–L4, B1–B3, D1–D7, P1–P3 **on real network/failure domains** |
| **4 — API hardening + monitoring** | Ordinary receipts, rate-limit fix, `AddData` timeout semantics (§7.7); AppHash reconciliation monitor + fail-closed isolation (§9.2) | D6, N-series, S6/S7 |
| **5 — Migration tooling** | Legacy validate/report/resolve/seal (§12); genesis generation; chain-migration runbook (§10.5) | M1–M6; V2 rehearsal |
| **6 — Staging dress rehearsal** | Full staging: bootstrap, transaction finality, validator failure/recovery, **database loss/rebuild**, migration, validator-replacement rehearsal (V2), perf baseline (N1) | Entire §14 matrix green in staging **and reviewed** |

## 17. Dependency and version pinning (determinism-critical)

All pins are **exact** (no version ranges); `go.sum` is committed; **every
validator and node runs the identical toolchain and dependency set** — this is a
correctness requirement (a toolchain/dependency skew is a classic nondeterminism
source, §9.2/§19).

> **PHASE S RESULT (2026-11-23) — see `docs/PHASE_S_DEPENDENCY_REVIEW.md`.**
> The security review **disqualified the design's CometBFT `v1.0.1`**: the v1.x
> line is unmaintained and is **not** patched for the **Critical** consensus
> advisory **CSA-2026-001 "Tachyon"** (BFT-Time inflation), which directly affects
> our block-time use. The scan also found **20 reachable vulnerabilities** in the
> current set (incl. a gRPC authorization bypass and a mongo-driver OOB read).
> The **Phase-S-selected secure pins** below are to be **applied in Phase 0**
> (upgrades are out of Phase S scope). The CometBFT change and the §4/§11/App-B
> internals that were verified against v1.0.1 require **re-verification against
> the chosen tag** and your approval (unresolved decision, §11 of the report).

| Component | Current | **Phase S selected (apply in Phase 0)** | Note |
|---|---|---|---|
| **CometBFT** | — (design said `v1.0.1`, **disqualified**) | **`v0.40.0`** (recommended) / `v0.38.26` (conservative) | Tachyon-fixed, ABCI 2.0, maintained; re-verify internals |
| **Go toolchain** | `go 1.24.0` directive; installed `1.26.6` | **`1.26.11`**, identical on all nodes+CI | 8 reachable stdlib CVEs fixed in 1.26.9–1.26.11 |
| `google.golang.org/grpc` | `v1.69.0` | **`v1.83.1`** | 4 reachable vulns (authz bypass, panic, OOM) |
| `go.mongodb.org/mongo-driver` | `v1.17.6` | **`v1.17.7`** | GSSAPI heap OOB read |
| `google.golang.org/protobuf` | `v1.35.2` | keep / align with grpc | not flagged reachable |
| `golang.org/x/net` (transitive) | `v0.43.0` | **`≥ v0.60.0`** | 5 reachable HTTP/2 vulns |
| `golang.org/x/text` (transitive) | `v0.29.0` | **`≥ v0.39.0`** | infinite-loop DoS |
| `github.com/klauspost/compress` (transitive) | `v1.17.11` | **`≥ v1.20.2`** | panic on two-block frames |
| `github.com/gorilla/websocket` | `v1.5.3` | **REMOVE** | unmaintained; `ws/` removed |
| `github.com/tetratelabs/wazero` | `v1.9.0` | **REMOVE** | BVM execution removed |
| `github.com/alecthomas/participle/v2` | `v2.1.4` | **REMOVE** | unused |

These Phase-S pins are **recommended selections**; applying them (editing
`go.mod`/`go.sum`, adding CometBFT, removing the three deps, bumping the toolchain
directive) is **Phase 0 work**, gated on your approval of the selections and the
CometBFT version change.

### 17.1 Dependency inventory & vulnerability scanning (Phase S deliverables)

- **Auditable inventory (SBOM):** a complete, committed SBOM (SPDX or CycloneDX)
  covering every direct **and critical transitive** dependency with exact
  versions and hashes, generated in CI (`go list -m all` + `govulncheck` facts).
  This is the authoritative dependency record and is reviewed at every phase gate.
- **Vulnerability scanning:** CI runs `govulncheck` (reachability-aware Go CVE
  check), OSV-Scanner, and a container/filesystem scanner (e.g. Trivy) on every
  build; results are gating (no known exploitable High/Critical in the pinned set
  without a documented, reviewed exception). Scans cover CometBFT, MongoDB driver,
  gRPC, protobuf, the Go toolchain, and the crypto/edge libraries.
- **Per-dependency review:** for each direct + critical transitive dependency —
  current version, CVEs/GHSAs, maintenance status, license, and whether an upgrade
  is required — recorded in the Phase S report.

### 17.2 Final version selection (Phase S deliverable)

A **justified** selection, not an assumption. Phase S surveys:
- **CometBFT:** the latest **v1.0.x** patch vs the newer parallel lines (e.g.
  `v0.40.x`, any `v1.y`), weighing LTS status, security fixes, PBTS, API/protocol
  stability, and Go requirement. The outcome (confirm `v1.0.1` or move to a newer
  justified patch/minor) is recorded with rationale and upstream references.
- **Go toolchain:** the exact patch that satisfies CometBFT's requirement and is
  itself free of known vulnerabilities; pinned identically everywhere (§17.4).
- **MongoDB driver / gRPC / protobuf:** current patched versions compatible with
  the chosen Go + CometBFT; justified over the existing pins.

### 17.3 Upgrade policy and security-patch procedure

- **Patch (e.g. v1.0.1 → v1.0.2):** expected non-breaking; routine. Flow:
  advisory/release monitoring → CVE assessment → bump pin → full CI (incl. vuln
  scan + §14 regression) → rolling restart (validators one at a time where the
  patch is consensus-compatible).
- **Minor (may change Go APIs):** planned; may be consensus-compatible but requires
  code changes + full re-test; scheduled maintenance.
- **Major / consensus- or state-breaking:** a **coordinated consensus upgrade**
  (§17.5).
- **Security-patch SLA:** Critical/High advisories affecting a reachable path are
  triaged within a defined SLA (e.g. Critical ≤ 72h to a patched, tested build),
  with an out-of-band release process; the SBOM + scanning make "are we affected?"
  answerable quickly.
- **Monitoring:** subscribe to CometBFT, Go, MongoDB, gRPC/protobuf security
  feeds and the project's GHSA/CVE sources; scanning in CI is the backstop.

### 17.4 Toolchain uniformity (determinism requirement)

The exact Go toolchain (and `GOFLAGS`/`CGO_ENABLED`/build tags) is **identical on
every validator, node, and CI runner** and pinned (e.g. via `go.mod` `toolchain`
directive + a checked-in builder image). A toolchain/dependency skew is a
nondeterminism source (§9.2/§19); builds record their toolchain+deps hash and a
node refuses to start if it does not match the approved set.

### 17.5 Coordinated consensus upgrade (definition)

A **coordinated consensus upgrade** is any change that cannot be rolled out node-
by-node while the chain keeps making progress safely — i.e. a CometBFT
major/minor, a state-machine-breaking change, an AppHash-scheme change (§9.1), a
genesis/validator-set change, or a consensus-param change. Procedure:
1. Announce + freeze feature work; cut a release with the exact pins + SBOM.
2. Rehearse in staging (§14 full matrix + the specific change).
3. Schedule a maintenance window; **stop all validators at a known committed
   height `H`** with the chain quiesced.
4. If **consensus-compatible**: swap binary, restart; chain continues from `H+1`.
   If **state-breaking** (AppHash/validator-set/state format): perform the §10.5
   **chain migration** (state export → new genesis with the new chain-id) — never
   an in-place genesis edit (§10.6).
5. Verify AppHash continuity + `GetReceipt` on all validators; **rollback plan**
   (retain the old binary + Mongo/consensus backups; the quiesced height is the
   rollback point) is documented before the window.
A non-coordinated (patch) upgrade never changes consensus state and does not
require a full stop of the set.

## 18. Deployment requirements (Phase 3)

- **Validators:** exactly **4**, each on **independent infrastructure / failure
  domains** (not four processes on one host). Each holds its own **ed25519
  consensus key** (`priv_validator_key.json`) and **node key** (`node_key.json`),
  provisioned and rotatable independently (§4.5.3).
- **MongoDB:** each validator has its **own independent MongoDB Replica Set**
  instance (decision 7) — never shared. A **single-node replica set** is
  acceptable for transaction support but gives **no DB HA**; a multi-node replica
  set per validator is recommended for availability (R3/L5 recovery still applies
  if a validator's DB is lost).
- **Network / ports:** p2p `26656` reachable **only** by the 4 validators
  (firewall/security-group allowlist); RPC `26657` and ABCI `26658` bound to
  **loopback**, never exposed; **only the gRPC gate is public**. Enable
  `private_peer_ids` + `FilterPeers` allowlist, `pex` off, `addr_book_strict`
  on, per-IP/connection caps (§4.5).
- **Genesis:** `genesis.json` with the **chain-id**, 4 ed25519 validators (equal
  power), and `app_state` (seeded `sender_tails`, `FIRST_BLOCK_HASH`, proposer
  labels). The chain-id is part of the tx/TxID domain (§3.4).
- **Config consistency:** CometBFT `config.toml` and the size limits (§3.5) are
  **identical** across validators (limits layering OS2); uniform binary/toolchain
  (§17).
- **Observability:** the AppHash reconciliation monitor (§9.2) with alerting;
  tamper-evident forensic retention; per-validator Mongo backups; genesis/config in
  version control (no secrets).
- **Runbooks (operator):** genesis/bootstrap, coordinated chain migration for
  validator replacement (§10.5), legacy cutover (§12), divergence recovery (§9.2),
  and key rotation (§4.5.3).

## 19. Remaining risks (honest register)

- **f = 1 only.** With n = 4, loss/partition of **2** validators **halts** the
  chain (liveness), though safety holds. Availability is the main operational
  risk; a 5th/7th validator (future on-chain phase) would raise fault tolerance.
- **No DB HA at the single-node-RS baseline.** A validator's DB loss is recovered
  by state-sync + replay (R3), but until resynced that validator is unavailable;
  multi-node RS per validator reduces this.
- **Nondeterminism from toolchain/dependency skew** is the top *correctness*
  risk. Mitigated by exact uniform pins (§17), the AppHash monitor + fail-closed
  isolation (§9.2), and the handshake AppHash cross-check. A determinism bug still
  causes a **halt (liveness)**, not a fork (safety) — but it is an outage.
- **No slashing in Phase 3.** Equivocation produces CometBFT evidence that is
  logged/surfaced, not economically penalized; deterrence is weak until a future
  on-chain/slashing phase.
- **Large self-contained contract txs** stress mempool/block limits and cap
  throughput (§3.5); the limits are tuned and tested, but very large payloads
  reduce TPS and increase propagation time.
- **Migration ambiguity.** Legacy senders with conflicting histories require
  manual resolution; a `no-continuity` choice leaves a deliberate continuity gap
  for that sender (auditable, §12).
- **State-sync trust.** A syncing node trusts a weak-subjectivity checkpoint /
  honest peers; malicious snapshots are rejected (L5), but a Byzantine majority
  could feed a *consistent-but-wrong* state to a node syncing from genesis without
  a recent checkpoint — mitigated by using a recent trusted checkpoint.
- **Config/genesis error** (wrong validator set, mismatched limits, wrong
  chain-id) causes a halt or, if partially adopted across operators, a **fork**;
  the genesis-hash guard (§12.4) and identical-config requirement (§18) reduce but
  do not eliminate this.
- **CometBFT version maintenance.** Staying on a supported v1.0.x patch is
  required; minor/major upgrades are breaking, planned migrations.

## Appendix A — Exact integration steps (implementation checklist)

> Nothing below is executed until the implementation plan is approved **and** the
> user explicitly authorizes **Phase 0**. Phase S (step 0) is a hard gate that must
> complete and be signed off first.

0. **Phase S — dependency & security review (gate before everything):**
   re-justify the CometBFT pin and all dependency pins against current stable
   releases, upstream advisories, fixes, and API/protocol/Go compatibility; review
   all direct + critical transitive dependencies; run vulnerability scanning
   (`govulncheck`/OSV/Trivy); produce and commit the SBOM/inventory; write the
   upgrade + security-patch + coordinated-consensus-upgrade policy (§17). **Exit:
   signed report + final justified pins + committed SBOM + CI scanning.**
1. **Dependencies:** add the Phase-S-confirmed CometBFT pin to `go.mod`; remove
   `wazero` + `participle` (+ `ws`). Confirm `go build ./...` on the pinned Go.
2. **Consensus config & keys:** define `genesis.json` (4 ed25519 validators,
   equal power, `app_state` with `sender_tails`, `FIRST_BLOCK_HASH`, proposer
   labels); add `cmd/validatorkeygen` (validator key) and node-key provisioning
   (§4.5); CometBFT `config.toml` with the §4.5 p2p allowlist; reconcile
   `nm.NewNode`/`proxy.NewLocalClientCreator` to the v1.0.1 signatures.
3. **ABCI app (`consensus/app`):** implement the full v1.0.1 `Application`
   interface — at minimum `Info`, `InitChain`, `CheckTx`, `ProcessProposal`,
   `FinalizeBlock` (returns app_hash + empty validator updates), `Commit`
   (returns retain_height); plus empty `ExtendVote`/`VerifyVoteExtension` and the
   state-sync methods (`ListSnapshots`/`OfferSnapshot`/`LoadSnapshotChunk`/
   `ApplySnapshotChunk`) per §4.3/§6/§8/§9/§11. Unit-test each against an
   in-memory Mongo fake.
4. **Persistence changes (`db/`):** add `ConsensusMeta`, `Era`, `_consensus_state`
   docs, new indexes, unique `(senderUid,signature)` index, and the **atomic,
   watermark-last** `CommitApply` (§11.2). Keep legacy read paths.
5. **Gate (`protos/`):** add `SubmitTx`, `GetReceipt` (ordinary receipt,
   `Verifiable=false`); add `AddData` compat wrapper; add canonical tx
   encoding/TxID (§3.4); move the gossip fan-out off the request path; fix rate
   limiter ordering (§15.6); add `BlockValidator`-equivalent to acceptance.
6. **Remove `ws/` and BVM** execution surface; update `main.go` bootstrap
   (§5.2).
7. **Migration tool:** legacy validate/report/resolve/seal + tail export + genesis
   generation, with refuse-on-conflict (§12).
8. **Tests:** implement the full §14 matrix (S/L/B/R/D/M/N/P/C/V), including
   multi-node staging runs, AppHash-divergence detection (§9.2), and the
   crash-injection matrix R1–R6 (§11.4).
9. **Docs:** reconcile `README.md`; write the operator runbook for the §10.5
   chain-migration validator replacement and the §12 legacy cutover; update
   `docs/MIGRATION.md`.

## Appendix B — Sources for CometBFT facts (verified at tag v1.0.1)

All facts in this document were verified against the **v1.0.1** source (the
latest v1.0.x patch). Primary references:

- **Version / Go / ports** — release <https://github.com/cometbft/cometbft/releases/tag/v1.0.1>;
  `go.mod` (go 1.23.5) <https://github.com/cometbft/cometbft/blob/v1.0.1/go.mod>;
  default ports in `config/config.go` (P2P 26656, RPC 26657, ABCI 26658).
- **Commit ordering** (§11.1) — `internal/consensus/state.go` (`finalizeCommit`:
  SaveBlock → WAL → ApplyVerifiedBlock → updateToState) and
  `state/execution.go` (`applyBlock`: FinalizeBlock → SaveFinalizeBlockResponse →
  updateState → app.Commit → stateStore.Save(AppHash)):
  <https://github.com/cometbft/cometbft/blob/v1.0.1/internal/consensus/state.go>,
  <https://github.com/cometbft/cometbft/blob/v1.0.1/state/execution.go>.
- **Handshake / replay** (§11.3) — `internal/consensus/replay.go`
  (`ErrAppBlockHeightTooHigh`, app-hash-mismatch panic, `storeH==stateH+1`
  real-vs-mock replay, `ExecCommitBlock` re-calls FinalizeBlock+Commit):
  <https://github.com/cometbft/cometbft/blob/v1.0.1/internal/consensus/replay.go>.
- **AppHash validation** (§9.2) — `state/validation.go` `validateBlock`
  (`wrong Block.Header.AppHash ... Check ABCI app for non-determinism`; a per-node
  local check, not a cross-validator protocol):
  <https://github.com/cometbft/cometbft/blob/v1.0.1/state/validation.go>.
- **ABCI 2.0 interface** (§4.3) — `abci/types/application.go` (full `Application`
  interface) and `proto/cometbft/abci/v1/types.proto` (`FinalizeBlockResponse`
  carries `app_hash` + `validator_updates`; `CommitResponse` carries only
  `retain_height`):
  <https://github.com/cometbft/cometbft/blob/v1.0.1/abci/types/application.go>,
  <https://github.com/cometbft/cometbft/blob/v1.0.1/proto/cometbft/abci/v1/types.proto>.
- **P2P keys & config** (§4.5) — `p2p/key.go` (node key, node ID =
  `hex(pubkey.Address())`), `privval/file.go` (validator key, `SignVote`),
  `p2p/conn/secret_connection.go` (ed25519 + X25519 + ChaCha20-Poly1305;
  authenticated ≠ authorized), peer config fields in `config/config.go`
  (`persistent_peers`, `private_peer_ids`, `unconditional_peer_ids`,
  `addr_book_strict`, `pex`, `max_num_inbound_peers`/`max_num_outbound_peers`).

Supporting background: CometBFT docs (built-in Go app; ABCI 2.0 `FinalizeBlock`/
`Commit` model; v1.0 versioning guarantees incl. PBTS). v0.40.0 exists on a
parallel line and is **not** used. (The consensus package is `internal/consensus/`
in v1.0.x; re-verify any pinned patch if upgrading.)

## Appendix C — Canonical encoding & AppHash golden test vectors

Golden vectors are **normative cross-implementation tests**. They are generated
by a reference implementation, committed to the repo, and asserted by ≥ 2
independent implementations (Go is one; a second language is required if
multi-language clients are supported — acceptance C1, §14.9).

**Vector file format** (`testdata/vectors.json`, one object per vector):

```json
{
  "name": "simple-record-01",
  "canonicalTxHex": "<hex of version ‖ 0x00 ‖ canon(fields)>",
  "txId": "<hex SHA256( blvchain/v2/txid ‖ 0x00 ‖ canonicalTx )>",
  "blockData": { "...": "the logical typed input, for reconstruction" },
  "expectedRecordId": "<hex, from BlockHashMaker>",
  "note": "field order and integer width are fixed by §3.4"
}
```

**AppHash vectors** (`testdata/apphash_vectors.json`):

```json
{
  "name": "height-0-genesis",
  "H": 0,
  "prevAppHashHex": "<64 zeros>",
  "recordIds": [],
  "senderTails": [{"senderUid":"...","recordId":"..."}],
  "appHash": "<hex AppHash[0]>"
}
```

**Vectors that MUST be present (minimum set):**
- Canonical tx: empty-optional-fields record; a record with a contract payload;
  a record with a max-size `Data`; a record whose signature is high-S (rejected);
  a record with an unknown field (rejected — C2).
- AppHash: genesis (`H=0`, empty record set, seeded sender-tails); an empty block
  at `H>0` (exercises the empty Merkle root constant); a block with one record; a
  block with an odd number of records (exercises the Merkle split rule).
- The **empty Merkle root** constant is the fixed value `SHA256("")` =
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
  (RFC 6962 empty-tree root), used for empty blocks and the empty sender-tail map.

Vectors are **locked**: any change to the encoding, field order, Merkle rules, or
domain tags changes the vectors and is a breaking change requiring a version
bump of the `"BLVT1"`/AppHash domain.

## Appendix D — Merkle construction (normative)

Both `RecordIDsRoot` and `SenderTailsRoot` use the **same** deterministic Merkle
tree, an **RFC 6962 (Certificate Transparency) tree** with explicit domain
separation:

- **Leaf hash:** `leaf(x) = SHA256(0x00 ‖ x)`
- **Internal node:** `node(l, r) = SHA256(0x01 ‖ l ‖ r)`
- **Empty tree:** `root([]) = SHA256("")` (the fixed constant above).
- **Single leaf:** `root([x]) = leaf(x)`.
- **n > 1 (split rule, no odd-node promotion):** let `k` = the **largest power of
  two strictly less than `n`**; `root = node( root(leaves[0:k]), root(leaves[k:n]) )`.
  This splits every time at a power-of-two boundary, so **no node ever has a
  single child** and there is no "odd node" ambiguity — the tree is fully
  determined by the leaf sequence and `n`.

Domain separation between the leaf and internal levels (`0x00` vs `0x01`) makes
second-preimage attacks across levels infeasible and keeps the two roots
unambiguous. The outer AppHash preimage is additionally tagged
`"blvchain/v2/apphash"` (§9.1), so an AppHash can never collide with a tx ID or
Record ID.

**`RecordIDsRoot(block H)`:** the leaves are the 32-byte Record IDs of the block's
records, in `(TxIndex)` ascending order. Empty block ⇒ the empty-root constant.

**`SenderTailsRoot(after H)`:** the leaves are `leaf( EncodeFields(senderUid,
recordId) )` over **all** sender-tail entries, sorted by `senderUid` (ascending,
compared as raw bytes of the 32-hex UID). Empty map (only possible pre-first-tx,
not after genesis seeding if any legacy sender exists) ⇒ the empty-root constant.

**Cross-implementation requirement:** the leaf ordering, the `k` split rule, the
domain-separation bytes, and the empty-root constant are fixed by this appendix
and locked by the AppHash golden vectors (Appendix C). Acceptance test S2
(§14.1) asserts all validators compute identical roots, and C1 asserts identical
results across implementations.