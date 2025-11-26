package acpt

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
)

type TreeManager struct {
	Shards    [256]*Shard
	Wal       *WALManager
	MongoColl *mongo.Collection
}

func NewTreeManager(walDir string, coll *mongo.Collection) (*TreeManager, error) {
	wal, err := NewWALManager(walDir)
	if err != nil {
		return nil, err
	}

	tm := &TreeManager{
		Wal:       wal,
		MongoColl: coll,
	}

	// Initialize 256 shards
	for i := 0; i < 256; i++ {
		tm.Shards[i] = NewShard(i)
	}

	return tm, nil
}

// CommitBlock is called by Validator after executing VM
func (tm *TreeManager) CommitBlock(data []KeyValue) ([]byte, error) {

	// 1. WAL Write (Safety First - Blocking)
	if err := tm.Wal.Append(data); err != nil {
		return nil, fmt.Errorf("critical WAL failure: %v", err)
	}

	// 2. Split data into shards (Parallel Prep)
	shardedData := make(map[int][]KeyValue)
	for _, item := range data {
		if len(item.Key) == 0 {
			continue
		}
		// Use first byte of key to determine shard (0-255)
		shardID := int(item.Key[0])
		shardedData[shardID] = append(shardedData[shardID], item)
	}

	// 3. Update RAM & Compute Roots (Parallel Execution)
	var wg sync.WaitGroup
	shardRoots := make([][]byte, 256)

	for i := 0; i < 256; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			shard := tm.Shards[id]

			// If this shard has updates, apply them
			if items, ok := shardedData[id]; ok {
				shard.UpdateInMemory(items)
			}
			// Always compute root (state might not change, but we need the hash)
			shardRoots[id] = shard.ComputeRoot()
		}(i)
	}
	wg.Wait()

	// 4. Compute Global State Root
	globalRoot := tm.computeGlobalRoot(shardRoots)

	// 5. Trigger Background Flush (Non-Blocking)
	go tm.backgroundFlush()

	return globalRoot, nil
}

func (tm *TreeManager) computeGlobalRoot(shardRoots [][]byte) []byte {
	// Collapse 256 hashes into 1
	finalHash := make([]byte, 32)
	for _, root := range shardRoots {
		finalHash = HashPair(finalHash, root)
	}
	return finalHash
}

// backgroundFlush pushes data to Mongo
func (tm *TreeManager) backgroundFlush() {
	ctx := context.Background()
	for i := 0; i < 256; i++ {
		// We check each shard. If it has dirty data, it writes.
		// This is concurrent-safe because Shard has its own Mutex.
		go tm.Shards[i].FlushToMongo(ctx, tm.MongoColl)
	}
}

// Checkpoint should be called periodically (e.g., every 5 mins)
func (tm *TreeManager) Checkpoint() {
	currentID := tm.Wal.GetCurrentSegmentID()
	// Trigger cleanup logic
	tm.Wal.TruncateOldSegments(currentID)
}
