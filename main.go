package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"blvchain/core/bvm"
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/protos"
	"blvchain/core/utils"
	"blvchain/core/ws"

	"google.golang.org/grpc"
)

func main() {

	bvm_err := bvm.RunWasm(config.SMART_CONTRACT_PATH)

	if bvm_err != nil {
		fmt.Println(bvm_err)
	}

	fmt.Println("d256c: ", utils.D256C("abcdefghijklmnopqrstuvwxyz", "2h4usk#5/73uytg#9/#4").String)
	fmt.Println("d512c: ", utils.D512C("abcdefghijklmnopqrstuvwxyz", "2h4usk#5/73uytg#9/#4").String)

	return

	//! ===========================================

	syncDone := make(chan bool)

	client, client_err := db.ConnectToMongoDB()
	if client_err != nil {
		logger.INTERNAL_LOGGER.Fatal(client_err)
	}
	defer func() {
		if client_err = client.Disconnect(context.TODO()); client_err != nil {
			logger.INTERNAL_LOGGER.Fatal(client_err)
		}
	}()

	dataBase := client.Database(config.DATABASE_NAME)
	config.BLOCK_COLL = dataBase.Collection(config.BLOCK_COLLECTION_NAME)

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

			//* gRPC
			go func() {
				grpcListener, grpcListener_err := net.Listen("tcp", config.GRPC_PORT)
				if grpcListener_err != nil {
					logger.GRPC_F_LOGGER.Fatalf("Error: Failed to listen gRPC: %v", grpcListener_err)
				}
				grpcServer := grpc.NewServer()

				// Register the services
				protos.RegisterAddDataServer(grpcServer, &protos.AddDataService{})
				protos.RegisterReadDataServer(grpcServer, &protos.ReadDataService{})

				logger.GRPC_S_LOGGER.Println("Success: gRPC server is running on port", config.GRPC_PORT)

				if grpcServer_err := grpcServer.Serve(grpcListener); grpcServer_err != nil {
					logger.GRPC_F_LOGGER.Fatalf("Error: Failed to serve: %v", grpcServer_err)
				}

			}()

			// Sync missed data
			go func() {
				ws.SyncData(&ws.ClientManagerVar)
			}()

		} else {
			logger.INTERNAL_LOGGER.Fatal("Error: Data sync failed")
		}

		// Prevent main from exiting
		select {}

	} else {
		// Print error if genesis conditions fail
		logger.INTERNAL_LOGGER.Printf("Error: %v", check_genesis_err)
	}
}
