package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
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
	if err := run(); err != nil {
		logger.INTERNAL_LOGGER.Println("Fatal:", err)
		fmt.Println("Fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("===== Starting core =====")

	// ------------------------------------------------------------------
	// Configuration
	// ------------------------------------------------------------------
	if err := config.Validate(); err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Database
	// ------------------------------------------------------------------
	client, err := db.ConnectToMongoDB()
	if err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.INTERNAL_LOGGER.Printf("Warning: could not disconnect from MongoDB: %v", err)
		}
	}()

	collection := client.Database(config.DATABASE_NAME).Collection(config.BLOCK_COLLECTION_NAME)
	config.BLOCK_COLL = collection
	db.SetCollection(collection)

	createIndexes(collection)

	if ok, err := db.Genesis_check(); err != nil {
		return fmt.Errorf("genesis: %w", err)
	} else if !ok {
		return errors.New("genesis: the first block could not be created")
	}

	// ------------------------------------------------------------------
	// WebSocket server (peer-to-peer)
	// ------------------------------------------------------------------
	go func() {
		ws.ClientManagerVar.ConnectToServers(config.DNS_SEED_LIST)

		http.HandleFunc("/", ws.WS_Server_Handler)

		server := &http.Server{
			Addr:              config.WEBSOCKET_PORT,
			ReadHeaderTimeout: 5 * time.Second,
		}

		logger.WS_S_LOGGER.Println("Success: WebSocket server is running on port", config.WEBSOCKET_PORT)
		fmt.Println("Success: WebSocket server is running on port", config.WEBSOCKET_PORT)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WS_F_LOGGER.Printf("Failed to listen WebSocket: %v", err)
		}
	}()

	go ws.MonitorAndReconnectToServers(&ws.ClientManagerVar)

	// ------------------------------------------------------------------
	// Initial sync: the gRPC API must not answer before the chain is current.
	// ------------------------------------------------------------------
	syncDone := make(chan bool, 1)
	go func() {
		syncDone <- ws.FirstTimeSyncData(&ws.ClientManagerVar)
	}()

	if !<-syncDone {
		return errors.New("data sync failed")
	}

	logger.INTERNAL_LOGGER.Println("Success: Data sync completed, starting gRPC server")
	fmt.Println("Success: Data sync completed, starting gRPC server")

	go ws.SyncData(&ws.ClientManagerVar)

	// ------------------------------------------------------------------
	// gRPC API
	// ------------------------------------------------------------------
	grpcListener, err := net.Listen("tcp", config.GRPC_PORT)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}

	methodCfg := map[string]protos.MethodLimit{
		"/gate.AddData/addData":   {R: config.ADD_DATA_R, Burst: config.ADD_DATA_BURST},
		"/gate.ReadData/readData": {R: config.READ_DATA_R, Burst: config.READ_DATA_BURST},
	}
	defaultCfg := protos.MethodLimit{R: 5, Burst: 10}

	rl := protos.NewRateLimiter(methodCfg, defaultCfg, 5*time.Minute)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(rl.UnaryServerInterceptor()),
		grpc.StreamInterceptor(rl.StreamServerInterceptor()),
	)

	protos.RegisterAddDataServer(grpcServer, &protos.AddDataService{})
	protos.RegisterReadDataServer(grpcServer, &protos.ReadDataService{})

	go func() {
		logger.GRPC_S_LOGGER.Println("Success: gRPC server is running on port", config.GRPC_PORT)
		fmt.Println("Success: gRPC server is running on port", config.GRPC_PORT)

		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.GRPC_F_LOGGER.Printf("Error: gRPC server stopped: %v", err)
		}
	}()

	// ------------------------------------------------------------------
	// Wait for a termination signal
	// ------------------------------------------------------------------
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	fmt.Println("Shutting down...")
	grpcServer.GracefulStop()
	ws.ClientManagerVar.DisconnectFromServers()
	return nil
}

// createIndexes installs the indexes the read path relies on. A failure is
// logged but not fatal: the node still works, just slower.
func createIndexes(collection *mongo.Collection) {
	ctx, cancel := context.WithTimeout(context.Background(), config.DB_OP_TIMEOUT)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "blockMeta.timeStamp", Value: -1}},
			Options: options.Index().SetName("idx_blockmeta_timestamp_desc"),
		},
		{
			Keys:    bson.D{{Key: "blockData.senderUid", Value: 1}, {Key: "blockMeta.timeStamp", Value: -1}},
			Options: options.Index().SetName("idx_blockdata_senderuid_ts"),
		},
		{
			Keys:    bson.D{{Key: "blockData.receiverUid", Value: 1}, {Key: "blockMeta.timeStamp", Value: -1}},
			Options: options.Index().SetName("idx_blockdata_receiveruid_ts"),
		},
		{
			Keys:    bson.D{{Key: "blockData.useContract", Value: 1}},
			Options: options.Index().SetName("idx_blockdata_usecontract"),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		logger.INTERNAL_LOGGER.Printf("Warning: failed to create indexes: %v", err)
		fmt.Println("Warning: see log/internal folder for details.")
	}
}
