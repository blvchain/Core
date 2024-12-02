package main

import (
	"context"
	"log"

	"matinramznegar/core/config"
	"matinramznegar/core/db"
	"matinramznegar/core/utils"
)

func main() {

	client, err := db.ConnectToMongoDB()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()

	dataBase := client.Database(config.DATABASE_NAME)

	config.NODE_ID_COLL = dataBase.Collection(config.NODE_ID_COLLECTION_NAME)
	config.SELF_AUTH_COLL = dataBase.Collection(config.SELF_AUTH_COLLECTION_NAME)
	config.CLIENT_AUTH_COLL = dataBase.Collection(config.CLIENT_AUTH_COLLECTION_NAME)
	config.Data_COLL = dataBase.Collection(config.Data_COLLECTION_NAME)
	config.DNS_SEED_COLL = dataBase.Collection(config.DNS_SEED_COLLECTION_NAME)
	config.RATE_LIMIT_COLL = dataBase.Collection(config.RATE_LIMIT_COLLECTION_NAME)

	// Genesis makers
	check_genesis, check_genesis_err := db.Genesis_check()

	// Check gensis block and dns seed in DB in first run of Node
	//! first get data from other nodes, then work
	if check_genesis {

	} else {
		// Print error if gensis conditions fail
		utils.PrintError(check_genesis_err)
	}
}
