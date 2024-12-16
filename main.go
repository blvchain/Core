package main

import (
	"context"
	"log"
	"net"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/protos"
	"blvchain/core/utils"

	"google.golang.org/grpc"
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
	config.DATA_COLL = dataBase.Collection(config.DATA_COLLECTION_NAME)

	// Genesis makers
	check_genesis, check_genesis_err := db.Genesis_check()

	// Check gensis block and dns seed in DB in first run of Node
	if check_genesis {

		// gRPC
		listener, err := net.Listen("tcp", config.GRPC_PORT)
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		grpcServer := grpc.NewServer()

		// Register the services
		protos.RegisterAddDataServer(grpcServer, &protos.AddDataService{})
		protos.RegisterReadDataServer(grpcServer, &protos.ReadDataService{})

		log.Println("gRPC server is running on port: ", config.GRPC_PORT)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}

	} else {
		// Print error if gensis conditions fail
		utils.PrintError(check_genesis_err)
	}
}
