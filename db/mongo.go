package db

import (
	"context"

	"blvchain/core/config"
	"blvchain/core/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectToMongoDB() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(config.MONGO_URI)

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, config.ErrCanNotConnectToMongoDB
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, config.ErrCanNotGetPingFromMongoDB
	}

	return client, nil
}

func InsertOne(collection *mongo.Collection, document interface{}, unique_key string) (bool, error) {

	_, index_err := collection.Indexes().CreateOne(
		context.Background(),
		mongo.IndexModel{
			Keys:    bson.M{unique_key: 1},
			Options: options.Index().SetUnique(true),
		},
	)
	if index_err != nil {
		return false, index_err
	}
	_, err := collection.InsertOne(context.TODO(), document)
	if err != nil {
		return false, err
	}

	return true, nil
}

func FindOne(collection *mongo.Collection, filter primitive.M, result interface{}) error {
	findOptions := options.FindOne()
	err := collection.FindOne(context.TODO(), filter, findOptions).Decode(result)
	if err != nil {
		return err
	}

	return nil
}

func FindManyDatasLimited(filter primitive.M, skip string, limit string) ([]Data, error) {
	var result []Data

	if skip == "" {
		skip = "0"
	}
	if limit == "" {
		limit = "1"
	}

	findOptions := options.Find().SetSort(config.DESC).SetLimit(utils.StringToInt64(limit)).SetSkip(utils.StringToInt64(skip))
	cursor, find_err := config.DATA_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return result, config.ErrFindMany
	}
	cursor_err := cursor.All(context.TODO(), &result)
	if cursor_err != nil {
		return result, config.ErrCursor
	}

	if result == nil {
		return result, mongo.ErrNoDocuments
	}

	return result, nil
}

func FindAllDatas(filter primitive.M) ([]Data, error) {
	var result []Data

	findOptions := options.Find().SetSort(config.DESC)
	cursor, find_err := config.DATA_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return result, config.ErrFindMany
	}
	cursor_err := cursor.All(context.TODO(), &result)
	if cursor_err != nil {
		return result, config.ErrCursor
	}

	if result == nil {
		return result, mongo.ErrNoDocuments
	}

	return result, nil
}
