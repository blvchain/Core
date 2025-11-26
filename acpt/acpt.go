package acpt

// Async-Commit Parallel Tree

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func RunACPT() {
	// 1. Setup MongoDB
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	// Use specific collection for Contract State
	stateColl := client.Database("blv_chain").Collection("contract_state")

	// 2. Initialize ACPT Manager
	// "wal_logs" is the folder where .log files will be stored
	treeManager, err := NewTreeManager("./wal_logs", stateColl)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ACPT Initialized.")

	// 3. Start Background Cleanup Routine (Every 5 minutes)
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			fmt.Println("Performing WAL Checkpoint/Cleanup...")
			treeManager.Checkpoint()
		}
	}()

	// --- SIMULATION OF BLOCKCHAIN LOOP ---

	// Mock Data from VM
	updates := []KeyValue{
		{Key: []byte("contractA_user1_balance"), Value: []byte("1000")},
		{Key: []byte("contractA_user2_balance"), Value: []byte("500")},
		{Key: []byte("contractB_owner"), Value: []byte("AdminAddress")},
	}

	// 4. Commit Block
	start := time.Now()
	rootHash, err := treeManager.CommitBlock(updates)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Block Committed! State Root: %x\n", rootHash)
	fmt.Printf("Time Taken: %v\n", time.Since(start))

	// Prevent main from exiting so background tasks can finish (for demo)
	select {}
}
