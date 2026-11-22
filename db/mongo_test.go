package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"blvchain/core/config"
	"blvchain/core/db/dbtest"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestCollection() *dbtest.Memory {
	store := dbtest.New()
	SetCollection(store)
	return store
}

func collectionCount(t *testing.T) int {
	t.Helper()

	store, ok := GetCollection().(*dbtest.Memory)
	if !ok {
		t.Fatalf("expected a *dbtest.Memory, got %T", GetCollection())
	}
	return store.Len()
}

// failingCollection fails every operation; it is used to prove that storage
// errors are propagated instead of being swallowed.
type failingCollection struct{}

func (failingCollection) InsertOne(context.Context, interface{}, ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	return nil, errors.New("boom insert one")
}

func (failingCollection) InsertMany(context.Context, []interface{}, ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	return nil, errors.New("boom insert many")
}

func (failingCollection) FindOne(context.Context, interface{}, ...*options.FindOneOptions) *mongo.SingleResult {
	return mongo.NewSingleResultFromDocument(nil, errors.New("boom find one"), nil)
}

func (failingCollection) Find(context.Context, interface{}, ...*options.FindOptions) (*mongo.Cursor, error) {
	return nil, errors.New("boom find")
}

func newFailingCollection() Collection {
	return failingCollection{}
}

// blockWithTime builds a block whose meta timestamp is fixed.
func blockWithTime(t *testing.T, sender identity, receiverUID string, timestamp int64, data string) Block {
	t.Helper()

	block := makeBlock(t, sender, receiverUID, data, fmt.Sprintf("%064x", timestamp))
	block.BlockMeta.TimeStamp = timestamp
	BlockHashMaker(&block, config.SELF_UID)
	return block
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestNoCollectionReturnsError(t *testing.T) {
	ResetCollection()
	t.Cleanup(ResetCollection)

	if _, err := InsertOneBlock(Block{}); !errors.Is(err, ErrNoCollection) {
		t.Errorf("InsertOneBlock without a collection = %v, want ErrNoCollection", err)
	}
	if _, err := InsertManyBlock([]interface{}{Block{}}); !errors.Is(err, ErrNoCollection) {
		t.Errorf("InsertManyBlock without a collection = %v, want ErrNoCollection", err)
	}
	if _, err := FindAllBlocks(bson.M{}); !errors.Is(err, ErrNoCollection) {
		t.Errorf("FindAllBlocks without a collection = %v, want ErrNoCollection", err)
	}
	if _, err := FindManyBlocksLimited(bson.M{}, 0, 10); !errors.Is(err, ErrNoCollection) {
		t.Errorf("FindManyBlocksLimited without a collection = %v, want ErrNoCollection", err)
	}
	if err := FindOneBlock("abc", &Block{}); !errors.Is(err, ErrNoCollection) {
		t.Errorf("FindOneBlock without a collection = %v, want ErrNoCollection", err)
	}
}

func TestInsertOneAndFindOne(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "payload", fmt.Sprintf("%064x", 1))

	ok, err := InsertOneBlock(block)
	if err != nil || !ok {
		t.Fatalf("InsertOneBlock = %v, %v", ok, err)
	}

	var found Block
	if err := FindOneBlock(block.ID, &found); err != nil {
		t.Fatalf("FindOneBlock: %v", err)
	}
	if found.ID != block.ID || found.BlockData.Data != "payload" {
		t.Fatalf("FindOneBlock returned %+v", found)
	}

	if err := FindOneBlock("does-not-exist", &found); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("FindOneBlock missing = %v, want mongo.ErrNoDocuments", err)
	}
}

func TestFindManyBlocksLimitedPaginates(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	const total = 5
	for i := 1; i <= total; i++ {
		block := blockWithTime(t, sender, receiver.uid, int64(1700000000000+i), fmt.Sprintf("payload-%d", i))
		if ok, err := InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert %d: %v %v", i, ok, err)
		}
	}

	// Newest first.
	page, err := FindManyBlocksLimited(bson.M{}, 0, 2)
	if err != nil {
		t.Fatalf("FindManyBlocksLimited: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("got %d blocks, want 2", len(page))
	}
	if page[0].BlockMeta.TimeStamp < page[1].BlockMeta.TimeStamp {
		t.Fatal("descending sort is not descending")
	}

	next, err := FindManyBlocksLimited(bson.M{}, 2, 2)
	if err != nil {
		t.Fatalf("FindManyBlocksLimited skip: %v", err)
	}
	if len(next) != 2 || next[0].ID == page[0].ID {
		t.Fatalf("skip did not advance the cursor: %+v", next)
	}

	// Oldest first.
	asc, err := FindManyBlocksLimitedASE(bson.M{}, 0, total)
	if err != nil {
		t.Fatalf("FindManyBlocksLimitedASE: %v", err)
	}
	if len(asc) != total {
		t.Fatalf("got %d blocks, want %d", len(asc), total)
	}
	if asc[0].BlockMeta.TimeStamp > asc[len(asc)-1].BlockMeta.TimeStamp {
		t.Fatal("ascending sort is not ascending")
	}
}

