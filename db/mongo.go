package db

import (
	"context"
	"errors"
	"fmt"

	"blvchain/core/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Collection is the subset of *mongo.Collection that this package depends on.
// It exists so the storage layer can be unit-tested with the in-memory fake in
// db/dbtest instead of a live MongoDB.
type Collection interface {
	InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error)
	InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error)
	FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) *mongo.SingleResult
	Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error)
}

// ErrNoCollection is returned when a storage function is called before
// SetCollection has been called.
var ErrNoCollection = errors.New("db: no collection configured; call db.SetCollection first")

var collection Collection

// SetCollection installs the backing collection. main calls it once at start-up;
// tests call it with dbtest.New().
func SetCollection(c Collection) { collection = c }

// GetCollection returns the installed collection (may be nil).
func GetCollection() Collection { return collection }

// ResetCollection removes the installed collection. Intended for tests.
func ResetCollection() { collection = nil }

func opContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), config.DB_OP_TIMEOUT)
}

// ConnectToMongoDB opens a client and verifies the connection with a ping.
func ConnectToMongoDB() (*mongo.Client, error) {
	ctx, cancel := opContext()
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.MONGO_URI)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping: %w", err)
	}

	return client, nil
}

// InsertOneBlock stores a single block.
func InsertOneBlock(document interface{}) (bool, error) {
	if collection == nil {
		return false, ErrNoCollection
	}

	ctx, cancel := opContext()
	defer cancel()

	if _, err := collection.InsertOne(ctx, document); err != nil {
		return false, err
	}
	return true, nil
}

// InsertManyBlock stores a batch of blocks. Unordered inserts are used so one
// duplicate does not discard the rest of the batch.
func InsertManyBlock(documents []interface{}) (bool, error) {
	if collection == nil {
		return false, ErrNoCollection
	}
	if len(documents) == 0 {
		return true, nil
	}

	ctx, cancel := opContext()
	defer cancel()

	if _, err := collection.InsertMany(ctx, documents, options.InsertMany().SetOrdered(false)); err != nil {
		return false, err
	}
	return true, nil
}

// FindOneBlock decodes the block with the given hash into result.
func FindOneBlock(blockHash string, result interface{}) error {
	if collection == nil {
		return ErrNoCollection
	}

	ctx, cancel := opContext()
	defer cancel()

	return collection.FindOne(ctx, bson.M{"_id": blockHash}).Decode(result)
}

// FindManyBlocksLimited returns blocks sorted newest first.
//
// An empty result is reported as an empty slice and a nil error; callers should
// check len() rather than comparing against mongo.ErrNoDocuments.
func FindManyBlocksLimited(filter primitive.M, skip int64, limit int64) ([]Block, error) {
	return findBlocks(filter, skip, limit, config.DESC)
}

// FindManyBlocksLimitedASE returns blocks sorted oldest first.
func FindManyBlocksLimitedASE(filter primitive.M, skip int64, limit int64) ([]Block, error) {
	return findBlocks(filter, skip, limit, config.ASC)
}

// FindAllBlocks returns every block matching filter, newest first, without a
// limit. It is used by the genesis check and by the "does this hash exist?"
// lookups.
func FindAllBlocks(filter primitive.M) ([]Block, error) {
	return findBlocks(filter, 0, 0, config.DESC)
}

func findBlocks(filter primitive.M, skip int64, limit int64, sort primitive.M) ([]Block, error) {
	if collection == nil {
		return nil, ErrNoCollection
	}
	if filter == nil {
		filter = bson.M{}
	}
	if skip < 0 {
		skip = 0
	}
	if limit < 0 {
		limit = 0
	}

	ctx, cancel := opContext()
	defer cancel()

	findOptions := options.Find().SetSort(sort)
	if skip > 0 {
		findOptions.SetSkip(skip)
	}
	if limit > 0 {
		findOptions.SetLimit(limit)
	}

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make([]Block, 0)
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}
