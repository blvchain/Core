package main

import (
	"context"
	"log"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/utils"
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

	config.SELF_AUTH_COLL = dataBase.Collection(config.SELF_AUTH_COLLECTION_NAME)
	config.CLIENT_AUTH_COLL = dataBase.Collection(config.CLIENT_AUTH_COLLECTION_NAME)
	config.DATA_COLL = dataBase.Collection(config.DATA_COLLECTION_NAME)

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
