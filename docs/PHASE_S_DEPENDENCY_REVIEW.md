# Phase S — Dependency & Security Review (deliverable)

> **Status:** Phase S COMPLETE — awaiting review. This is a review/analysis
> artifact. **No source code, business logic, database, `go.mod`, or `go.sum`
> was modified.** Dependency *upgrades* are **out of Phase S scope** and are
> deferred to Phase 0; this report *selects and justifies* the versions Phase 0
> will apply. Scope authorized: CometBFT/advisory review, final version + Go
> selection, dependency review, SBOM, CI scanning, and policy documentation.

## 0. Executive summary (decisions)

1. **CometBFT: the design's `v1.0.1` pin is DISQUALIFIED.** The v1.x line is
   unmaintained (no `v1.0.2`) and is **not** patched for the **Critical**
   consensus advisory **CSA-2026-001 "Tachyon"** (BFT-Time inflation). Selected
   replacement: **`v0.40.0`** (recommended; current, security-maintained,
   Tachyon-fixed, ABCI 2.0), with **`v0.38.26`** as the conservative fallback.
   This changes a design-level assumption and **requires your approval** (§11).
2. **Go toolchain: `1.26.11`** (current supported patch). The installed `1.26.6`
   is affected by 8 reachable stdlib vulnerabilities fixed in 1.26.9–1.26.11.
3. **The current dependency set has 20 reachable vulnerabilities** (govulncheck),
   incl. a **gRPC authorization bypass** and a **mongo-driver OOB read**. All
   have upstream fixes; Phase 0 must adopt the secure versions in §6.
4. **SBOM generated** (CycloneDX 1.5) and **CI vuln-scanning added**
   (`.github/workflows/security.yml`: govulncheck + OSV-Scanner + Trivy + Syft).
5. **Removed in Phase 0** (justified here): `gorilla/websocket` (unmaintained),
   `participle` (unused), `wazero` (BVM removed).

## 1. Method & artifacts

- Scanners available locally: Go 1.26.6, Python 3.14. `govulncheck` installed via
  `go install golang.org/x/vuln/cmd/govulncheck@latest` and run against the
  module. OSV-Scanner/Syft/Trivy are wired in CI (not installed locally).
- Artifacts (committed under `docs/phase-s/`):
  - `sbom.cyclonedx.json` — CycloneDX 1.5 SBOM (67 components, go.sum-pinned).
  - `DEPENDENCIES.md` — human-readable inventory.
  - `govulncheck.txt` — full reachability-aware scan output.
  - `gen_sbom.py`, `modules.json` — reproducible SBOM generation.
- Upstream advisory/version facts sourced from CometBFT releases/CHANGELOG/GHSA,
  the Go vuln DB (pkg.go.dev/vuln), and endoflife.date (cited inline).

## 2. CometBFT version selection

### 2.1 Candidate assessment

