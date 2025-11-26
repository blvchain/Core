package acpt

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Shard struct {
	ID    int
	mu    sync.RWMutex
	Cache map[string][]byte // RAM State
	Dirty map[string]bool   // Keys waiting for Mongo
}

func NewShard(id int) *Shard {
	return &Shard{
		ID:    id,
		Cache: make(map[string][]byte),
		Dirty: make(map[string]bool),
	}
}

// UpdateInMemory updates RAM immediately
func (s *Shard) UpdateInMemory(kvs []KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range kvs {
		keyStr := string(item.Key)
		s.Cache[keyStr] = item.Value
		s.Dirty[keyStr] = true
	}
}

// ComputeRoot calculates Merkle Root for this shard
func (s *Shard) ComputeRoot() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Get all keys and sort them (Determinism is mandatory)
	keys := make([]string, 0, len(s.Cache))
	for k := range s.Cache {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Simple Accumulator Hashing (Replace with JMT logic for production)
	// Root = Hash(PrevHash + Key + Value)
	rootHash := make([]byte, 32) // Start with empty 32 bytes
	for _, k := range keys {
		val := s.Cache[k]
		input := append(rootHash, []byte(k)...)
		input = append(input, val...)
		rootHash = Hash(input)
	}
	return rootHash
}

// FlushToMongo sends dirty keys to DB in one network request
func (s *Shard) FlushToMongo(ctx context.Context, collection *mongo.Collection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Dirty) == 0 {
		return nil
	}

	var writes []mongo.WriteModel

	for key := range s.Dirty {
		value := s.Cache[key]

		// UPSERT: Insert if new, Update if exists
		filter := bson.M{"key": key}
		update := bson.M{"$set": bson.M{"value": value}}

		model := mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true)

		writes = append(writes, model)
	}

	// Bulk Write (Unordered is faster)
	opts := options.BulkWrite().SetOrdered(false)
	_, err := collection.BulkWrite(ctx, writes, opts)

	if err != nil {
		// If DB fails, we keep Dirty flags true to try again later
		fmt.Printf("Shard %d Mongo Write Error: %v\n", s.ID, err)
		return err
	}

	// Success: Clear dirty flags
	s.Dirty = make(map[string]bool)
	return nil
}
