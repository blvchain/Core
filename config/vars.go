package config

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	// Global ENV
	MONGO_URI string = GetEnv("MONGO_URI")
	SELF_UID  string = GetEnv("SELF_UID")

	// Mongodb
	NO_FILTER primitive.M = bson.M{}
	DESC      primitive.M = bson.M{"_id": -1}
	ASC       primitive.M = bson.M{"_id": 1}

	BLOCK_COLL *mongo.Collection

	// Get vars form files
	DELIUM_CONFIG = GetDeliumConfigFile()
	DNS_SEED_LIST = GetDnsSeedListFile()
	API_KEY_LIST  = GetApiKeyFile()
)