| Candidate | Date | Status | Security posture | Verdict |
|---|---|---|---|---|
| **v1.0.1** (design baseline) | 2025-02-03 | Last v1.x; **no v1.0.2**; branch inactive since 2025-06; pkg.go.dev still pins it | Tachyon status **UNRESOLVED** (advisory prose "all versions" vs metadata ranges that omit v1.0.1; no v1.x fix; exploitability not established) — see §2.2 | **Excluded** (unresolved Critical + unmaintained) |
| **v0.40.0** | 2026-07-27 | Current release line; ABCI 2.0; 2026.1 / SDK-v0.55 family (new deployments) | Includes Tachyon + ASA fixes; adds the v0.38.26-absent items (proposer vote-ext self-verify #5204, blocksync deadlock fixes #5931/#5839, RPC Host/JSON-RPC hardening #5921/#5861) + mldsa65, secp256k1eth; full list in §2.5 | **Selected** |
| **v0.38.26** | 2026-08-13 | Latest v0.38.x; widely deployed (2025.1 / SDK-v0.53); Go 1.22.11 | v0.38 line backported core hardening in .22–.25 (blocksync full verification, MsgBytesFilter #5946, double-sign off-by-one #5668, sig-count #5860); **.26 itself is mldsa65 only**; see §2.5 | Conservative fallback |

### 2.2 Why v1.0.1 is disqualified (Critical)

**CSA-2026-001 "Tachyon" (GHSA-c32p-wcqj-j677, Critical, 2026-01-23):** a BFT-Time
consensus flaw — inconsistent commit-signature verification vs block-time
derivation lets a faulty process arbitrarily inflate block time. BLVChain **derives
`BlockMeta.TimeStamp` from CometBFT block time** (consensus-critical, §9/§11), so
this is directly material. Full source-verified analysis in
`docs/COMET_BFT_V0_40_0_COMPAT.md`; summary separating fact from inference:

- ✅ **VERIFIED:** advisory prose says **"All versions of CometBFT"** affected,
  **no workarounds**; patches shipped for **v0.38.21 / v0.37.18 / main** only;
  **no v1.0.x patch exists** (v1.0.1 is the final v1.x release, branch dead since
  2025-06). The fix code (validator-address-mismatch check + `MedianTime`
  error-return) is **present in v0.40.0, absent in v1.0.1**.
- ⚠️ **METADATA NUANCE (verified):** the GHSA *structured ranges* are `<= v0.38.20`
  and `<= v0.37.17` — they do **not** list v1.0.1, so **automated scanners will NOT
  flag v1.0.1**. The disqualification is a **human security judgment** (prose "all
  versions" + fix-code analysis), not a scanner hit — which is exactly why a manual
  review was required.
- ⚠️ **UNRESOLVED (not asserted vulnerable):** whether v1.0.1 is *actually
  exploitable* for Tachyon is **NOT established**. The prose says "all versions"
  and the vulnerable MedianTime/verifyCommit code path present in v0.38.20 appears
  to be present in v1.0.1, but (a) the GHSA ranges do not list v1.0.1 and (b) no
  source-level exploitability analysis proving block-time inflation on the v1.0.x
  code path has been performed here. Per the review instruction, v1.0.1's Tachyon
  status is therefore recorded as **UNRESOLVED**, not "definitively vulnerable."

**Conclusion:** v1.0.1 is **excluded from production selection** — not on a claim
of confirmed vulnerability, but because (1) its Tachyon status is **unresolved** on
a Critical consensus issue affecting block-time (which BLVChain relies on), and an
unresolved Critical is itself disqualifying for a *new* production design; and
(2) it is on a line that is **officially unsupported**. **Terminology (precise):**
"retracted/unsupported" is used **only with attribution** — **Cosmos Labs** (the
production-ecosystem steward) officially declares "CometBFT v1.x is not supported;
that release line was retracted" (docs.cosmos.network release-family + security
policy; cosmos.network "Retracting CometBFT v1 and v2" 2026 roadmap). This is **not**
a Go `retract` directive (none exists for v1.x — the go.mod retract block lists only
old `v0.38.x`) and **not** a CometBFT-repo announcement: the CometBFT `v1.x`
README still lists v1.x as **"pre-release."** Either ground alone excludes it;
together they are decisive. (If a source-level exploitability analysis later *clears*
v1.0.1, ground (2) still excludes it.)

### 2.3 Justification for v0.40.0 (recommended) — corrected with per-version evidence

My earlier "v0.38.26 mostly lacks security hardening" was **wrong and is withdrawn.**
A source-level changelog review (§2.5) shows the v0.38.x line **did** backport the
core security-relevant fixes in v0.38.22–v0.38.25 — blocksync full commit
verification (#5663/#5711/#5753), `MsgBytesFilter` heap-amplification fix (#5946),
blocksync sender/signature-count validation (#5860), the `double_sign_check_height`
off-by-one (#5668), filter `ReadTag` int32 truncation (#5948), nil-vote handling
(#5777) — none of which carry a new advisory ID but are real hardening. **v0.38.26
itself is a feature-only release** (just the ML-DSA-65 backport #6028).

So the choice is **close**, and the honest differentiators that remain in v0.40.0
and are **not** in v0.38.26 (code-verified, §2.5):

- **Blocksync robustness / liveness:** `AddBlock` deadlock fix (#5931),
  `removeTimedoutPeers` deadlock (#5839), `Enable()` + atomic `numPending`
  (#5888/#5889) — relevant to any node that syncs.
- **Consensus robustness:** proposer **self-verifies its own vote extension** before
  precommitting (#5204) — turns a stall into a clean `CONSENSUS FAILURE` halt.
- **RPC hardening:** request `Host` HTML-escaping (#5921), JSON-RPC numeric-ID
  validation (#5861) — **low relevance to BLVChain**, whose CometBFT RPC is bound
  to loopback and never client-facing (§4.5).
- **Forward support runway:** v0.40.0 is the 2026.1 / SDK-v0.55 line recommended
  for **new** deployments; v0.38.x (2025.1) is maintained but older and will EOL
  sooner.

**Recommendation: v0.40.0**, because BLVChain is a **new** deployment (so v0.40.0's
state/API-breaking nature — e.g. `MedianTime` Nil/Absent, #5901 — carries no
migration penalty) and benefits from the blocksync-deadlock and proposer-self-
verify fixes plus the longest forward support. **v0.38.26 is a fully security-
hardened conservative fallback** (core fixes present, correct version string
"0.38.26", smaller dep surface, Go 1.22.11); choose it only if maximum battle-
testing is weighted above the above deltas. This is a genuine close call — both are
defensible; the final call is yours (Phase 0 gate).

> **Already reconciled:** the architecture's CometBFT internals (public `consensus/`
> package, `proto/tendermint/abci`, `nm.NewNodeWithContext`, `Request*`/`Response*`
> ABCI types, no PBTS, large default block params) have been verified against
> **v0.40.0** and the arch doc updated (§4/§11/App B). If **v0.38.26** is chosen
> instead, re-verify those same points against the v0.38.26 tag (notably: v0.38.26
> also has no PBTS; default params differ; ABCI proto is `proto/tendermint/abci`).

### 2.5 Per-version factual comparison (v0.38.22 → v0.40.0), source-referenced

Sources: `CHANGELOG.md` + `go.mod` at each tag, GitHub release pages, and tagged
source (raw.githubusercontent.com/cometbft/cometbft/<tag>/...). No advisory IDs are
attached to any of these entries; the only published advisories are ASA-2023/024/
025 and CSA-2026-001 (Tachyon, patched in v0.38.21).

| Version (release) | Security-relevant | Consensus | Blocksync | Mempool | ABCI / RPC |
|---|---|---|---|---|---|
| **v0.38.22** (2026-04-10) | (inherits Tachyon fix from v0.38.21) | light-client-attack evidence validation (#5638, #5757) | **full commit verification** instead of light (#5663/#5711/#5753) | — | — |
| **v0.38.23** (2026-05-08) | — | nil-vote handling (#5777) | `maxPeerHeight` poisoning guard (#5803) | — | ABCI socket unlock/panic fix (#5593); light witness comparison (#5820) |
| **v0.38.24** | *tag only, skipped — no release* | — | — | — | — |
| **v0.38.25** (2026-07-27) | **`MsgBytesFilter` heap-amplification fix (#5946)**; filter `ReadTag` int32 (#5948) | `double_sign_check_height=1` off-by-one (#5668) | sender + sig-count validation (#5860); late `BlockResponse` tolerance (#5959) | `setRecheckFull`/`setDone` race (#5837) | `EventBusBufferCapacity` (#5849) |
| **v0.38.26** (2026-08-13) | — (feature only) | — | — | — | **mldsa65 backport only** (#6028) |
| **v0.40.0** (2026-07-27) | **RPC Host HTML-escape (#5921); JSON-RPC ID validation (#5861)**; MsgBytesFilter; ReadTag int32 | **proposer vote-ext self-verify (#5204)**; double_sign off-by-one; `MedianTime` skips Nil+Absent (**state-breaking #5901**) | AddBlock deadlock (#5931); removeTimedoutPeers deadlock (#5839); Enable/atomic numPending (#5888/#5889); sig-count (#5860) | setRecheckFull race (#5837); AppReactor framing (#5956) | **ABCI app-mempool `InsertTx`/`ReapTxs` socket fix (#5958)**; callback re-entrancy deadlock (#5850); pre-bound listener (#5904) |

**Adjudicating the examples I previously mis-stated:**
- `MsgBytesFilter` heap-amplification — **IS backported** (v0.38.25, #5946). Not a gap.
- `double_sign_check_height` off-by-one — **IS backported** (v0.38.25, #5668). Not a gap.
- blocksync sender/signature validation — **IS backported** (v0.38.25, #5860). Not a gap.
- RPC `Host` escaping (#5921) & JSON-RPC ID validation (#5861) — **NOT backported** (v0.40.0 only) — but low relevance (loopback RPC).
- Proposer vote-extension self-verification (#5204) — **NOT backported** (v0.40.0 only).
- Blocksync `AddBlock`/`removeTimedoutPeers` deadlocks (#5931/#5839) — **NOT backported** (v0.40.0 only).

`go.mod` Go directives: v0.38.22–.26 = **`go 1.22.11`**; v0.40.0 = **`go 1.25.0`**.
(v0.40.0's binary still misreports `TMCoreSemVer = "0.39.0"` — see §4/item 4.)

## 3. Go toolchain selection

- **Selected: Go `1.26.11`** (latest 1.26 patch, 2026-11-10; supported line).
  Justification: the installed `1.26.6` is affected by **8 reachable stdlib
  vulnerabilities** (net/http, crypto/x509, crypto/tls, net/textproto,
  html/template) fixed across 1.26.9–1.26.11 (see §5); 1.26.11 is the superset.
  1.25 is EOL and 1.27 is newer than needed; 1.26.x is the conservative current
  choice and satisfies CometBFT v0.40.0 (built on the Go 1.25 line) and v0.38.x
  (Go 1.22+).
- **Pinning:** set the module `go`/`toolchain` directive to `1.26.11` and build
  with a pinned builder image in CI; record the toolchain+deps hash per build
  (determinism, arch §17.4). *(Applying this to `go.mod` is a Phase 0 action.)*

## 4. Direct & critical transitive dependencies

Direct dependencies (`go.mod`) and their review disposition:

| Module | Current | Selected (Phase 0) | Disposition |
|---|---|---|---|
| `google.golang.org/grpc` | v1.69.0 | **v1.83.1** | Upgrade — 4 reachable vulns (authz bypass, server panic, OOM) |
| `go.mongodb.org/mongo-driver` | v1.17.6 | **v1.17.7** | Upgrade — GSSAPI heap OOB read |
| `google.golang.org/protobuf` | v1.35.2 | keep / align with grpc | Not flagged reachable; bump if grpc requires |
| `github.com/joho/godotenv` | v1.5.1 | keep | Config loader; no advisory |
| `github.com/BurntSushi/toml` | v1.4.0 | keep | Config; no advisory |
| `github.com/natefinch/lumberjack` | v2.0.0 | keep | Log rotation; no advisory |
| `golang.org/x/time` | v0.14.0 | keep / bump with x/* | Rate limiting; not flagged |
| `github.com/gorilla/websocket` | v1.5.3 | **REMOVE** | Unmaintained (archived); `ws/` removed in Phase 3 |
| `github.com/alecthomas/participle/v2` | v2.1.4 | **REMOVE** | Unused (no non-test callers) |
| `github.com/tetratelabs/wazero` | v1.9.0 | **REMOVE** | BVM execution surface removed |
| *(new)* `CometBFT` | — | **v0.40.0** | Consensus engine (Phase 0) |

Critical **transitive** dependencies (pulled by grpc/mongo-driver) that govulncheck
flags as reachable and that Phase 0 must pin to at least:

| Transitive | Current | Minimum (secure) | Why |
|---|---|---|---|
| `golang.org/x/net` | v0.43.0 | **v0.60.0** | 5 reachable HTTP/2 vulns (HPACK race, flow-control, CPU, malformed framing, trailer OOM) |
| `golang.org/x/text` | v0.29.0 | **v0.39.0** | Infinite loop on invalid input (via mongo norm) |
| `github.com/klauspost/compress` | v1.17.11 | **v1.20.2** | Panic on two-block frames |

> Bumping grpc/mongo-driver will transitively pull newer `golang.org/x/*`,
> `google.golang.org/genproto`, etc.; Phase 0 MUST run full build + `go test` +
> the §14 regression to confirm compatibility. These upgrades are **not** performed
> in Phase S (scope boundary).

## 5. Vulnerability scan results (govulncheck, reachability-aware)

`govulncheck ./...` against the **current** module (Go 1.26.6) reports **20
reachable vulnerabilities** (i.e. called on a code path), plus non-reachable ones
in required modules. All 20 have upstream fixes; none are false positives
(govulncheck confirms reachability). **Disposition for all 20: fix by upgrade,
deferred to Phase 0** (§4/§6 give the secure versions). Full traces:
`docs/phase-s/govulncheck.txt`.

### 5.1 Go standard library (Go 1.26.6 → fix in 1.26.9–1.26.11)

| ID | Area | Fixed in |
|---|---|---|
| GO-2026-6859 | net/http | go1.26.11 |
| GO-2026-6809 | crypto/x509 (eMudhra CA denylist, CVE-2026-111943) | go1.26.10 |
| GO-2026-6613 | net/http | go1.26.9 |
| GO-2026-6608 | net/textproto | go1.26.9 |
| GO-2026-6607 | crypto/tls | go1.26.9 |
| GO-2026-6605 | net/http | go1.26.9 |
| GO-2026-6600 | html/template | go1.26.9 |
| GO-2026-6599 | html/template | go1.26.9 |
| **→ resolution:** toolchain **go1.26.11** |

### 5.2 Third-party modules (reachable)

| ID | Module | Issue | Found → Fixed |
|---|---|---|---|
| GO-2026-4762 | google.golang.org/grpc | **Authorization bypass** via missing leading slash in `:path` | v1.69.0 → **v1.79.3** |
| GO-2026-6061 | google.golang.org/grpc | xDS RBAC / HTTP2 | v1.69.0 → **v1.82.1** |
| GO-2026-6443 | google.golang.org/grpc | Server panic via missing authority/Host headers | v1.69.0 → **v1.82.2** |
| GO-2026-6348 | google.golang.org/grpc | Heap OOM via HTTP/2 DATA fragmentation | v1.69.0 → **v1.83.1** |
| GO-2026-5327 | go.mongodb.org/mongo-driver | Heap OOB read in GSSAPI error handling | v1.17.6 → **v1.17.7** |
| GO-2026-5970 | golang.org/x/text | Infinite loop on invalid input | v0.29.0 → **v0.39.0** |
| GO-2026-6617 | golang.org/x/net | HTTP/2 HPACK encoder race (crash) | v0.43.0 → **v0.60.0** |
| GO-2026-6612 | golang.org/x/net | HTTP/2 double flow-control refund | v0.43.0 → **v0.60.0** |
| GO-2026-6611 | golang.org/x/net | HTTP/2 excessive CPU (window churn) | v0.43.0 → **v0.60.0** |
| GO-2026-6610 | golang.org/x/net | HTTP/2 malformed framing headers | v0.43.0 → **v0.60.0** |
| GO-2026-6603 | golang.org/x/net | HTTP/2 trailer-header memory exhaustion | v0.43.0 → **v0.60.0** |
| GO-2026-6865 | github.com/klauspost/compress | Panic on two-block frames | v1.17.11 → **v1.20.2** |

**Highest-priority for BLVChain:** the **gRPC authorization bypass (GO-2026-4762)**
and **server-panic/OOM** issues (GO-2026-6443/6348), because the **gRPC gate is the
client-facing server**; and the **mongo-driver OOB read** on the DB connect path.
These reinforce upgrading grpc→v1.83.1 and mongo-driver→v1.17.7 in Phase 0.

> **Note (out of scope, flagged):** a few reachable stdlib net/http findings sit
> on code paths used by gRPC's HTTP/2 transport; the toolchain bump to 1.26.11 +
> x/net v0.60.0 + grpc v1.83.1 clears them together.

## 6. CometBFT upstream advisory review (disposition)

| Advisory | Severity | Affects design's v1.0.1? | Disposition |
|---|---|---|---|
| ASA-2025-001 (blocksync peer disruption) | Low→Informational | Yes (fixed only in v1.0.1) | Addressed by v1.0.1; also fixed in v0.38.17+/v0.40.0 |
| ASA-2025-002 (peer stall via block parts) | High | Yes (fixed only in v1.0.1) | Addressed by v1.0.1; also fixed in v0.38.17+/v0.40.0 |
| ASA-2025-003 (BitArray → network halt) | High | Metadata gap (not listed) | Fixed v0.38.19+; **v0.40.0/v0.38.26 include it** |
| **CSA-2026-001 "Tachyon" (BFT-Time)** | **Critical** | **UNRESOLVED** (prose "all versions"; metadata ranges omit v1.0.1; no v1.x fix; exploitability not established) | v0.40.0 / v0.38.26 include the fix; v1.0.1 status unresolved → excluded |

Conclusion: the selected **v0.40.0** (or v0.38.26) contains the Tachyon fix; the
design's v1.0.1 has an **unresolved** Tachyon status (not a confirmed vuln) and is
excluded (see §2.2).

## 7. SBOM and CI

- **SBOM:** `docs/phase-s/sbom.cyclonedx.json` (CycloneDX 1.5, 67 components with
  go.sum hashes) + `docs/phase-s/DEPENDENCIES.md`. Regenerate with
  `python -I docs/phase-s/gen_sbom.py` (after `go list -m -json all > modules.json`).
- **CI:** `.github/workflows/security.yml` — on push/PR/weekly/manual:
  1. **govulncheck** (gating, reachability-aware) on the pinned Go.
  2. **OSV-Scanner** on `go.mod` / the SBOM.
  3. **Syft SBOM** (CycloneDX + SPDX) uploaded as artifacts.
  4. **Trivy** filesystem+config scan, **failing on High/Critical** (reviewed
     exceptions only, per arch §17.3).
  CI uses the pinned `GO_VERSION` (1.26.11); keep in sync with `go.mod` `toolchain`.

## 8. Upgrade / security-patch / coordinated-upgrade policy

Documented normatively in `docs/ARCHITECTURE_PHASE3.md` §17.3–§17.5:
- **Patch** (non-breaking): routine, rolling restart.
- **Minor** (Go-API-breaking): planned maintenance.
- **Major / consensus- or state-breaking** = **coordinated consensus upgrade**:
  announce → rehearse in staging → quiesce all validators at height `H` → binary
  swap (consensus-compatible) or §10.5 chain migration (state-breaking) → verify
  AppHash continuity + rollback plan. Security-patch SLA: Critical/High reachable →
  triaged within a defined SLA with an out-of-band release path; SBOM + CI scanning
  make "are we affected?" fast to answer.

## 9. Dependencies removed in Phase 0 (justified here)

- `github.com/gorilla/websocket` v1.5.3 — **archived/unmaintained** upstream and
  the entire `ws/` layer is removed by the CometBFT design (ordering is now
  CometBFT's). Removing it also removes an unmaintained network dependency.
- `github.com/alecthomas/participle/v2` v2.1.4 — no non-test callers (leftover from
  the deleted Rust/grammar front end).
- `github.com/tetratelabs/wazero` v1.9.0 — the BVM WASM host; Phase 3 is
  storage-only (no WASM execution), so it is dead code.

## 10. Remaining risks

- **grpc v1.69 → v1.83.1 and x/net v0.43 → v0.60.0 are large jumps**; behavioral/
  API compatibility with mongo-driver/protobuf must be validated by full build +
  tests in Phase 0. (Mitigation: Phase 0 gate; pin exact versions.)
- **CometBFT v0.40.0 is a 0.x minor** (API-breaking model); the ABCI/consensus
  internals in the design were verified against v1.0.1 and need re-verification
  (§2.3).
- **govulncheck coverage is Go-only**; OSV/Trivy in CI add coverage for
  non-Go/OS-layer issues but are not yet run locally (no scanners installed here).
- Advisories can appear after this review; the weekly CI scan + feed monitoring
  (§8) is the ongoing control.

## 11. Unresolved decisions (need your approval before Phase 0)

1. **CometBFT version change v1.0.1 → v0.40.0** (recommended) vs v0.38.26
   (conservative). **Source-verified** (see `docs/COMET_BFT_V0_40_0_COMPAT.md`):
   v0.40.0 keeps the ABCI 2.0 FinalizeBlock/Commit/handshake/AppHash/P2P semantics
   the BLVChain design relies on, but **drops PBTS** (v0.40.0 uses BFT-Time via
   unconditional `MedianTime`), renames ABCI types (`Request*`/`Response*`, proto
   `proto/tendermint/abci`, `consensus/` public), and changes `node.NewNode`
   (no `ctx`). Approve the change **and** the follow-on **documentation-only**
   corrections to §4.1/§4.2/§4.3/§9.2/§11/App B enumerated in the compat report
   §C (the state-machine/ABCI *design* is unaffected). A PBTS-based design would
   instead require staying on a PBTS-capable line (v0.39/v1.x) — flag if PBTS is a
   requirement.
2. **Go toolchain → 1.26.11** and the **dependency upgrade set** in §4/§6
   (grpc v1.83.1, mongo-driver v1.17.7, x/net v0.60.0, x/text v0.39.0,
   klauspost/compress v1.20.2). Confirm these are acceptable to apply in Phase 0.
3. **Removal of gorilla/websocket, participle, wazero** in Phase 0 (§9).
4. **CI provider/executors:** the workflow assumes GitHub Actions + the named
   actions; confirm that is the intended CI (else I will port to the actual CI).

---

# Clarification pass (Rev 2, 2026-11-23)

This pass resolves the review's follow-up items 2–4 (CometBFT internals are in
the companion `COMET_BFT_V0_40_0_COMPAT.md` and summarized in §16).

## 12. Go toolchain — 1.26.11 vs 1.27.4 (reassessed with test evidence)

**Verified facts:**
- The CometBFT-v0.40.0-inclusive resolved graph (§13) needs go **≥ 1.26.0**
  (grpc v1.85.0, x/net v0.60.0, x/text v0.42.0, x/crypto v0.57.0); CometBFT
  v0.40.0 needs go 1.25.0. So 1.26.x is the floor; 1.27 is not required.
- **Both toolchains were actually installed and run here** (via `GOTOOLCHAIN`
  auto-download; dl.google.com was blocked but the module proxy served both):
  BLVChain `go build`/`vet`/`test ./...` pass under **both** go1.26.11 and go1.27.4
  (all 9 packages `ok`), and CometBFT v0.40.0 `go install` succeeds under **both**.
- **govulncheck:** the latest published version is **v1.10.0** (no newer). It
  runs on go1.26.11 (stdlib clean) but **fails on go1.27.4** — its vendored
  `x/tools`/`go-types` cannot parse the go1.27 stdlib (`math/rand/v2/rand.go:
  generic method requires go1.27 or later`). Rebuilding govulncheck with go1.27.4
  did **not** help (the limitation is in govulncheck's bundled parser, not the
  build toolchain).

**Options considered:**
- **(a) Go 1.26.11 + govulncheck v1.10.0** — works end-to-end; the vuln scan runs
  on the *same* toolchain that ships, so stdlib results are accurate.
- **(b) Go 1.27.4 + updated govulncheck** — **not available**: no govulncheck
  release (v1.10.0 is latest) parses go1.27 yet.
- **(c) Split CI jobs (build/test on 1.27.4, vuln-scan on 1.26.11)** — possible for
  third-party/module scanning (OSV-Scanner/Trivy are version-range based and don't
  care about the toolchain), **but NOT sound for stdlib analysis**: reachability-
  aware stdlib results depend on the Go version that *builds/ships* the binary.
  Scanning 1.27-built code with a 1.26 toolchain would mis-report stdlib vulns
  (false-clear 1.26-fixed issues, miss 1.27-introduced ones). So a split is only
  acceptable if the scan job also builds with 1.26 — i.e. you still ship 1.26.

**Recommendation: Go `1.26.11`** — **not** merely because "v1.10.0 can't parse
1.27," but because (1) accurate stdlib vulnerability analysis requires the vuln
scanner to support the **shipping** toolchain, and the only reachability-aware Go
scanner in our CI (govulncheck) does not yet support go1.27; (2) 1.26.11
builds/vets/tests everything and builds CometBFT v0.40.0; (3) it is the more
field-proven line. **Reassess when govulncheck (or an equivalent) supports go1.27**
— at that point 1.27.4 (which builds/tests clean) becomes equally valid.
**Recorded tooling limitation:** govulncheck v1.10.0 lacks go1.27 stdlib support.
Final exact pin: `go 1.26.11` / `toolchain go1.26.11`.

## 13. Dependencies — exact pins + resolved graph + conflicts (item 3)

All `>=` constraints from Rev 1 are replaced with **exact** versions below. The
CometBFT-inclusive graph was resolved in a **throwaway scratch module** (outside
this repo — `go.mod`/`go.sum` here untouched) via
`go get cometbft@v0.40.0 grpc@v1.85.0 mongo-driver@v1.17.10 klauspost/compress@v1.20.2`.
Full resolved list (221 modules): `docs/phase-s/resolved-graph-candidate.txt`.

**Final exact pins (direct):**
`cometbft v0.40.0`, `grpc v1.85.0`, `protobuf v1.36.12`, `mongo-driver v1.17.10`,
`klauspost/compress v1.20.2`, `joho/godotenv v1.5.1`, `BurntSushi/toml v1.4.0`,
`natefinch/lumberjack v2.0.0+incompatible`, `golang.org/x/time v0.14.0`.

**Final exact pins (critical transitive, MVS-resolved):** `x/net v0.60.0`,
`x/text v0.42.0`, `x/crypto v0.57.0`, `x/sys v0.48.0`, `x/sync v0.23.0`,
`genproto/googleapis/{api,rpc} v0.0.0-20260817212433-ac3dfec99bb1`,
`cometbft-db v0.14.1`, `cel.dev/expr v0.25.3`, `cosmos/gogoproto v1.7.2`.

**Each verified against advisories:** grpc v1.85.0 ≥ all 4 grpc fixes; mongo-driver
v1.17.10 ≥ v1.17.7; x/net v0.60.0 = fix; x/text v0.42.0 ≥ v0.39.0;
klauspost/compress v1.20.2 = fix. (A final govulncheck/OSV re-scan of the *tidied*
CometBFT-inclusive graph is a Phase 0 gate — reachability needs the real code.)

**Unavoidable conflicts / required overrides (item 3):**
1. **`klauspost/compress`: MVS resolves a VULNERABLE v1.19.1** (pulled by
   mongo-driver); we MUST add an explicit `require github.com/klauspost/compress
   v1.20.2` to override MVS. This is the one unavoidable manual pin.
2. **`gorilla/websocket` cannot be removed from the graph** — CometBFT v0.40.0
   depends on it for its RPC WebSocket. We drop only our **direct** use (the `ws/`
   layer); it remains transitive. (Corrects the Rev-1 "REMOVE" wording.)
3. **`x/time`: project direct v0.14.0 > CometBFT's v0.12.0** → MVS keeps v0.14.0;
   no conflict.
4. **Large CometBFT surface:** v0.40.0 expands the graph to ~221 modules (libp2p
   crypto, pebble/badger, cosmos/gogoproto, go-git, etc.). `go mod tidy` with the
   real import set will prune tooling-only deps (tm-load-test, docker, gotty), but
   the core set remains a significant attack-surface increase — flagged as a risk.
   No hard version *conflicts* among grpc/protobuf/genproto/x-net (MVS resolved a
   single consistent set).

## 14. CI & SBOM hardening (item 4)

- **SBOM uses the actual resolved graph:** `go list -m -json all` is Go's resolved
  build list (MVS), so the SBOM reflects exactly what would be compiled.
- **Reproducibility verified:** regenerating produced a **byte-identical**
  `sbom.cyclonedx.json` (deterministic: sorted components, fixed timestamp).
- **Actions pinned to immutable commit SHAs** in `.github/workflows/security.yml`:
  checkout `11bd7190…` (v4.2.2), setup-go `40f1582b…` (v5.6.0), sbom-action
  `fffec3d8…` (v0.24.3), trivy-action `915b19bb…` (0.28.0). Scanner **tools pinned
  to exact versions**: govulncheck `v1.10.0`, osv-scanner `v1.9.2`.
- **Binary-level scanning added:** a `govulncheck -scan=bin` job (build → scan the
  artifact) plus a Trivy rootfs scan of the built binary.
- **Exact failure thresholds (documented in-workflow):**
  - govulncheck (source **and** binary): **any reachable vulnerability fails**.
  - OSV-Scanner (go.mod + SBOM): **any finding fails**.
  - Trivy (fs + binary): **HIGH or CRITICAL, `exit-code: 1`**, `ignore-unfixed:
    false` → fails on any High/Critical.
  - Exceptions require a reviewed, committed waiver (none currently).

## 15. CometBFT v1.0.1 → v0.40.0 internals (item 1)

See `docs/COMET_BFT_V0_40_0_COMPAT.md`: Tachyon applicability to v1.0.1 (verified
vs assumed), and re-verified v0.40.0 internals (ABCI 2.0, FinalizeBlock/Commit
ordering, handshake/replay, AppHash, node construction, P2P, state sync) with the
exact list of `ARCHITECTURE_PHASE3.md` sections needing correction.

---

## 16. What has and has NOT been tested (evidence boundary)

**Tested here (real results):**
- **BLVChain current code, current `go.mod`/`go.sum`** — `go build ./...`,
  `go vet ./...`, `go test ./...` (9 packages) **pass under go1.26.11 AND go1.27.4**.
- **CometBFT standalone** — `go install github.com/cometbft/cometbft/cmd/cometbft@v0.40.0`
   succeeds under **go1.26.11 and go1.27.4**; `@v0.38.26` under go1.26.11.
   Runtime version string confirmed (v0.40.0 → "0.39.0"; v0.38.26 → "0.38.26").
- **govulncheck on the CURRENT dependency set** (go1.26.11): stdlib clean; 11
   reachable third-party findings (the planned upgrades).

**NOT tested / NOT claimed validated:**
- **The proposed UPGRADED dependency matrix** (CometBFT v0.40.0 + grpc v1.85.0 +
   protobuf v1.36.12 + mongo-driver v1.17.10 + x/net v0.60.0 + x/text v0.42.0 +
   klauspost/compress v1.20.2, minus wazero/participle/direct-gorilla) **integrated
   with BLVChain code**. It was only **resolved as a module graph in a throwaway
   scratch module**; it has **not** been compiled against BLVChain, **not**
   `go test`-ed, **not** `-race`-tested, and **not** vuln-scanned as an integrated
   build. **The upgraded matrix is NOT validated.** Compatibility of these upgrades
   with BLVChain's code is unproven until Phase 0 applies and tests them.

### 16.1 Exact Phase-0 validation checklist (run after applying the upgrades)

1. Apply pins in `go.mod` (CometBFT v0.40.0; grpc v1.85.0; protobuf v1.36.12;
   mongo-driver v1.17.10; klauspost/compress v1.20.2; toolchain/go 1.26.11);
   remove wazero/participle/gorilla-direct; `go mod tidy`; commit `go.mod`/`go.sum`.
2. `go build ./...` (go1.26.11) — pass.
3. `go vet ./...` — pass.
4. `go test ./...` — all packages pass (existing tests, unchanged behavior).
5. `go test -race ./...` — on **Linux CI** (needs gcc; not runnable on the Windows
   dev host) — pass.
6. `govulncheck ./...` (go1.26.11) — **zero reachable** (confirm the 11 cleared).
7. `osv-scanner --lockfile=go.mod` + Trivy (fs) — no High/Critical (or reviewed
   exception).
8. **In-process CometBFT smoke test:** construct `nm.NewNodeWithContext(...,
   proxy.NewLocalClientCreator(app), ...)` with the real ABCI app; start a
   single-node devnet; produce ≥ 1 block; verify `Info`/`InitChain`/`FinalizeBlock`/
   `Commit` are invoked and the ABCI wiring compiles/runs.
9. Regenerate the SBOM; review the diff (new CometBFT transitive set).
10. Confirm `go version -m` of a built node shows `mod github.com/cometbft/cometbft
    v0.40.0` (the authoritative build identity, not the runtime string).

Items 1–10 are **dependency/build validation** (Phase 0). The full §14 consensus/
crash/migration acceptance matrix is Phase 1+, not part of Phase-0 dependency
validation.