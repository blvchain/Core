package protos

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/utils"
	"blvchain/core/ws"
	context "context"
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
		return &AddDataResult{
			IsSuccess: false,
			Log:       err.Error(),
		}, status.Errorf(codes.InvalidArgument, "Invalid input data")
	} else {
		//* Valid data
		// check block validation
		message := req.SenderUID +
			utils.Int64ToStr(req.SenderRole) +
			req.SenderPubKey +
			req.ReceiverUID +
			utils.Int64ToStr(req.ReceiverRole) +
			req.Data +
			utils.Int64ToStr(req.TimeStamp)

		valid, validation_err := utils.Verify(req.SenderPubKey, req.SenderUID, message, req.Signature)
		fmt.Println("valid", valid)
		if !valid {
			// Block validation failed
			logger.GRPC_F_LOGGER.Printf("WARNING!!!: Error in data %v signature validation from %v:\n%v", req, apiKey, validation_err)
			return &AddDataResult{
				IsSuccess: false,
				Log:       validation_err.Error(),
			}, nil
		} else {
			// valid signature
			// find last sernder block for make preHashBlock
			var preBlock db.Block
			sender_last_blocks, sender_last_blocks_err := db.FindManyBlocksLimited(bson.M{"blockData.senderUid": req.SenderUID}, 0, 1)

			if sender_last_blocks_err != nil {
				if sender_last_blocks_err == mongo.ErrNoDocuments {
					preBlock.ID = config.FIRST_BLOCK_HASH
				} else {
					logger.INTERNAL_LOGGER.Printf("Error: Error in finding many blocks for uid %v from api key %v. Error: %v", req.SenderUID, apiKey, sender_last_blocks_err)
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
					SenderUID:    req.SenderUID,
					SenderRole:   req.SenderRole,
					SenderPubKey: req.SenderPubKey,
					Signature:    req.Signature,
					ReceiverUID:  req.ReceiverUID,
					ReceiverRole: req.ReceiverRole,
					Data:         req.Data,
					TimeStamp:    req.TimeStamp,
				},
			}
			db.BlockHashMaker(&block, block.BlockMeta.NodeUID)

			db_blocks, _ := db.FindAllBlocks(bson.M{"_id": block.ID})

			// No genesis block
			if len(db_blocks) == 0 {
				// Add Block to local db
				Block_insert_result, Block_insert_result_err := db.InsertOneBlock(block)

				if !Block_insert_result {
					// Internal error to add data to DB
					logger.INTERNAL_LOGGER.Printf("Error: Error in adding block '%v' to db from user '%v' : \n %v", block.ID, req.SenderUID, Block_insert_result_err)
					return &AddDataResult{
						IsSuccess: false,
						Log:       "Internal server error",
					}, nil
				} else {
					// Successfully added data to DB
					logger.GRPC_S_LOGGER.Printf("Success: Block '%v' successfully added from user '%v' ", block.ID, req.SenderUID)

					// Send new block data to other nodes
					ws.ClientManagerVar.AddBlockToServers(block)
				}
			}

		}

		return &AddDataResult{
			IsSuccess: true,
			Log:       "Data successfully added.",
		}, nil

	}

}

func (s *ReadDataService) ReadData(ctx context.Context, req *ReadDataRequest) (*ReadDataResult, error) {

	//* Invalid data
	// Check auth from metadata
	apiKey, auth_err := validateAuth(ctx)
	if auth_err != nil {
		logger.GRPC_F_LOGGER.Println("Auth failed:", auth_err)
		return &ReadDataResult{
			IsSuccess: false,
			Log:       auth_err.Error(),
		}, auth_err
	}

	// Check input data
	if err := validateReadDataRequest(req); err != nil {
		// Invalid data
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		return &ReadDataResult{
			IsSuccess: false,
			Log:       err.Error(),
		}, status.Errorf(codes.InvalidArgument, "Invalid input data")
	} else {
		//* Valid data

		filter := []bson.M{}

		filter = append(filter, bson.M{"boycott": false})

		if req.SenderUID != "" {
			filter = append(filter, bson.M{"blockData.senderUid": req.SenderUID})
		}
		if req.SenderRole != 0 {
			filter = append(filter, bson.M{"blockData.senderRole": req.SenderRole})
		}
		if req.SenderPubKey != "" {
			filter = append(filter, bson.M{"blockData.senderPubKey": req.SenderPubKey})
		}
		if req.ReceiverUID != "" {
			filter = append(filter, bson.M{"blockData.receiverUid": req.ReceiverUID})
		}
		if req.ReceiverRole != 0 {
			filter = append(filter, bson.M{"blockData.receiverRole": req.ReceiverRole})
		}
		if req.BlockHash != "" {
			filter = append(filter, bson.M{"_id": req.BlockHash})
		}
		if req.PreBlockHash != "" {
			filter = append(filter, bson.M{"blockMeta.preBlockHash": req.PreBlockHash})
		}
		if req.NodeUID != "" {
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
				message := dbBlock.BlockData.SenderUID +
					utils.Int64ToStr(dbBlock.BlockData.SenderRole) +
					dbBlock.BlockData.SenderPubKey +
					dbBlock.BlockData.ReceiverUID +
					utils.Int64ToStr(dbBlock.BlockData.ReceiverRole) +
					dbBlock.BlockData.Data +
					utils.Int64ToStr(dbBlock.BlockData.TimeStamp)

				valid, validation_err := utils.Verify(dbBlock.BlockData.SenderPubKey, dbBlock.BlockData.SenderUID, message, dbBlock.BlockData.Signature)

				if valid {
					grpcBlocks = append(grpcBlocks, &Block{
						BlockHash: dbBlock.ID,
						BlockMeta: &BlockMeta{
							PreBlockHash: dbBlock.BlockMeta.PreBlockHash,
							NodeUID:      dbBlock.BlockMeta.NodeUID,
							TimeStamp:    dbBlock.BlockMeta.TimeStamp,
						},
						BlockData: &BlockData{
							SenderUID:    dbBlock.BlockData.SenderUID,
							SenderRole:   dbBlock.BlockData.SenderRole,
							SenderPubKey: dbBlock.BlockData.SenderPubKey,
							Signature:    dbBlock.BlockData.Signature,
							ReceiverUID:  dbBlock.BlockData.ReceiverUID,
							ReceiverRole: dbBlock.BlockData.ReceiverRole,
							Data:         dbBlock.BlockData.Data,
							TimeStamp:    dbBlock.BlockData.TimeStamp,
						},
					})
				} else {
					logger.GRPC_F_LOGGER.Printf("WARNING!!! : Block validation error in database. Block data:\n%v\nErro:\n%v", dbBlock, validation_err)
				}
			}

			logger.GRPC_F_LOGGER.Printf("Success: Send blocks to api key '%v'. Blocks: \n %v", apiKey, blocks)
			return &ReadDataResult{
				IsSuccess: true,
				Log:       "",
				Data:      grpcBlocks,
			}, nil
		}
	}
}