func TestFindManyBlocksEmptyIsNotAnError(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	// The old implementation returned mongo.ErrNoDocuments for an empty result,
	// which forced every caller into an awkward error branch.
	blocks, err := FindManyBlocksLimited(bson.M{"blockData.senderUid": "nobody"}, 0, 10)
	if err != nil {
		t.Fatalf("empty result returned an error: %v", err)
	}
	if len(blocks) != 0 {
		t.Fatalf("got %d blocks, want 0", len(blocks))
	}

	all, err := FindAllBlocks(bson.M{})
	if err != nil || len(all) != 0 {
		t.Fatalf("FindAllBlocks on an empty collection = %v, %v", all, err)
	}
}

func TestFindAllBlocksFilters(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	first := makeBlock(t, sender, receiver.uid, "first", fmt.Sprintf("%064x", 1))
	second := makeBlock(t, sender, receiver.uid, "second", fmt.Sprintf("%064x", 2))
	for _, block := range []Block{first, second} {
		if ok, err := InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert: %v %v", ok, err)
		}
	}

	bySender, err := FindAllBlocks(bson.M{"blockData.senderUid": sender.uid})
	if err != nil {
		t.Fatalf("FindAllBlocks: %v", err)
	}
	if len(bySender) != 2 {
		t.Fatalf("sender filter returned %d blocks, want 2", len(bySender))
	}

	byHash, err := FindAllBlocks(bson.M{"_id": first.ID})
	if err != nil {
		t.Fatalf("FindAllBlocks by id: %v", err)
	}
	if len(byHash) != 1 || byHash[0].ID != first.ID {
		t.Fatalf("hash filter returned %+v", byHash)
	}

	byPre, err := FindAllBlocks(bson.M{"blockMeta.preBlockHash": second.BlockMeta.PreBlockHash})
	if err != nil {
		t.Fatalf("FindAllBlocks by prehash: %v", err)
	}
	if len(byPre) != 1 || byPre[0].ID != second.ID {
		t.Fatalf("prehash filter returned %+v", byPre)
	}
}

func TestInsertManySkipsDuplicates(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	first := makeBlock(t, sender, receiver.uid, "one", fmt.Sprintf("%064x", 1))
	second := makeBlock(t, sender, receiver.uid, "two", fmt.Sprintf("%064x", 2))

	docs := []interface{}{first, second, first}
	ok, err := InsertManyBlock(docs)
	if err != nil || !ok {
		t.Fatalf("InsertManyBlock = %v, %v", ok, err)
	}

	if got := collectionCount(t); got != 2 {
		t.Fatalf("stored %d blocks, want 2 (the duplicate must be dropped)", got)
	}
}

func TestInsertManyEmptyIsNoop(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	ok, err := InsertManyBlock(nil)
	if err != nil || !ok {
		t.Fatalf("InsertManyBlock(nil) = %v, %v", ok, err)
	}
	if got := collectionCount(t); got != 0 {
		t.Fatalf("stored %d blocks, want 0", got)
	}
}

func TestFindManyBlocksLimitedRespectsLimitZero(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	for i := 1; i <= 3; i++ {
		block := blockWithTime(t, sender, receiver.uid, int64(1700000000000+i), fmt.Sprintf("p%d", i))
		if ok, err := InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert: %v %v", ok, err)
		}
	}

	// limit = 0 means "no limit" in MongoDB.
	all, err := FindManyBlocksLimited(bson.M{}, 0, 0)
	if err != nil {
		t.Fatalf("FindManyBlocksLimited: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("limit 0 returned %d blocks, want 3", len(all))
	}
}

func TestFindManyBlocksLimitedIgnoresNegativeSkip(t *testing.T) {
	newTestCollection()
	t.Cleanup(ResetCollection)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "payload", fmt.Sprintf("%064x", 1))
	if ok, err := InsertOneBlock(block); !ok || err != nil {
		t.Fatalf("insert: %v %v", ok, err)
	}

	blocks, err := FindManyBlocksLimited(bson.M{}, -5, 10)
	if err != nil {
		t.Fatalf("FindManyBlocksLimited: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("negative skip returned %d blocks, want 1", len(blocks))
	}
}

func TestFindManyBlocksLimitedPropagatesErrors(t *testing.T) {
	SetCollection(newFailingCollection())
	t.Cleanup(ResetCollection)

	if _, err := FindManyBlocksLimited(bson.M{}, 0, 10); err == nil {
		t.Fatal("FindManyBlocksLimited should propagate the storage error")
	}
	if _, err := FindManyBlocksLimitedASE(bson.M{}, 0, 10); err == nil {
		t.Fatal("FindManyBlocksLimitedASE should propagate the storage error")
	}
	if _, err := InsertManyBlock([]interface{}{Block{}}); err == nil {
		t.Fatal("InsertManyBlock should propagate the storage error")
	}
}
