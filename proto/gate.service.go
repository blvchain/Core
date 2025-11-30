package proto

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/utils"
	context "context"
	"encoding/base64"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

func (s *AddDataService) AddData(ctx context.Context, req *BlockData) (*AddDataResult, error) {

	//* Invalid data
	// Check auth from metadata
	apiKey, auth_err := validateAuth(ctx)
	if auth_err != nil {
		return &AddDataResult{
			IsSuccess: false,
			Log:       auth_err.Error(),
		}, auth_err
	}

	// Check input data
	if err := validateAddDataRequest(req); err != nil {
		// Invalid data
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		fmt.Println("Error: see log/gRPC/fail folder for details.")
		return &AddDataResult{
			IsSuccess: false,
			Log:       err.Error(),
		}, status.Errorf(codes.InvalidArgument, "Invalid input data")
	} else {
		//* Valid data
		// check block validation
		message := db.MessageMaker(db.BlockData{
			SenderUID:    utils.ToMongoBinary(req.SenderUID),
			SenderPubKey: utils.ToMongoBinary(req.SenderPubKey),
			ReceiverUID:  utils.ToMongoBinary(req.ReceiverUID),
			UseContract:  utils.ToMongoBinary(req.UseContract),
			TimeStamp:    req.TimeStamp,
		})

		valid, validation_err := utils.Verify(req.SenderPubKey, req.SenderUID, message.Data, req.Signature)

		if !valid {
			// Block validation failed
			logger.GRPC_F_LOGGER.Printf("WARNING!!!: Error in data %v signature validation from %v:\n%v", req, apiKey, validation_err)
			fmt.Println("Error: see log/gRPC/fail folder for details.")
			return &AddDataResult{
				IsSuccess: false,
				Log:       validation_err.Error(),
			}, nil
		} else {
			// valid signature
			// find last sender block for make preHashBlock
			var preBlock db.Block
			sender_last_blocks, sender_last_blocks_err := db.FindManyBlocksLimited(bson.M{"blockData.senderUid": req.SenderUID}, 0, 1)

			if sender_last_blocks_err != nil {
				if sender_last_blocks_err == mongo.ErrNoDocuments {
					preBlock.ID = config.FIRST_BLOCK_HASH
				} else {
					logger.INTERNAL_LOGGER.Printf("Error: Error in finding many blocks for uid %v from api key %v. Error: %v", req.SenderUID, apiKey, sender_last_blocks_err)
					fmt.Println("Error: see log/internal folder for details.")
					return &AddDataResult{
						IsSuccess: false,
						Log:       "Internal server error",
					}, nil
				}
			} else {
				preBlock = sender_last_blocks[0]
			}

			// make block to add in db
			var block db.Block = db.Block{
				Boycott: false,
				BlockMeta: db.BlockMeta{
					PreBlockHash: preBlock.ID,
					NodeUID:      config.SELF_UID,
					TimeStamp:    utils.NowTimeInt64UnixMilli(),
				},
				BlockData: db.BlockData{
					SenderUID:    utils.ToMongoBinary(req.SenderUID),
					SenderPubKey: utils.ToMongoBinary(req.SenderPubKey),
					Signature:    utils.ToMongoBinary(req.Signature),
					ReceiverUID:  utils.ToMongoBinary(req.ReceiverUID),
					UseContract:  utils.ToMongoBinary(req.UseContract),
					TimeStamp:    req.TimeStamp,
				},
			}

			// If this block includes a smart contract (wasm) upload, decode and save the wasm file
			if len(req.ContractBase64) != 0 {
				wasmBytes, err := base64.StdEncoding.DecodeString(string(req.ContractBase64))
				if err != nil {
					logger.GRPC_F_LOGGER.Printf("Invalid base64 wasm from %v: %v", apiKey, err)
					fmt.Println("Error: see log/gRPC/fail folder for details.")
					return &AddDataResult{IsSuccess: false, Log: "Invalid wasm data"}, nil
				}

				// Enforce 1MB size limit for wasm
				if len(wasmBytes) > 1024*1024 {
					logger.GRPC_F_LOGGER.Printf("WASM too large from %v: %d bytes", apiKey, len(wasmBytes))
					fmt.Println("Error: see log/gRPC/fail folder for details.")
					return &AddDataResult{IsSuccess: false, Log: "wasm file must be lesser than 1024KB"}, nil
				}

				// Compute checksum and save file to SMART_CONTRACT_UPLOAD_PATH
				// if utils.FileCheckSumSHA256(req.ContractData.Checksum) {
				// 	path := config.SMART_CONTRACT_UPLOAD_PATH + req.ContractData.Checksum
				// 	if err := os.WriteFile(path, wasmBytes, 0644); err != nil {
				// 		logger.INTERNAL_LOGGER.Printf("Error saving wasm file %v: %v", path, err)
				// 		fmt.Println("Error: see log/internal folder for details.")
				// 		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
				// 	}
				// } else {
				// 	return &AddDataResult{IsSuccess: false, Log: "Checksum not match"}, nil
				// }

			}

			db.BlockHashMaker(&block)

			db_blocks, _ := db.FindAllBlocks(bson.M{"_id": block.ID})

			// No genesis block
			if len(db_blocks) == 0 {
				// Add Block to local db
				Block_insert_result, Block_insert_result_err := db.InsertOneBlock(block)

				if !Block_insert_result {
					// Internal error to add data to DB
					logger.INTERNAL_LOGGER.Printf("Error: Error in adding block '%v' to db from user '%v' : \n %v", block.ID, req.SenderUID, Block_insert_result_err)
					fmt.Println("Error: see log/internal folder for details.")
					return &AddDataResult{
						IsSuccess: false,
						Log:       "Internal server error",
					}, nil
				} else {
					// Successfully added data to DB
					logger.GRPC_S_LOGGER.Printf("Success: Block '%v' successfully added from user '%v' ", block.ID, req.SenderUID)

					//! TODO: Send new block data to other nodes
				}
			}

			return &AddDataResult{
				IsSuccess: true,
				Log:       "Data successfully added.",
				BlockHash: block.ID.Data,
			}, nil

		}

	}

}

