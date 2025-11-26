package db

import (
	"context"

	"blvchain/core/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectToMongoDB() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(config.MONGO_URI)

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func InsertOneBlock(document interface{}) (bool, error) {

	_, err := config.BLOCK_COLL.InsertOne(context.TODO(), document)
	if err != nil {
		return false, err
	}

	return true, nil
}
func InsertManyBlock(document []interface{}) (bool, error) {
	options := options.InsertMany().SetOrdered(false)

	_, err := config.BLOCK_COLL.InsertMany(context.TODO(), document, options)
	if err != nil {
		return false, err
	}

	return true, nil
}

func FindOneBlock(blockHash string, result interface{}) error {
	err := config.BLOCK_COLL.FindOne(context.TODO(), bson.M{"_id": blockHash}).Decode(result)
	if err != nil {
		return err
	}

	return nil
}

func FindManyBlocksLimited(filter primitive.M, skip int64, limit int64) ([]Block, error) {
	var result []Block

	findOptions := options.Find().SetSort(config.DESC).SetLimit(limit).SetSkip(skip)
	cursor, find_err := config.BLOCK_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return result, find_err
	}
	cursor_err := cursor.All(context.TODO(), &result)
	if cursor_err != nil {
		return result, cursor_err
	}

	if result == nil {
		return result, mongo.ErrNoDocuments
	}

	return result, nil
}

func FindManyBlocksLimitedASE(filter primitive.M, skip int64, limit int64) ([]Block, error) {
	var result []Block

	findOptions := options.Find().SetSort(config.ASC).SetLimit(limit).SetSkip(skip)
	cursor, find_err := config.BLOCK_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return result, find_err
	}
	cursor_err := cursor.All(context.TODO(), &result)
	if cursor_err != nil {
		return result, cursor_err
	}

	if result == nil {
		return result, mongo.ErrNoDocuments
	}

	return result, nil
}

func FindAllBlocks(filter primitive.M) ([]Block, error) {
	var result []Block

	findOptions := options.Find().SetSort(config.DESC)
	cursor, find_err := config.BLOCK_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return result, find_err
	}
	cursor_err := cursor.All(context.TODO(), &result)
	if cursor_err != nil {
		return result, cursor_err
	}

	if result == nil {
		return result, mongo.ErrNoDocuments
	}

	return result, nil
}

func FindLastBlockBy(filter primitive.M) (*Block, error) {
	var results []Block

	findOptions := options.Find().SetSort(config.DESC).SetLimit(1)
	cursor, find_err := config.BLOCK_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return nil, find_err
	}
	cursor_err := cursor.All(context.TODO(), &results)
	if cursor_err != nil {
		return nil, cursor_err
	}

	if len(results) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return &results[0], nil
}
