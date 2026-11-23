# Phase 0 — Dependency Upgrade, Integration Validation & CometBFT Smoke Test (deliverable)

> **Status:** Phase 0 implementation complete — **AWAITING GATE REVIEW**. This
> report documents exactly what was changed, the validation performed, the
> security scan results (with per-finding dispositions), the CometBFT in-process
> smoke-test evidence, and the pass/fail gate verdict. **No Phase 1 work was
> started.** Source of truth: `docs/ARCHITECTURE_PHASE3.md` (Rev 4/5, approved)
> and `docs/PHASE_S_DEPENDENCY_REVIEW.md` (Rev 2/3).

## 0. Executive summary

- **CometBFT v0.40.0**, **Go 1.26.11** applied (per Phase S final selection).
- All approved dependency upgrades applied; dependency graph tidied; **full
  build / vet / test green** across all packages including the new CometBFT
  smoke test.
- **In-process CometBFT smoke test implemented and passing** — real single-node
  node driven through `InitChain` → `Info` → `FinalizeBlock` → `Commit`,
  committing a block, with **AppHash verified** and **build identity confirmed**
  via `go list -m` (the v0.40.0 runtime string misreports "0.39.0").
- **govulncheck: 0 reachable vulnerabilities.** This is *not* claimed as "zero
  vulnerabilities": after the `x/net v0.60.0 → v0.62.0` security update, 1
  module-level finding remains (`x/crypto/openpgp`, GO-2026-5932, **not
  imported / not applicable**) plus CometBFT-libp2p `pion` findings surfaced by
  OSV/Trivy — all **not reachable** and **not on any BLVChain code path**. Each
  is itemized in §5 with its disposition.
- **Gate verdict: PASS** — all Phase-0 conditions met, including
  `go test -race ./...` (run locally with MinGW-W64 gcc 16.2.0, `CGO_ENABLED=1`;
  all packages pass, no data races). See §8.

## 1. Exact dependency changes (go.mod)

### 1.1 Direct dependencies

| Module | Before | After | Reason |
|---|---|---|---|
| `github.com/cometbft/cometbft` | — | **v0.40.0** | Consensus engine (Phase 0 gate requirement) |
| `google.golang.org/grpc` | v1.69.0 | **v1.85.0** | 4 reachable vulns (authz bypass, server panic, OOM) |
| `go.mongodb.org/mongo-driver` | v1.17.6 | **v1.17.10** | GSSAPI heap OOB read (≥ v1.17.7) |
| `google.golang.org/protobuf` | v1.35.2 | **v1.36.12** | Align with grpc v1.85.0 |
| `github.com/alecthomas/participle/v2` | v2.1.4 | **REMOVED** | Unused (no non-test callers) |
| `github.com/gorilla/websocket` | v1.5.3 (direct) | **kept** | Live `ws/` layer; also CometBFT transitive. **Not removed** (see §1.3) |
| `github.com/tetratelabs/wazero` | v1.9.0 | **kept** | Live `bvm/` VM package. **Not removed** (see §1.3) |
| `github.com/BurntSushi/toml` | v1.4.0 | v1.6.0 | Pulled by CometBFT v0.40.0 (transitive bump; no advisory) |
| `github.com/joho/godotenv`, `natefinch/lumberjack`, `golang.org/x/time` | unchanged | unchanged | No advisory |

### 1.2 Transitive (MVS-resolved) upgrades

