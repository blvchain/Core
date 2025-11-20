package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/protos"
	"blvchain/core/ws"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"google.golang.org/grpc"
)

func main() {

	// Init BVM internal functions
	// bvm.InitBVMInternalFunctions()

	// // Run BVM
	// bvm_err := bvm.RunBVM(config.SMART_CONTRACT_PATH)
	// if bvm_err != nil {
	// 	fmt.Println(bvm_err)
	// }

	fmt.Println("===== Starting core =====")

	syncDone := make(chan bool)

	client, client_err := db.ConnectToMongoDB()
	if client_err != nil {
		logger.INTERNAL_LOGGER.Fatal(client_err)
		fmt.Println("Error: see log/internal folder for details.")
	}
	defer func() {
		if client_err = client.Disconnect(context.TODO()); client_err != nil {
			logger.INTERNAL_LOGGER.Fatal(client_err)
			fmt.Println("Error: see log/internal folder for details.")
		}
	}()

	dataBase := client.Database(config.DATABASE_NAME)
	config.BLOCK_COLL = dataBase.Collection(config.BLOCK_COLLECTION_NAME)

	// create Mongo indexes for Block collection
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Single-field index for timestamp (used for sorting and range queries)
		tsIndexModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "blockMeta.timeStamp", Value: -1}},
			Options: options.Index().SetName("idx_blockmeta_timestamp_desc"),
		}

		// Compound indexes to support common query patterns: filter by sender/receiver and sort by timestamp
		senderUidTsIndex := mongo.IndexModel{
			Keys:    bson.D{{Key: "blockData.senderUid", Value: 1}, {Key: "blockMeta.timeStamp", Value: -1}},
			Options: options.Index().SetName("idx_blockdata_senderuid_ts"),
		}

		receiverUidTsIndex := mongo.IndexModel{
			Keys:    bson.D{{Key: "blockData.receiverUid", Value: 1}, {Key: "blockMeta.timeStamp", Value: -1}},
			Options: options.Index().SetName("idx_blockdata_receiveruid_ts"),
		}

		// Indexes for equality filters used in several places
		useContractIndex := mongo.IndexModel{
			Keys:    bson.D{{Key: "blockData.useContract", Value: 1}},
			Options: options.Index().SetName("idx_blockdata_usecontract"),
		}

		idxs := []mongo.IndexModel{tsIndexModel, senderUidTsIndex, receiverUidTsIndex, useContractIndex}

		if _, idxErr := config.BLOCK_COLL.Indexes().CreateMany(ctx, idxs); idxErr != nil {
			logger.INTERNAL_LOGGER.Printf("Warning: failed to create indexes: %v", idxErr)
			fmt.Println("Error: see log/internal folder for details.")
		}
	}

	// Genesis makers
	check_genesis, check_genesis_err := db.Genesis_check()

	// Check genesis block and dns seed in DB in first run of Node
	if check_genesis {

		//* WebSocket
		go func() {

			//* Connect to other servers
			ws.ClientManagerVar.ConnectToServers(config.DNS_SEED_LIST)

			//* Local server gateways
			http.HandleFunc("/", ws.WS_Server_Handler)

			logger.WS_S_LOGGER.Println("Success: WebSocket Server is running on port", config.WEBSOCKET_PORT)
			websocketListener_err := http.ListenAndServe(config.WEBSOCKET_PORT, nil)
			if websocketListener_err != nil {
				logger.WS_F_LOGGER.Fatalf("Failed to listen WebSocket: %v", websocketListener_err)
				fmt.Println("Error: see log/websocket/fail folder for details.")
			}

		}()

		// Monitor and reconnect to missed nodes
		go func() {
			ws.MonitorAndReconnectToServers(&ws.ClientManagerVar)
		}()

		go func() {
			genesis_sync_result := ws.FirstTimeSyncData(&ws.ClientManagerVar)
			syncDone <- genesis_sync_result
		}()

		// Wait for sync to complete before starting gRPC server
		if <-syncDone {

			logger.INTERNAL_LOGGER.Println("Success: Data sync completed, running gRPC server")
			fmt.Println("Success: Data sync completed, running gRPC server")

			//* gRPC
			go func() {
				grpcListener, grpcListener_err := net.Listen("tcp", config.GRPC_PORT)
				if grpcListener_err != nil {
					logger.GRPC_F_LOGGER.Fatalf("Error: Failed to listen gRPC: %v", grpcListener_err)
					fmt.Println("Error: see log/gRPC/fail folder for details.")
				}

				// Per-method configuration
				methodCfg := map[string]protos.MethodLimit{
					"/gate.AddData/addData":   {R: config.ADD_DATA_R, Burst: config.ADD_DATA_BURST},
					"/gate.ReadData/readData": {R: config.READ_DATA_R, Burst: config.READ_DATA_BURST},
				}

				// default used for other RPCs (if any)
				defaultCfg := protos.MethodLimit{R: 5, Burst: 10}

				// create rate limiter with TTL for idle entries
				rl := protos.NewRateLimiter(methodCfg, defaultCfg, 5*time.Minute)
				grpcServer := grpc.NewServer(
					grpc.UnaryInterceptor(rl.UnaryServerInterceptor()),
					grpc.StreamInterceptor(rl.StreamServerInterceptor()),
				)

				// Register the services
				protos.RegisterAddDataServer(grpcServer, &protos.AddDataService{})
				protos.RegisterReadDataServer(grpcServer, &protos.ReadDataService{})

				logger.GRPC_S_LOGGER.Println("Success: gRPC server is running on port", config.GRPC_PORT)
				fmt.Println("Success: gRPC server is running on port", config.GRPC_PORT)

				if grpcServer_err := grpcServer.Serve(grpcListener); grpcServer_err != nil {
					logger.GRPC_F_LOGGER.Fatalf("Error: Failed to serve: %v", grpcServer_err)
					fmt.Println("Error: see log/gRPC/fail folder for details.")
				}

			}()

			// Sync missed data
			go func() {
				ws.SyncData(&ws.ClientManagerVar)
			}()

		} else {
			logger.INTERNAL_LOGGER.Fatal("Error: Data sync failed")
			fmt.Println("Error: see log/internal folder for details.")
		}

		// Prevent main from exiting
		select {}

	} else {
		// Print error if genesis conditions fail
		logger.INTERNAL_LOGGER.Printf("Error: %v", check_genesis_err)
		fmt.Println("Error: see log/internal folder for details.")
	}
}
