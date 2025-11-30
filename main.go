package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"blvchain/core/acpt"
	"blvchain/core/bvm"
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/proto"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"google.golang.org/grpc"
)

func main() {

	/// ------- Test area ----------- //

	tm, err := acpt.NewTreeManager("mongodb://localhost:27017", "blockchain_db_prod")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("--- Initial Root: %x ---\n", tm.CurrentRoot)

	// 2. Commit Batch 1
	batch1 := []acpt.KeyValue{
		{Key: []byte("user_A"), Value: []byte("1000")},
	}
	root1, err := tm.CommitBlock(batch1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Root after Batch 1: %x\n", root1)

	// 3. Commit Batch 2 (Modifying unrelated key)
	// logic: This should fetch user_A from DB to calculate the new root
	batch2 := []acpt.KeyValue{
		{Key: []byte("user_B"), Value: []byte("5000")},
	}
	root2, err := tm.CommitBlock(batch2)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Root after Batch 2: %x\n", root2)

	/// ------- Test area ----------- //
	return

	fmt.Println("===== Starting core =====")

	// Run ACPT

	// Init BVM internal functions
	bvm.InitBVMInternalFunctions()

	// Run Smart contract
	bvm_err := bvm.RunSmartContract("./bvm/smart_contract.wasm")
	if bvm_err != nil {
		fmt.Println("Error: see smartContract/fail folder for details.")
		logger.SC_F_LOGGER.Fatal(bvm_err)
	}

	syncDone := make(chan bool)

	client, client_err := db.ConnectToMongoDB()
	if client_err != nil {
		fmt.Println("Error: see log/internal folder for details.")
		logger.INTERNAL_LOGGER.Fatal(client_err)
	}
	defer func() {
		if client_err = client.Disconnect(context.TODO()); client_err != nil {
			fmt.Println("Error: see log/internal folder for details.")
			logger.INTERNAL_LOGGER.Fatal(client_err)
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

		// Wait for sync to complete before starting gRPC server
		if <-syncDone {

			logger.INTERNAL_LOGGER.Println("Success: Data sync completed, running gRPC server")
			fmt.Println("Success: Data sync completed, running gRPC server")

			//* gRPC
			go func() {
				grpcListener, grpcListener_err := net.Listen("tcp", config.GRPC_PORT)
				if grpcListener_err != nil {
					fmt.Println("Error: see log/gRPC/fail folder for details.")
					logger.GRPC_F_LOGGER.Fatalf("Error: Failed to listen gRPC: %v", grpcListener_err)
				}

				// Per-method configuration
				methodCfg := map[string]proto.MethodLimit{
					"/gate.AddData/addData":   {R: config.ADD_DATA_R, Burst: config.ADD_DATA_BURST},
					"/gate.ReadData/readData": {R: config.READ_DATA_R, Burst: config.READ_DATA_BURST},
				}

				// default used for other RPCs (if any)
				defaultCfg := proto.MethodLimit{R: 5, Burst: 10}

				// create rate limiter with TTL for idle entries
				rl := proto.NewRateLimiter(methodCfg, defaultCfg, 5*time.Minute)
				grpcServer := grpc.NewServer(
					grpc.UnaryInterceptor(rl.UnaryServerInterceptor()),
					grpc.StreamInterceptor(rl.StreamServerInterceptor()),
				)

				// Register the services
				proto.RegisterAddDataServer(grpcServer, &proto.AddDataService{})
				proto.RegisterReadDataServer(grpcServer, &proto.ReadDataService{})

				logger.GRPC_S_LOGGER.Println("Success: gRPC server is running on port", config.GRPC_PORT)
				fmt.Println("Success: gRPC server is running on port", config.GRPC_PORT)

				if grpcServer_err := grpcServer.Serve(grpcListener); grpcServer_err != nil {
					fmt.Println("Error: see log/gRPC/fail folder for details.")
					logger.GRPC_F_LOGGER.Fatalf("Error: Failed to serve: %v", grpcServer_err)
				}

			}()

		} else {
			fmt.Println("Error: see log/internal folder for details.")
			logger.INTERNAL_LOGGER.Fatal("Error: Data sync failed")
		}

		// Prevent main from exiting
		select {}

	} else {
		// Print error if genesis conditions fail
		logger.INTERNAL_LOGGER.Printf("Error: %v", check_genesis_err)
		fmt.Println("Error: see log/internal folder for details.")
	}
}