| Module | Before | After |
|---|---|---|
| `golang.org/x/net` | v0.43.0 | **v0.62.0** (was v0.60.0; bumped to clear x/net/html DoS GO-2026-6860/6866 / CVE-2026-78661) |
| `golang.org/x/text` | v0.29.0 | **v0.43.0** (MVS minimum for x/net v0.62.0) |
| `golang.org/x/crypto` | v0.42.0 | **v0.58.0** (MVS minimum for x/net v0.62.0) |
| `golang.org/x/sys` | v0.36.0 | **v0.49.0** (MVS minimum for x/net v0.62.0) |
| `golang.org/x/sync` | v0.17.0 | **v0.24.0** (MVS minimum for x/net v0.62.0) |
| `golang.org/x/telemetry`, `golang.org/x/tools`, `golang.org/x/mod`, `golang.org/x/exp` | — | bumped transitively by x/net v0.62.0 (all **indirect**; not used by BLVChain code — CometBFT tooling only) |
| `github.com/klauspost/compress` | v1.17.11 | **v1.20.2** (explicit `require` overrides MVS's vulnerable v1.19.1 pulled by mongo-driver) |
| `google.golang.org/genproto/googleapis/rpc` | 2024 pseudo | **2026-08-17 pseudo** |

Go directive: `go 1.24.0` → **`go 1.26.11`** (+ `toolchain go1.26.11`).

### 1.3 Deliberate deviation from the Phase S "REMOVE" list

Phase S §9 recommended removing **wazero**, **participle**, and direct
**gorilla/websocket** in Phase 0. On inspection of the current tree:

- **`participle`** — genuinely unused (no `.go` file imports it). **Removed.**
- **`wazero`** — imported by the live `bvm/bvm.go` package, which **is BLVChain's
  VM** and is intentional production code (user-confirmed). Removing wazero would
  require deleting `bvm/` — an **architectural change outside Phase 0 scope**.
  **Kept.**
- **`gorilla/websocket`** — imported directly by the live `ws/` layer (the
  current P2P mesh). Removing it would require deleting `ws/` (a Phase-3
  architectural change). It also remains a CometBFT transitive. **Kept as a
  direct require.**

This deviation was **explicitly approved by the user** before any change was
made (Phase-0 scope forbids architectural changes; Phase S's assumption that
`bvm/` and `ws/` were already removed does not hold in this tree).

### 1.4 Graph growth

CometBFT v0.40.0 expands the module graph from ~67 to **389 SBOM components**
(~221 resolved modules), as predicted in Phase S §13. `go.sum`: 119 → 589 lines.
`go mod tidy` pruned tooling-only CometBFT deps; the core set (libp2p crypto,
pebble/badger, cosmos/gogoproto, etc.) remains a **significant attack-surface
increase** — flagged as a residual risk (§7).

## 2. Files changed

| File | Change |
|---|---|
| `go.mod` | Dependency upgrades + CometBFT v0.40.0 + participle removal + `go`/`toolchain` 1.26.11 |
| `go.sum` | Regenerated (589 lines) |
| `comet/smoke_test.go` | **NEW** — in-process CometBFT smoke test (Phase-0 gate) |
| `docs/phase-s/modules.json` | Regenerated from `go list -m -json all` (upgraded graph) |
| `docs/phase-s/sbom.cyclonedx.json` | Regenerated CycloneDX 1.5 SBOM (389 components) |
| `docs/phase-s/DEPENDENCIES.md` | Regenerated inventory |
| `docs/PHASE_0_REPORT.md` | **NEW** — this report |

(Pre-existing uncommitted doc edits to `ARCHITECTURE_PHASE3.md`,
`COMET_BFT_V0_40_0_COMPAT.md`, `PHASE_S_DEPENDENCY_REVIEW.md` were already in the
working tree before Phase 0 and are unchanged by this work.)

## 3. Validation results

| Check | Result |
|---|---|
| `go build ./...` (go1.26.11) | ✅ PASS |
| `go vet ./...` (go1.26.11) | ✅ PASS |
| `go test ./...` (go1.26.11) | ✅ PASS — all 10 packages (`core`, `bvm`, `comet`, `config`, `db`, `db/dbtest`, `logger`, `protos`, `utils`, `ws`) |
| `go test -race ./...` (go1.26.11) | ✅ **PASS** — run with `CC=gcc CGO_ENABLED=1` (MinGW-W64 UCRT gcc 16.2.0 at `C:\Dev\mingw64`); all 10 packages pass, no data races. |
| `govulncheck ./...` (go1.26.11) | ✅ **0 reachable** (see §5) |
| OSV-Scanner v1.9.2 (`--lockfile=go.mod`) | ⚠️ findings, all unreachable (see §5) |
| Trivy (fs, HIGH/CRITICAL) via Docker | ⚠️ **1 HIGH, 0 CRITICAL** — `x/net/html` DoS, unreachable (see §5) |
| SBOM regenerated & reviewed | ✅ 389 components; CometBFT v0.40.0 present; participle absent |
| Dependency graph review | ✅ `go mod tidy` clean; single MVS-consistent set; klauspost explicit pin required |

## 4. CometBFT in-process smoke test — evidence

`comet/smoke_test.go` (`TestCometBFTInProcessSmoke`) stands up a real
single-node CometBFT node in-process over a real ABCI application and asserts the
Phase-0 gate steps. Observed run (go1.26.11):

```
=== RUN   TestCometBFTInProcessSmoke
    CometBFT build identity: github.com/cometbft/cometbft v0.40.0
        (authoritative; runtime string would misreport 0.39.0)
    smoke OK: 1 blocks, Info=1 InitChain=1 FinalizeBlock=1 Commit=1,
        height=1, appHash=b337a3074a0b7f055b019edd1710bc078c7f92846d7ba236d5ceb4679571e795
--- PASS: TestCometBFTInProcessSmoke (0.22s)
```

Gate steps covered:

1. **Node initialization** — `node.NewNodeWithContext(ctx, cfg, pv, nodeKey,
   proxy.NewLocalClientCreator(app), genProvider, cfg.DefaultDBProvider, …)` on
   `cfg.TestConfig()` with `memdb` backend (no disk).
2. **InitChain** — called (count=1) during the ABCI handshake at genesis.
3. **Info** — called (count=1) during the handshake (app reports committed
   height).
4. **FinalizeBlock** — called (count=1); returns the staged `app_hash`.
5. **Commit** — called (count=1); `ResponseCommit` (v0.40.0 returns only
   `retain_height`, verified).
6. **≥ 1 committed block** — block height 1 committed (node entered height 2
   round 0; `txindex` indexed block 1).
7. **AppHash verified** — the test recomputes `sha256("blvchain-smoke:" ‖
   height)` independently and asserts equality with the app's reported AppHash;
   they match.
8. **CometBFT/App height agreement** — the committed CometBFT block height
   equals the app's committed height.

The test's single genesis validator **is the node's own private validator**
(otherwise CometBFT concludes "this node is not a validator" and stays in sync
mode — an early failure mode that was found and fixed during bring-up). The test
binds the P2P listener to a **dynamically chosen free loopback port** so repeated
or parallel runs never collide; it was verified stable across 5 consecutive runs.

### 4.1 Build-identity verification (not the runtime string)

Per Phase S item 4 and `COMET_BFT_V0_40_0_COMPAT.md`: the v0.40.0 binary's
`cometbft version` misreports `0.39.0` (stale `TMCoreSemVer`). The smoke test
therefore asserts identity via the **module build list** —
`go list -m -f '{{.Version}}' github.com/cometbft/cometbft` → `v0.40.0` — which
is the authoritative identity of the compiled module. (`debug.ReadBuildInfo()`
is empty for `go test` binaries, so `go list -m` is the correct source.)

## 5. Security scan results (per-finding disposition)

govulncheck is **reachability-aware** (symbol-level) and is the authoritative
"can our code actually reach this?" signal. OSV-Scanner and Trivy are
**version-range** scanners (no reachability) and therefore over-report; their
hits are reconciled below.

### 5.1 govulncheck (go1.26.11, `./...`)

**0 reachable vulnerabilities.** Symbol results clean; 1 module-level finding in
a required-but-unreached module (`x/crypto/openpgp`).

### 5.2 Findings, individually (after the x/net v0.62.0 update)

| # | ID / CVE | Module @ installed | Severity | Reachable? | Disposition |
|---|---|---|---|---|---|
| 1 | GO-2026-5932 | `golang.org/x/crypto@v0.58.0` | — | **No** — `x/crypto/openpgp` is **not imported** (mongo-driver pulls x/crypto but not openpgp) | **Not applicable.** The advisory is about `openpgp` (unmaintained/unsafe); BLVChain does not use it. |
| 2 | GO-2026-4479 (pion/dtls/v2) | `github.com/pion/dtls/v2@v2.2.12` | CVSS 5.9 (OSV) | **No** — pulled only by CometBFT's **libp2p** transport; BLVChain uses the **default Comet networking** transport (libp2p is opt-in, `P2P.LibP2PEnabled()`), and govulncheck confirms the vulnerable symbols are not called | **Present-but-unused.** Would matter only if libp2p were enabled (it is not). |
| 3 | GO-2025-3442 (pion/stun) | `github.com/pion/stun@v0.6.1` | CVSS 5.3 (OSV) | **No** — same libp2p-only provenance | **Present-but-unused** (same as #2). |
| 4 | GO-2026-6165 (pion/dtls/v3) | `github.com/pion/dtls/v3@v3.0.11` | CVSS 6.3 (OSV) | **No** — libp2p-only; v3 variant | **Present-but-unused** (same as #2). |
| 5 | GO-2026-6163 (pion/stun/v3) | `github.com/pion/stun/v3@v3.0.0` | CVSS 5.3 (OSV) | **No** — libp2p-only | **Present-but-unused** (same as #2). |

**Cleared by the x/net v0.60.0 → v0.62.0 security update (user-approved):**
GO-2026-6860 and GO-2026-6866 (both `x/net/html` DoS, CVE-2026-78661, Trivy
HIGH) — these are now **gone** (Trivy: 0 HIGH/CRITICAL; govulncheck no longer
lists them). `x/net` was bumped to v0.62.0, which also raised the MVS minimums
for x/text v0.43.0, x/crypto v0.58.0, x/sys v0.49.0, x/sync v0.24.0 (and the
indirect x/tools/x/telemetry/x/mod/x/exp). No unrelated direct dependency was
changed.

### 5.3 `openpgp` and `pion` residuals (not actionable by version bump)

- **`openpgp` (GO-2026-5932)** — `x/crypto/openpgp` is **never imported**; the
  advisory does not apply to BLVChain.
- **`pion/dtls` + `pion/stun`** — pulled only by CometBFT's **libp2p** transport.
  BLVChain uses CometBFT's **default Comet networking** (libp2p is opt-in via
  `P2P.LibP2PEnabled()` and is **not** enabled). These would matter only if
  libp2p were turned on, which the design does not do (validators-only Comet
  networking, §4.5.2). Tracked as an assumption to keep libp2p disabled.

### 5.4 Standard-library findings

**None.** The go1.26.6 → go1.26.11 toolchain bump cleared all 8 reachable stdlib
vulnerabilities (net/http, crypto/x509, crypto/tls, net/textproto, html/template)
identified in Phase S §5.1.

### 5.5 What "0 reachable" does and does not mean

- It **does** mean: no vulnerability's *vulnerable symbols* are called on any
  code path reachable from BLVChain's packages (govulncheck's guarantee).
- It **does not** mean "zero vulnerabilities": 5 module-level findings remain in
  the dependency graph (all unreached — §5.2). This report does **not** claim a
  clean bill of health; it enumerates each residual with its disposition. Trivy
  reports **0 HIGH/CRITICAL**.

## 6. Files changed — summary / git diff stat

```
 docs/ARCHITECTURE_PHASE3.md       | (pre-existing, unchanged by Phase 0)
 docs/COMET_BFT_V0_40_0_COMPAT.md  | (pre-existing, unchanged by Phase 0)
 docs/PHASE_S_DEPENDENCY_REVIEW.md | (pre-existing, unchanged by Phase 0)
 docs/phase-s/DEPENDENCIES.md      | 414 ++++   (regenerated)
 docs/phase-s/modules.json         | 3310 +++   (regenerated)
 docs/phase-s/sbom.cyclonedx.json  | 3691 +++   (regenerated)
 go.mod                            | 171 +++    (upgrades + cometbft + toolchain)
 go.sum                            | 576 +++    (regenerated)
 comet/smoke_test.go               | (new, untracked)
 docs/PHASE_0_REPORT.md            | (new, this file)
```

## 7. Remaining risks and blockers

1. **`-race` closed** — `go test -race ./...` now passes locally (MinGW-W64 gcc
   16.2.0). Still worth re-running on Linux CI for cross-platform coverage.
2. **CometBFT graph attack surface** — +~221 modules (libp2p, pebble/badger,
   cosmos/gogoproto, go-git, etc.). The libp2p transport (and its pion deps) is
   linked but **not used** (default is Comet networking); keep it that way.
3. **`openpgp` (GO-2026-5932)** — `x/crypto/openpgp` is not imported; advisory
   not applicable.
4. **`pion`/libp2p CVEs (unreachable)** — only matter if `P2P.LibP2PEnabled()`
   is ever set; BLVChain's design does not (validators-only Comet networking,
   §4.5.2). Document the assumption.
5. **OSV/Trivy local coverage gap** — OSV-Scanner ran locally (v1.9.2); Trivy
   ran via Docker. CI (`.github/workflows/security.yml`) is the durable control.
6. **Advisories post-date this review** — weekly CI scan is the ongoing control.

## 8. Gate verdict

**PASS** — the Phase-0 dependency gate passes on:
- ✅ All approved dependency upgrades applied and integrated, **including the
  user-approved `x/net v0.62.0` security update** (which cleared the two
  `x/net/html` HIGH findings).
- ✅ `go build` / `go vet` / `go test ./...` green (go1.26.11).
- ✅ **`go test -race ./...` green** (MinGW-W64 gcc 16.2.0, `CGO_ENABLED=1`;
  all 10 packages, no data races) — the final Phase-0 condition, now closed.
- ✅ In-process CometBFT v0.40.0 smoke test passes with verified AppHash + build
  identity.
- ✅ govulncheck: 0 reachable; every residual finding individually dispositioned
  (§5.2).
- ✅ **Trivy: 0 HIGH/CRITICAL.**
- ✅ SBOM + dependency docs regenerated and reviewed.

**No open Phase-0 conditions remain.** (Re-running `-race` on Linux CI remains
good practice for cross-platform coverage, but is no longer a gate blocker.)

**No Phase 1 work has been started.** I will wait for explicit approval before
proceeding.

## 9. Commit

Committed on branch **`phase-0/dependency-integration`** with message
`Phase 0: integrate CometBFT v0.40.0 and validate dependencies`. Only the
explicitly authorized Phase-0 files were staged (go.mod, go.sum, comet/, the
regenerated SBOM/inventory, and this report); pre-existing unrelated architecture
doc edits were **not** included. Commit hash recorded in the completion summary.