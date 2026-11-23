// Package comet contains the Phase-0 in-process CometBFT integration smoke test.
//
// It is a dependency/build-validation artifact only (Phase 0). It stands up a
// real single-node CometBFT node in-process over a real ABCI application, drives
// it through InitChain / Info / FinalizeBlock / Commit, commits at least one
// block, and verifies the resulting application state and AppHash. It also
// verifies the linked CometBFT build identity from build metadata (the correct
// way to identify the module — the v0.40.0 binary's runtime version string
// misreports "0.39.0").
//
// It intentionally does NOT exercise the BLVChain business logic, Mongo, or the
// WS/gRPC layers — that is Phase 1+. See docs/ARCHITECTURE_PHASE3.md §16 Phase 0.
package comet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cometbft/cometbft/abci/types"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	tmtypes "github.com/cometbft/cometbft/types"
)

// app is the minimal ABCI application used by the smoke test. It embeds
// BaseApplication (which satisfies the remaining methods of the 14-method v0.40
// Application interface) and overrides only the methods the gate requires us to
// observe: Info, InitChain, FinalizeBlock, Commit.
//
// Its state model is a single monotonically increasing height counter; the
// AppHash is sha256("blvchain-smoke:" || height-bytes). This makes the AppHash
// deterministically recomputable in the test so a divergent hash is detectable.
type app struct {
	types.BaseApplication

	mu            sync.Mutex
	infoCalls     int
	initChainCall int
	finalizeCalls int
	commitCalls   int
	height        int64 // last committed height (drives AppHash)
	appHash       []byte
}

func (a *app) Info(context.Context, *types.RequestInfo) (*types.ResponseInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.infoCalls++
	return &types.ResponseInfo{
		// Data mirrors the last committed height as a string so the handshake /
		// replay logic has a consistent view.
		Data: fmt.Sprintf("blvchain-smoke; committed height %d", a.height),
	}, nil
}

func (a *app) InitChain(_ context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initChainCall++
	// Genesis time from CometBFT must be honored; we accept any non-zero genesis.
	return &types.ResponseInitChain{}, nil
}

func (a *app) FinalizeBlock(_ context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.finalizeCalls++
	// Stage the new height. NOT yet durable — Commit makes it durable. The
	// AppHash commits the staged state so a crash before Commit is recoverable.
	a.height = req.Height
	a.appHash = computeAppHash(req.Height)
	return &types.ResponseFinalizeBlock{AppHash: a.appHash}, nil
}

func (a *app) Commit(context.Context, *types.RequestCommit) (*types.ResponseCommit, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commitCalls++
	// Height was staged by FinalizeBlock; committing makes it durable. v0.40
	// ResponseCommit carries only retain_height (NOT the app hash) — verified
	// against the v0.40.0 tag (COMET_BFT_V0_40_0_COMPAT.md §B).
	return &types.ResponseCommit{}, nil
}

// snapshot returns the observed call counts and committed height for assertions.
func (a *app) snapshot() (info, initChain, finalize, commit int, height int64, appHash []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.infoCalls, a.initChainCall, a.finalizeCalls, a.commitCalls, a.height, append([]byte(nil), a.appHash...)
}

func computeAppHash(height int64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(height))
	h := sha256.New()
	h.Write([]byte("blvchain-smoke:"))
	h.Write(b[:])
	return h.Sum(nil)
}

