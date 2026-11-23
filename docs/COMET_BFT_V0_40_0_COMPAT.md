# CometBFT v0.40.0 compatibility report (Phase S item 1)

> Source-verified against the **v0.40.0** and **v1.0.1** tags (raw.githubusercontent
> + GitHub API + a real v0.40.0 build on go1.26.6). Each claim is marked
> ✅ VERIFIED (seen in source/metadata/build) or ⚠️ INFERENCE. Read-only: no
> project files changed to produce this.

## Part A — Tachyon (CSA-2026-001 / GHSA-c32p-wcqj-j677) applicability to v1.0.1

### A.1 Verified facts
- ✅ Advisory: severity **critical**, published 2026-01-23, no CVE. Prose says
  **"Affected versions: All versions of CometBFT"**, **"Affected users: Validators
  and protocols relying on block timestamps"**, patches **v0.38.21 / v0.37.18 +
  main**, **"no effective workarounds."**
  <https://github.com/cometbft/cometbft/security/advisories/GHSA-c32p-wcqj-j677>
- ✅ **Structured metadata is NARROWER than the prose.** The `vulnerabilities`
  array lists exactly two ranges: **`<= v0.38.20` (patched v0.38.21)** and
  **`<= v0.37.17` (patched v0.37.18)**. **v1.0.1 satisfies NEITHER range.** No
  v1.0.x range and no v1.0.x patched version is present.
- ✅ **v1.0.1 is the final v1.x release** (2025-02-03). No v1.0.2 exists. The v1.x
  branch's last commit is 2025-06-25 (CI/Docker/Makefile only, 35 commits past
  v1.0.1, **none touch the Tachyon code**).
- ✅ **The fix is present in v0.40.0, absent in v1.0.1.** The fix (merge
  `c56d64ec`, 2026-01-23, in v0.38.20→v0.38.21) adds a **validator-address
  mismatch** check in `types/validation.go` and makes `MedianTime` **return an
  error** on an unknown validator (`state/state.go`) instead of silently skipping.
  v0.40.0 `types/validation.go` contains `"validator address mismatch"` and
  `state.MedianTime(...) (time.Time, error)`; **v1.0.1 contains neither** — its
  `Commit.MedianTime` returns a plain `time.Time` and keeps the old nil-guard.

### A.2 Verified vs. conservative inference
- **VERIFIED:** (1) the GHSA ranges do **not** list v1.0.1; (2) there is **no
  v1.0.x patch**; (3) the Tachyon fix code is **absent from v1.0.1** and **present
  in v0.40.0**.
- ⚠️ **INFERENCE (clearly conservative):** a v1.0.1 node is **affected in
  substance** by Tachyon and has **no official patched upgrade path on the v1.x
  line**. Reasoning: (a) the prose explicitly says "All versions" and warns
  block-timestamp users — and BLVChain derives `BlockMeta.TimeStamp` from CometBFT
  block time; (b) the vulnerable code path present in v0.38.20 is **also present in
  v1.0.1** (v1.0.1 branched after v0.38.0, before the fix), and the fix was only
  ever applied to 0.37/0.38/main — i.e., v1.0.1 is a **prose-affected,
  metadata-unlisted** version with no fix.
- ⚠️ **Important nuance (do not overstate):** because GHSA is *range-based*, an
  automated scanner (govulncheck/OSV) will **NOT flag v1.0.1** for Tachyon. The
  disqualification is therefore a **human security judgment** from the advisory
  prose + fix-code analysis, **not** a scanner hit. This is exactly why a
  non-automated review was required.

