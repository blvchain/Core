package db

import (
	"context"

	"matinramznegar/core/config"

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
	findOptions := options.FindOne().SetProjection(bson.M{"_id": 0})
	err := collection.FindOne(context.TODO(), filter, findOptions).Decode(result)
	if err != nil {
		return err
	}

	return nil
}