func (s *ReadDataService) ReadData(ctx context.Context, req *ReadDataRequest) (*ReadDataResult, error) {

	//* Invalid data
	// Check auth from metadata
	apiKey, auth_err := validateAuth(ctx)
	if auth_err != nil {
		logger.GRPC_F_LOGGER.Println("Auth failed:", auth_err)
		fmt.Println("Error: see log/gRPC/fail folder for details.")
		return &ReadDataResult{
			IsSuccess: false,
			Log:       auth_err.Error(),
		}, auth_err
	}

	// Check input data
	if err := validateReadDataRequest(req); err != nil {
		// Invalid data
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		fmt.Println("Error: see log/gRPC/fail folder for details.")
		return &ReadDataResult{
			IsSuccess: false,
			Log:       err.Error(),
		}, status.Errorf(codes.InvalidArgument, "Invalid input data")
	} else {
		//* Valid data

		filter := []bson.M{}

		filter = append(filter, bson.M{"boycott": false})

		if len(req.UID) != 0 {
			filter = append(filter, bson.M{"$or": []bson.M{
				{"blockData.senderUid": req.UID},
				{"blockData.receiverUid": req.UID},
			}})
		}
		if len(req.SenderUID) != 0 {
			filter = append(filter, bson.M{"blockData.senderUid": req.SenderUID})
		}
		if len(req.SenderPubKey) != 0 {
			filter = append(filter, bson.M{"blockData.senderPubKey": req.SenderPubKey})
		}
		if len(req.ReceiverUID) != 0 {
			filter = append(filter, bson.M{"blockData.receiverUid": req.ReceiverUID})
		}
		if len(req.BlockHash) != 0 {
			filter = append(filter, bson.M{"_id": req.BlockHash})
		}
		if len(req.PreBlockHash) != 0 {
			filter = append(filter, bson.M{"blockMeta.preBlockHash": req.PreBlockHash})
		}
		if len(req.NodeUID) != 0 {
			filter = append(filter, bson.M{"blockMeta.nodeUid": req.NodeUID})
		}
		if req.TimeStampFrom != 0 || req.TimeStampTo != 0 {
			timeFilter := bson.M{}
			if req.TimeStampFrom != 0 {
				timeFilter["$gte"] = req.TimeStampFrom
			}
			if req.TimeStampTo != 0 {
				timeFilter["$lte"] = req.TimeStampTo
			}
			filter = append(filter, bson.M{"blockData.timeStamp": timeFilter})
		}
		if len(req.UseContract) != 0 {
			filter = append(filter, bson.M{"blockData.useContract": req.UseContract})
		}

		blocks, err := db.FindManyBlocksLimited(bson.M{"$and": filter}, req.Skip, req.Limit)

		if err != nil {

			if err == mongo.ErrNoDocuments || len(blocks) == 0 {
				// Not found any block with this hash
				logger.GRPC_S_LOGGER.Printf("Success: Not found any block with details '%v' for api key '%v'", req, apiKey)
				return &ReadDataResult{
					IsSuccess: true,
					Log:       "Not found any block with these details",
				}, nil
			} else {
				// Error in finding blocks
				logger.GRPC_F_LOGGER.Printf("Error: Error in finding blocks based on request '%v', by api key '%v'. Error: %v", req, apiKey, err)
				fmt.Println("Error: see log/gRPC/fail folder for details.")
				return &ReadDataResult{
					IsSuccess: false,
					Log:       err.Error(),
				}, err
			}

		} else {

			// Send founded block
			var grpcBlocks []*Block
			for _, dbBlock := range blocks {

				//* Valid data
				// check block validation
				message := db.MessageMaker(dbBlock.BlockData)

				valid, validation_err := utils.Verify(dbBlock.BlockData.SenderPubKey.Data, dbBlock.BlockData.SenderUID.Data, message.Data, dbBlock.BlockData.Signature.Data)

				if valid {
					grpcBlocks = append(grpcBlocks, &Block{
						BlockHash: dbBlock.ID.Data,
						BlockMeta: &BlockMeta{
							PreBlockHash: dbBlock.BlockMeta.PreBlockHash.Data,
							NodeUID:      dbBlock.BlockMeta.NodeUID.Data,
							TimeStamp:    dbBlock.BlockMeta.TimeStamp,
						},
						BlockData: &BlockData{
							SenderUID:    dbBlock.BlockData.SenderUID.Data,
							SenderPubKey: dbBlock.BlockData.SenderPubKey.Data,
							Signature:    dbBlock.BlockData.Signature.Data,
							ReceiverUID:  dbBlock.BlockData.ReceiverUID.Data,
							UseContract:  dbBlock.BlockData.UseContract.Data,
							TimeStamp:    dbBlock.BlockData.TimeStamp,
						},
					})
				} else {
					logger.GRPC_F_LOGGER.Printf("WARNING!!! : Block validation error in database. Block data:\n%v\nError:\n%v", dbBlock, validation_err)
					fmt.Println("Error: see log/gRPC/fail folder for details.")
				}
			}

			logger.GRPC_F_LOGGER.Printf("Success: Send blocks to api key '%v'. Blocks: \n %v", apiKey, blocks)
			fmt.Println("Error: see log/gRPC/fail folder for details.")
			return &ReadDataResult{
				IsSuccess: true,
				Log:       "Success",
				Data:      grpcBlocks,
			}, nil
		}
	}
}
