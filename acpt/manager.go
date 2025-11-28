package acpt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type TreeManager struct {
	Wal            *WAL
	MongoCollData  *mongo.Collection
	MongoCollNodes *mongo.Collection
	MongoCollState *mongo.Collection // Stores "current_root"

	CurrentRoot []byte // In-Memory pointer to current root
	mu          sync.RWMutex
}

func NewTreeManager(mongoURI, dbName string) (*TreeManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wc := writeconcern.New(writeconcern.WMajority())
	opts := options.Client().ApplyURI(mongoURI).SetWriteConcern(wc)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %v", err)
	}

	db := client.Database(dbName)

	tm := &TreeManager{
		MongoCollData:  db.Collection("state_data"),
		MongoCollNodes: db.Collection("state_nodes"),
		MongoCollState: db.Collection("global_state"),
	}

	// 1. Initialize WAL
	tm.Wal, err = OpenWAL("wal_current.log")
	if err != nil {
		return nil, err
	}

	// 2. Load Last Root from DB
	var state GlobalState
	err = tm.MongoCollState.FindOne(ctx, bson.M{"_id": "current_root"}).Decode(&state)
	if err == nil {
		tm.CurrentRoot = state.RootHash.Data
		fmt.Printf("loaded existing root: %x\n", tm.CurrentRoot)
	} else {
		fmt.Println("starting with empty tree")
		tm.CurrentRoot = nil
	}

	return tm, nil
}

// DBFetcherImpl implements NodeFetcher for merkle.go
func (tm *TreeManager) DBFetcherImpl(hash []byte) (*DBNode, error) {
	var node DBNode
	// Look directly in state_nodes collection by Hash ID
	err := tm.MongoCollNodes.FindOne(context.Background(), bson.M{"_id": toBin(hash)}).Decode(&node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (tm *TreeManager) CommitBlock(batch []KeyValue) ([]byte, error) {
	if len(batch) == 0 {
		return tm.CurrentRoot, nil
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 1. Write to WAL
	if err := tm.Wal.Append(batch); err != nil {
		return nil, err
	}

	// 2. Calculate New Tree State
	// We pass tm.DBFetcherImpl so the algorithm can retrieve siblings from Mongo
	newRoot, nodesMap, err := ApplyChanges(tm.CurrentRoot, batch, tm.DBFetcherImpl)
	if err != nil {
		return nil, fmt.Errorf("merkle calc error: %v", err)
	}

	// 3. Flush to DB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := tm.flushToMongo(ctx, batch, nodesMap, newRoot); err != nil {
		return nil, err
	}

	// 4. Update Memory Root
	tm.CurrentRoot = newRoot
	return newRoot, nil
}

func (tm *TreeManager) flushToMongo(ctx context.Context, data []KeyValue, nodes map[string]DBNode, newRoot []byte) error {
	// A. Upsert Raw Data
	var dataModels []mongo.WriteModel
	for _, kv := range data {
		model := mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": toBin(kv.Key)}).
			SetReplacement(DBData{Key: toBin(kv.Key), Value: toBin(kv.Value)}).
			SetUpsert(true)
		dataModels = append(dataModels, model)
	}

	// B. Upsert Merkle Nodes (Only the new/changed ones)
	var nodeModels []mongo.WriteModel
	for _, node := range nodes {
		model := mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": node.ID}).
			SetReplacement(node).
			SetUpsert(true)
		nodeModels = append(nodeModels, model)
	}

	// C. Update Global Root Pointer
	stateModel := mongo.NewReplaceOneModel().
		SetFilter(bson.M{"_id": "current_root"}).
		SetReplacement(GlobalState{ID: "current_root", RootHash: toBin(newRoot)}).
		SetUpsert(true)

	// Execute
	if len(dataModels) > 0 {
		tm.MongoCollData.BulkWrite(ctx, dataModels)
	}
	if len(nodeModels) > 0 {
		tm.MongoCollNodes.BulkWrite(ctx, nodeModels)
	}
	tm.MongoCollState.BulkWrite(ctx, []mongo.WriteModel{stateModel})

	return nil
}

// Helper: Get raw value (used by VM)
func (tm *TreeManager) Get(key string) ([]byte, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var res DBData
	err := tm.MongoCollData.FindOne(context.Background(), bson.M{"_id": toBin([]byte(key))}).Decode(&res)
	if err != nil {
		return nil, nil
	}
	return res.Value.Data, nil
}
