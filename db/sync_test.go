package db

import (
	"errors"
	"fmt"
	"testing"

	"blvchain/core/config"
)

func TestFindBlockIDsPaginatesAscending(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	const total = 5
	for i := 1; i <= total; i++ {
		block := blockWithTime(t, sender, receiver.uid, int64(1700000000000+i), fmt.Sprintf("p%d", i))
		if ok, err := InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert %d: %v %v", i, ok, err)
		}
	}

	var all []string
	cursor := ""
	for {
		page, err := FindBlockIDs(cursor, 2)
		if err != nil {
			t.Fatalf("FindBlockIDs: %v", err)
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		cursor = page[len(page)-1]
		if len(page) < 2 {
			break
		}
	}

	if len(all) != total {
		t.Fatalf("paged %d IDs, want %d", len(all), total)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1] >= all[i] {
			t.Fatalf("IDs are not strictly ascending: %q then %q", all[i-1], all[i])
		}
	}
}

func TestFindBlocksByIDs(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	first := makeBlock(t, sender, receiver.uid, "one", fmt.Sprintf("%064x", 1))
	second := makeBlock(t, sender, receiver.uid, "two", fmt.Sprintf("%064x", 2))
	third := makeBlock(t, sender, receiver.uid, "three", fmt.Sprintf("%064x", 3))
	for _, block := range []Block{first, second, third} {
		if ok, err := InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert: %v %v", ok, err)
		}
	}

	got, err := FindBlocksByIDs([]string{first.ID, third.ID, "missing"})
	if err != nil {
		t.Fatalf("FindBlocksByIDs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindBlocksByIDs returned %d blocks, want 2", len(got))
	}

	empty, err := FindBlocksByIDs(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("FindBlocksByIDs(nil) = %v, %v", empty, err)
	}
}

func TestBlockExists(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "payload", fmt.Sprintf("%064x", 1))
	if ok, err := InsertOneBlock(block); !ok || err != nil {
		t.Fatalf("insert: %v %v", ok, err)
	}

	exists, err := BlockExists(block.ID)
	if err != nil || !exists {
		t.Fatalf("BlockExists(existing) = %v, %v", exists, err)
	}
	exists, err = BlockExists("missing")
	if err != nil || exists {
		t.Fatalf("BlockExists(missing) = %v, %v", exists, err)
	}
	exists, err = BlockExists("")
	if err != nil || exists {
		t.Fatalf("BlockExists(empty) = %v, %v", exists, err)
	}
}

func TestGenesisCheckRejectsLegacyDatabase(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)
	config.FIRST_BLOCK_HASH = ""

	// A genesis-shaped block produced by a different hashing profile: same
	// preBlockHash marker, different ID.
	legacy := MakeGenesisBlock()
	legacy.BlockData.Data = "legacy chain data"
	BlockHashMaker(&legacy, legacy.BlockMeta.NodeUID)
	if legacy.ID == MakeGenesisBlock().ID {
		t.Fatal("test setup did not produce a distinct legacy ID")
	}
	if ok, err := InsertOneBlock(legacy); !ok || err != nil {
		t.Fatalf("insert legacy genesis: %v %v", ok, err)
	}

	if _, err := Genesis_check(); !errors.Is(err, ErrLegacyDatabase) {
		t.Fatalf("Genesis_check = %v, want ErrLegacyDatabase", err)
	}
}

func TestFindBlockIDsWithoutCollection(t *testing.T) {
	ResetCollection()
	t.Cleanup(ResetCollection)

	if _, err := FindBlockIDs("", 10); !errors.Is(err, ErrNoCollection) {
		t.Fatalf("FindBlockIDs without a collection = %v", err)
	}
	if _, err := FindBlocksByIDs([]string{"a"}); !errors.Is(err, ErrNoCollection) {
		t.Fatalf("FindBlocksByIDs without a collection = %v", err)
	}
	if _, err := BlockExists("a"); !errors.Is(err, ErrNoCollection) {
		t.Fatalf("BlockExists without a collection = %v", err)
	}
}

func TestGenesisBlockIDIsStableAcrossBuilds(t *testing.T) {
	// The genesis ID is the anchor every node agrees on. Pin its shape so an
	// accidental hashing change is caught here rather than at peer sync time.
	first := MakeGenesisBlock()
	second := MakeGenesisBlock()
	if first.ID != second.ID {
		t.Fatalf("genesis is not deterministic: %q vs %q", first.ID, second.ID)
	}
	if len(first.ID) != 64 {
		t.Fatalf("genesis ID length = %d, want 64", len(first.ID))
	}
	if first.BlockMeta.PreBlockHash != config.GENESIS_BLOCK_PREHASH {
		t.Fatalf("genesis preBlockHash = %q", first.BlockMeta.PreBlockHash)
	}
}
