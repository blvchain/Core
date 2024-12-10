package config

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	// Global ENV
	MONGO_URI string = GetEnv("MONGO_URI")

	// Mongodb
	NO_FILTER primitive.M = bson.M{}
	DESC      primitive.M = bson.M{"_id": -1}
	ASC       primitive.M = bson.M{"_id": 1}

	DATA_COLL *mongo.Collection

	// WebSocket
	Broadcast = make(chan []byte)

	// Delium config
	DELIUM_CONFIG = GetDeliumConfigFile()

	BLV_INFO = GetBlvInfoFile()

	DNS_SEED_LIST = GetDnsSeedListFile()
)