### A.3 Decision — v1.0.1 Tachyon status: UNRESOLVED; v1.0.1 excluded from production
Per the review instruction, v1.0.1 is **not** classified as definitively vulnerable:
a source-level exploitability analysis proving block-time inflation on the v1.0.x
code path was **not** performed, and the GHSA ranges do not list v1.0.1. Its Tachyon
status is therefore recorded as **UNRESOLVED**. v1.0.1 is nonetheless **excluded
from production selection** because (1) an unresolved Critical consensus issue
affecting block time — which BLVChain relies on — is itself disqualifying for a
new production design, and (2) v1.0.1 is on a line that is **officially
unsupported** (v1.0.1 is the final v1.x; branch inactive since 2025-06). **Terminology
(precise):** "retracted/unsupported" is used **with attribution** — **Cosmos Labs**
officially declares v1.x "retracted"/"not supported" (docs.cosmos.network + 2026
roadmap); this is **not** a Go `retract` directive (none for v1.x — only old
`v0.38.x`) and **not** a CometBFT-repo statement — the CometBFT `v1.x` README still
lists v1.x as **"pre-release."** Prefer **v0.40.0** (current, Tachyon-fixed, ABCI 2.0,
new-deployment line); **v0.38.26** is the conservative fallback (also post-Tachyon via
v0.38.21, Go 1.22.11). See §B for the cost of the move and §C for the architecture
corrections. (If a future exploitability analysis *clears* v1.0.1, ground (2)
still excludes it.)

## Part B — v0.40.0 internals vs v1.0.1 (re-verified)

What the BLVChain design relies on, and how v0.40.0 differs from the v1.0.1 basis:

| Concern | v1.0.1 (design basis) | v0.40.0 (target) | Impact |
|---|---|---|---|
| **ABCI 2.0 FinalizeBlock** | present; returns `app_hash` + `validator_updates`; `Commit` returns `retain_height` only | **identical semantics**; `ResponseFinalizeBlock{events,tx_results,validator_updates,consensus_param_updates,app_hash}`, `ResponseCommit{retain_height}` | ✅ None — design holds |
| **Commit ordering** | SaveBlock → WAL → FinalizeBlock → SaveFinalizeBlockResponse → updateState → app.Commit → stateStore.Save(AppHash) | **materially identical**; `SaveBlock` happens in `finalizeCommit` *before* `ApplyVerifiedBlock`/`FinalizeBlock` | ✅ §11 state machine holds (verify line refs only) |
| **Handshake/replay** | app-ahead→`ErrAppBlockHeightTooHigh`; behind→replay (re-calls FinalizeBlock+Commit); mismatch→panic; `storeH==stateH+1`→real-vs-mock | **functionally identical**, same error/panic strings | ✅ §11.3 holds |
| **AppHash validateBlock** | per-node `block.AppHash != state.AppHash` → reject (+ "Check ABCI app for non-determinism") | **same check** (error string drops the hint); adds `blockTimeTolerance`/`skipLastCommitVerification` opts | ✅ §9.2 detection model holds (per-node, not cross-node) |
| **node.NewNode** | `NewNode(ctx, cfg, pv, nodeKey, clientCreator, genesisProvider, dbProvider, metricsProvider, logger, opts...)` | **no `ctx` param**; `NewNodeWithContext(ctx, ...)` is the ctx variant; **no `CliParams`/`NewNodeWithCliParams`** | 🔧 §4.2 pseudocode: drop `ctx`, use `NewNodeWithContext` |
| **proxy.ClientCreator** | 4 methods (consensus/mempool/query/snapshot) | **1 method** `NewABCIClient()` | 🔧 only matters if custom creator; `NewLocalClientCreator` same |
| **ABCI type names** | `FinalizeBlockRequest`, `CommitResponse` (alias shim over `cometbft/api` module) | **`RequestFinalizeBlock`, `ResponseCommit`** (in-tree pb; `Request*`/`Response*` prefix); **no `cometbft/api` module** | 🔧 §4.3 naming |
| **ABCI proto path** | `proto/cometbft/abci/v1` | **`proto/tendermint/abci` only** | 🔧 Appendix B path |
| **Consensus package** | `internal/consensus/` (internal) | **`consensus/` (public)** | 🔧 §11 source refs |
| **Application methods** | 12 | **14** (adds `InsertTx`/`ReapTxs` for app-mempool) | ➕ optionally implement (Phase 3 uses default mempool → can stub) |
| **PBTS** | **has PBTS** (`Feature.PbtsEnabled`) | **no PBTS**; unconditional `MedianTime` equality; `block.Time==genesisTime` exact | ⚠️ design claim "PBTS" is wrong for v0.40.0 — see §C |
| **Default block params** | MaxBytes 4 MiB, MaxGas 10,000,000 | **MaxBytes ~53 MiB, MaxGas -1**; mempool `MaxTxBytes 4 MiB`, `MaxTxsBytes 1 GiB` | ⚠️ §3.5 limits should be expressed as *explicit* consensus params, not defaults |
| **key types** | ed25519, secp256k1 | adds **`mldsa65` (PQ), `secp256k1eth`** | ➕ info (ed25519 unchanged) |
| **P2P identity** | node key = hex(pubkey.Address()), ed25519; validator key separate; config fields identical | **identical** | ✅ §4.5 holds |
| **state sync** | 4 snapshot methods; app-hash verified vs light client | **same**; adds `max-snapshot-chunks` | ✅ L5 holds |
| **version constant** | `CMTSemVer="1.0.1"` | **`TMCoreSemVer="0.39.0"` (STALE — misreports!)** | ⚠️ ops/version-gating gotcha |
| **P2P protocol ver** | 9 (ABCI 2.1.0) | **8 (ABCI 2.0.0)** | ⚠️ not cross-version wire-compatible |
| **go directive** | go 1.23.5 | **go 1.25.0**, **builds on go1.26.6 (verified)** | ✅ Go 1.26.11 fine |

