package config

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Delium_input struct {
	Delete_step int
	Repeat      int
}

var (
	// Global ENV
	NODE_URL     string = GetEnv("NODE_URL")
	NODE_WALLET  string = GetEnv("NODE_WALLET")
	NODE_PRIVKEY string = GetEnv("NODE_PRIVKEY")
	NODE_PUBKEY  string = GetEnv("NODE_PUBKEY")
	MONGO_URI    string = GetEnv("MONGO_URI")

	// Mongodb
	SET_ID_TO_ZERO primitive.M = bson.M{"_id": 0}
	NO_FILTER      primitive.M = bson.M{}
	DESC           primitive.M = bson.M{"_id": -1}
	ASC            primitive.M = bson.M{"_id": 1}

	NODE_ID_COLL     *mongo.Collection
	SELF_AUTH_COLL   *mongo.Collection
	CLIENT_AUTH_COLL *mongo.Collection
	DATA_COLL        *mongo.Collection
	DNS_SEED_COLL    *mongo.Collection
	RATE_LIMIT_COLL  *mongo.Collection

	// WebSocket
	Broadcast = make(chan []byte)

	// Delium config
	WALLET_DELIUM_CONFIG Delium_input = Delium_input{
		Delete_step: 10,
		Repeat:      2,
	}
	HASH_DELIUM_CONFIG Delium_input = Delium_input{
		Delete_step: 10,
		Repeat:      2,
	}
	MESSAGE_DELIUM_CONFIG Delium_input = Delium_input{
		Delete_step: 5,
		Repeat:      5,
	}
	AUTH_DELIUM_CONFIG Delium_input = Delium_input{
		Delete_step: 10,
		Repeat:      2,
	}
	MERKLE_DELIUM_CONFIG Delium_input = Delium_input{
		Delete_step: 10,
		Repeat:      2,
	}
	FEED_DELIUM_CONFIG Delium_input = Delium_input{
		Delete_step: 10,
		Repeat:      2,
	}
)
