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
| **v1.0.1** (design baseline) | 2025-02-03 | Last v1.x; **no v1.0.2**; line unmaintained; pkg.go.dev still pins it | **NOT patched for CSA-2026-001 (Critical)**; no v1.x fix exists | **Disqualified** |
| **v0.40.0** | 2026-07-27 | Current release line; ABCI 2.0; Cosmos "Ledger Security 2026.1" recommends it | Post-dates & includes Tachyon + ASA-2025-001/002/003 fixes; adds mldsa65, secp256k1eth, mempool/RPC hardening | **Selected (recommended)** |
| **v0.38.26** | 2026-08-13 | Latest v0.38.x; widely deployed; Go 1.22+ | Post-Tachyon (via v0.38.21) + all prior fixes; mldsa65 backport | Conservative fallback |

### 2.2 Why v1.0.1 is disqualified (Critical)

**CSA-2026-001 "Tachyon" (GHSA-c32p-wcqj-j677, Critical, 2026-01-23):** a BFT-Time
consensus flaw — inconsistent commit-signature verification vs block-time
derivation lets a faulty process arbitrarily inflate block time. The advisory
states **all versions are affected**; patches shipped for **v0.38.21 / v0.37.18 /
main** only, with **no v1.0.x patch** and no workaround. BLVChain **derives
`BlockMeta.TimeStamp` from CometBFT block time** (consensus-critical, §9/§11 of the
architecture), so this class of flaw is directly material. Running v1.0.1 in
production would mean a known **Critical, unpatched, consensus-level**
vulnerability on an unmaintained line. That is unacceptable for a production pin.

### 2.3 Justification for v0.40.0 (recommended)

- **Security-maintained:** on the active release line; includes the Tachyon,
  ASA-2025-001/002 (blocksync/peer stall), ASA-2025-003 (BitArray halt), and
  additional hardening (mempool `MsgBytesFilter` heap-amplification, RPC host
  escaping, `double_sign_check_height` off-by-one, blocksync validation, proposer
  vote-extension self-verification).
- **ABCI 2.0 compatible:** the architecture is written against ABCI 2.0
  (`FinalizeBlock` returning `app_hash` + validator updates; `Commit` returns
  `retain_height`) — present unchanged since v0.38 and in v0.40.0. The BLVChain
  ABCI design is therefore portable to v0.40.0.
- **Ecosystem alignment:** Cosmos SDK v0.55 / "Ledger Security 2026.1" ship and
  recommend v0.40.0 (or ≥ v0.38.26).
- **Fallback:** if the team wants the most battle-tested line, **v0.38.26** also
  satisfies all security requirements (post-Tachyon, Go 1.22+, ABCI 2.0).

> **Version-specific re-verification required (Phase 0, not Phase S):** the
> architecture's CometBFT internals (package path `internal/consensus` vs
> `consensus`, exact `finalizeCommit` ordering, `nm.NewNode` signature, default
> ports) were verified against **v1.0.1** and MUST be re-verified against the
> chosen tag (**v0.40.0** or v0.38.26) before Phase 0 implementation. This is a
> design-doc update, flagged as an unresolved decision (§11).

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
| **CSA-2026-001 "Tachyon" (BFT-Time)** | **Critical** | **Yes — NO v1.x fix** | **Disqualifies v1.0.1; v0.40.0 / v0.38.26 are fixed** |

Conclusion: the selected **v0.40.0** (or v0.38.26) clears **all** known CometBFT
advisories; the design's v1.0.1 does **not** clear the Critical one.

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
   (conservative). This invalidates v1.0.1-specific internals in the approved
   design (§4/§11/Appendix B) — approve the change and the follow-on design-doc
   re-verification against the chosen tag.
2. **Go toolchain → 1.26.11** and the **dependency upgrade set** in §4/§6
   (grpc v1.83.1, mongo-driver v1.17.7, x/net v0.60.0, x/text v0.39.0,
   klauspost/compress v1.20.2). Confirm these are acceptable to apply in Phase 0.
3. **Removal of gorilla/websocket, participle, wazero** in Phase 0 (§9).
4. **CI provider/executors:** the workflow assumes GitHub Actions + the named
   actions; confirm that is the intended CI (else I will port to the actual CI).