✅ **v0.40.0 builds on Go 1.26.6** — the agent compiled the full v0.40.0 tree
(`go build ./...` exit 0) and ran the binary. Go 1.27 not tested (toolchain
unavailable in that env) but go.mod `go 1.25.0` imposes only a floor; ⚠️ likely
fine, confirm in Phase 0.

## Part C — `ARCHITECTURE_PHASE3.md` sections requiring correction for v0.40.0

If v0.40.0 is approved, correct these (all were written against v1.0.1):
1. **Header / §1.1 / §4.1** — "PBTS" claim → **remove**; v0.40.0 has no PBTS
   (BFT-Time via unconditional `MedianTime`). Version → v0.40.0; Go directive 1.25.0.
2. **§4.2 (NewNode pseudocode)** — drop `ctx` from `NewNode`; use
   `NewNodeWithContext(ctx, ...)`; note `proxy.NewLocalClientCreator` unchanged,
   `ClientCreator` is 1-method.
3. **§4.3 (ABCI surface)** — type names `RequestFinalizeBlock`/`ResponseCommit`
   (not `...Request`/`...Response`); proto path `proto/tendermint/abci`; note the
   optional `InsertTx`/`ReapTxs` (default mempool → stub).
4. **§9.2 / §11** — source refs move `internal/consensus/` → `consensus/`; the
   AppHash `validateBlock` check is unchanged in substance (error string drops the
   "non-determinism" hint).
5. **§3.5 (size limits)** — v0.40.0 default block params are large (MaxBytes
   ~53 MiB, MaxGas -1); set **explicit** consensus params, do not rely on defaults.
6. **§4.4 / §4.5 / App B** — ports unchanged (26656/26657/26658); P2P identity
   unchanged; note libp2p transport exists but is **not** used; ABCI proto path +
   `TMCoreSemVer` staleness noted in App B.
7. **§10.4 (BFT assumption)** — CometBFT evidence handling unchanged; note no
   slashing (unchanged).

> These are **documentation** corrections only. They do **not** change the BLVChain
> ABCI/state-machine/crash-recovery **design** (§7–§12 stand as written), because
> the FinalizeBlock/Commit/handshake/AppHash/P2P semantics are materially identical
> between v1.0.1 and v0.40.0 (Part B). Applying them is a pre-Phase-0 design-doc
> update, pending your approval of the v0.40.0 selection.