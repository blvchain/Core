package main

import (
	"context"
	"net"
	"net/http"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/protos"
	"blvchain/core/ws"

	"google.golang.org/grpc"
)

func main() {

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

	// Check gensis block and dns seed in DB in first run of Node
	if check_genesis {

		//* gRPC
		go func() {
			grpcListener, grpcListener_err := net.Listen("tcp", config.GRPC_PORT)
			if grpcListener_err != nil {
				logger.GRPC_F_LOGGER.Fatalf("Failed to listen gRPC: %v", grpcListener_err)
			}
			grpcServer := grpc.NewServer()

			// Register the services
			protos.RegisterAddDataServer(grpcServer, &protos.AddDataService{})
			protos.RegisterReadDataServer(grpcServer, &protos.ReadDataService{})

			logger.GRPC_S_LOGGER.Println("gRPC server is running on port", config.GRPC_PORT)

			if grpcServer_err := grpcServer.Serve(grpcListener); grpcServer_err != nil {
				logger.GRPC_F_LOGGER.Fatalf("Failed to serve: %v", grpcServer_err)
			}

		}()

		//* WebSocket
		go func() {

			//* Connect to other servers
			ws.ClientManagerVar.ConnectToServers(config.DNS_SEED_LIST)

			//* Local server gateways
			http.HandleFunc("/", ws.WS_Server_Handler)

			logger.WS_S_LOGGER.Println("WebSocket Server is running on port", config.WEBSOCKET_PORT)
			websocketListener_err := http.ListenAndServe(config.WEBSOCKET_PORT, nil)
			if websocketListener_err != nil {
				logger.WS_F_LOGGER.Fatalf("Failed to listen WebSocket: %v", websocketListener_err)
			}

		}()

		// Monitor and reconnect to missed nodes
		go func() {
			ws.MonitorAndReconnectToServers(&ws.ClientManagerVar)
		}()

		// Prevent main from exiting
		select {}

	} else {
		// Print error if gensis conditions fail
		logger.INTERNAL_LOGGER.Fatal(check_genesis_err)
	}
}