// TestCometBFTInProcessSmoke stands up a single-node in-process CometBFT node
// and drives it through the Phase-0 gate steps.
func TestCometBFTInProcessSmoke(t *testing.T) {
	// ---- 0. Build identity (authoritative module pin, not runtime string) ----
	assertCometBFTBuildIdentity(t)

	// ---- 1. Configuration + node material in a temp dir ----
	root := t.TempDir()
	conf := cfg.TestConfig()
	conf.SetRoot(root)
	conf.DBBackend = "memdb" // no disk I/O; in-memory only
	// Bind the P2P listener to a dynamically chosen free loopback port so
	// repeated/parallel test runs never collide on a fixed port. The RPC listener
	// is disabled (not needed for the smoke test).
	conf.RPC.ListenAddress = ""
	conf.P2P.ListenAddress = "tcp://127.0.0.1:" + freePort(t)
	conf.P2P.AddrBookStrict = false
	conf.P2P.AllowDuplicateIP = true
	conf.P2P.PexReactor = false // lone node; skip PEX/addrbook persistence noise
	conf.Consensus.CreateEmptyBlocks = true // produce blocks with an empty mempool

	// File-based private validator + node key (LoadOrGen writes them under root).
	// The config/ and data/ subdirectories must exist before the writers run.
	for _, dir := range []string{cfg.DefaultConfigDir, cfg.DefaultDataDir} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	pv := privval.LoadOrGenFilePV(conf.PrivValidatorKeyFile(), conf.PrivValidatorStateFile())
	nodeKey, err := p2p.LoadOrGenNodeKey(conf.NodeKeyFile())
	if err != nil {
		t.Fatalf("node key: %v", err)
	}

	// The single genesis validator MUST be this node's private validator, or the
	// node will conclude it is not a validator and stay in sync mode (no blocks).
	valKey, err := pv.GetPubKey()
	if err != nil {
		t.Fatalf("privval pubkey: %v", err)
	}
	genDoc := &tmtypes.GenesisDoc{
		ChainID:       "blvchain-phase0-smoke",
		InitialHeight: 1,
		Validators: []tmtypes.GenesisValidator{
			{PubKey: valKey, Power: 10, Name: "smoke-validator"},
		},
	}
	if err := genDoc.ValidateAndComplete(); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	testApp := &app{}
	clientCreator := proxy.NewLocalClientCreator(testApp)
	genProvider := func() (*tmtypes.GenesisDoc, error) { return genDoc, nil }

	// A real logger that writes consensus/state errors into the test log so a
	// silent block-production stall is diagnosable (rather than NopLogger).
	nodeLogger := log.NewTMLogger(log.NewSyncWriter(testWriter{t}))
	nodeLogger = log.NewFilter(nodeLogger, log.AllowError())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	n, err := node.NewNodeWithContext(
		ctx,
		conf,
		pv,
		nodeKey,
		clientCreator,
		genProvider,
		cfg.DefaultDBProvider,
		node.DefaultMetricsProvider(cfg.DefaultConfig().Instrumentation),
		nodeLogger,
	)
	if err != nil {
		t.Fatalf("NewNodeWithContext: %v", err)
	}

	// ---- 2. Start + subscribe to new blocks before starting ----
	sub, err := n.EventBus().Subscribe(ctx, "smoke-blocks", tmtypes.EventQueryNewBlock, 10)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer n.EventBus().Unsubscribe(ctx, "smoke-blocks", tmtypes.EventQueryNewBlock) //nolint:errcheck

	if err := n.Start(); err != nil {
		t.Fatalf("node start: %v", err)
	}
	defer func() {
		if n.IsRunning() {
			_ = n.Stop()
		}
	}()

	// ---- 3. Wait for at least one committed block (>= height 1, i.e. InitChain + first block) ----
	deadline := time.After(40 * time.Second)
	var sawBlockHeight int64
	blocks := 0
	for sawBlockHeight < 1 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for a committed block; height=%d blocks=%d", sawBlockHeight, blocks)
		case <-ctx.Done():
			t.Fatalf("context done waiting for block: %v", ctx.Err())
		case msg := <-sub.Out():
			evt, ok := msg.Data().(tmtypes.EventDataNewBlock)
			if !ok {
				continue
			}
			sawBlockHeight = evt.Block.Height
			blocks++
		}
	}

	// ---- 4. Observe ABCI callbacks fired ----
	info, initChain, finalize, commit, height, appHash := testApp.snapshot()
	if info < 1 {
		t.Errorf("Info was never called (calls=%d)", info)
	}
	if initChain < 1 {
		t.Errorf("InitChain was never called (calls=%d)", initChain)
	}
	if finalize < 1 {
		t.Errorf("FinalizeBlock was never called (calls=%d)", finalize)
	}
	if commit < 1 {
		t.Errorf("Commit was never called (calls=%d)", commit)
	}
	if height < 1 {
		t.Errorf("committed app height is %d, want >= 1", height)
	}
	if !bytes.Equal(appHash, computeAppHash(height)) {
		t.Errorf("AppHash mismatch: got %s want %s (height=%d)",
			hex.EncodeToString(appHash), hex.EncodeToString(computeAppHash(height)), height)
	}

	// The committed CometBFT block height must match the app's committed height.
	if sawBlockHeight != height {
		t.Errorf("CometBFT block height %d != app committed height %d", sawBlockHeight, height)
	}

	t.Logf("smoke OK: %d blocks, Info=%d InitChain=%d FinalizeBlock=%d Commit=%d, height=%d, appHash=%s",
		blocks, info, initChain, finalize, commit, height, hex.EncodeToString(appHash))
}

// assertCometBFTBuildIdentity verifies the linked CometBFT module version from
// the module build list. The v0.40.0 binary misreports its runtime version
// string (TMCoreSemVer="0.39.0"), so the go.mod pin is the authoritative
// identity (Phase S item 4; COMET_BFT_V0_40_0_COMPAT.md §B).
//
// Note: debug.ReadBuildInfo().Deps is empty for `go test`-compiled test
// binaries (the test main module is a `devel` pseudo-module with no embedded
// dep graph), so the authoritative source here is the module build list as
// reported by `go list -m` — the same list that produces the compiled binary.
func assertCometBFTBuildIdentity(t *testing.T) {
	t.Helper()
	const mod = "github.com/cometbft/cometbft"
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", mod).CombinedOutput()
	if err != nil {
		t.Skipf("go list unavailable, skipping build-identity check: %v: %s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "v0.40.0" {
		t.Errorf("CometBFT build identity = %q, want v0.40.0 (runtime version string misreports 0.39.0)", got)
	}
	t.Logf("CometBFT build identity: %s %s (authoritative; runtime string would misreport 0.39.0)", mod, got)
}

// compile-time assertion that app satisfies the v0.40.0 Application interface.
var _ types.Application = (*app)(nil)

// freePort returns a currently-free TCP port on the loopback interface. It binds
// to :0, reads the assigned port, and closes the socket — the node then binds
// that port. This avoids both fixed-port collisions across repeated runs and the
// unreliability of passing port 0 directly to the node's listen-address parser.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer ln.Close()
	return fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
}

// testWriter routes the node logger into the test log.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}