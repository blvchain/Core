package protos

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/utils"
	"blvchain/core/ws"
	context "context"

	"go.mongodb.org/mongo-driver/bson"
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
			req.Signature +
			req.ReceiverUID +
			utils.Int64ToStr(req.ReceiverRole) +
			req.Data +
			utils.Int64ToStr(req.TimeStamp)

		valid, _ := utils.Verify(req.SenderPubKey, message, req.Signature)

		if !valid {
			// Block validation failed
			logger.WS_F_LOGGER.Printf("WARNING!!!: Error in data %v signature validation from %v", req, apiKey)
			return &AddDataResult{
				IsSuccess: false,
				Log:       "Signature validation failed",
			}, nil
		} else {
			// valid signature
			// find last sernder block for make preHashBlock
			var preBlockHash string
			sender_last_blocks, sender_last_blocks_err := db.FindManyBlocksLimited(bson.M{"senderuid": req.SenderUID}, 0, 1)

			if sender_last_blocks_err != nil {
				logger.INTERNAL_LOGGER.Printf("Error: Error in finding many blocks for uid %v from api key %v", req.SenderUID, apiKey)
				return &AddDataResult{
					IsSuccess: false,
					Log:       "Internal server error",
				}, nil
			}

			if len(sender_last_blocks) == 0 {
				preBlockHash = config.FIRST_BLOCK_HASH
			} else {
				preBlockHash = sender_last_blocks[0].BlockHash
			}

			// make block to add in db
			var block db.Block = db.Block{
				BlockMeta: db.BlockMeta{
					PreBlockHash: preBlockHash,
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
			db.BlockHashMaker(&block)

			db_blocks, _ := db.FindAllBlocks(bson.M{"blockHash": block.BlockHash})

			// No genesis block
			if len(db_blocks) == 0 {
				// Add Block to local db
				Block_insert_result, Block_insert_result_err := db.InsertOneBlock(block)

				if !Block_insert_result {
					// Internal error to add data to DB
					logger.INTERNAL_LOGGER.Printf("Error: Error in adding block '%v' to db from user '%v' : \n %v", block.BlockHash, req.SenderUID, Block_insert_result_err)
					return &AddDataResult{
						IsSuccess: false,
						Log:       "Internal server error",
					}, nil
				} else {
					// Successfully added data to DB
					logger.WS_S_LOGGER.Printf("Success: Block '%v' successfully added from user '%v' ", block.BlockHash, req.SenderUID)

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

		filter := bson.M{}

		if req.SenderUID != "" {
			filter["blockData.senderUid"] = req.SenderUID
		}
		if req.SenderRole != 0 {
			filter["blockData.senderRole"] = req.SenderRole
		}
		if req.ReceiverUID != "" {
			filter["blockData.receiverUid"] = req.ReceiverUID
		}
		if req.ReceiverRole != 0 {
			filter["blockData.receiverRole"] = req.ReceiverRole
		}
		if req.BlockHash != "" {
			filter["blockHash"] = req.BlockHash
		}
		if req.PreBlockHash != "" {
			filter["blockMeta.preBlockHash"] = req.PreBlockHash
		}
		if req.TimeStampFrom != 0 || req.TimeStampTo != 0 {
			timeFilter := bson.M{}
			if req.TimeStampFrom != 0 {
				timeFilter["$gte"] = req.TimeStampFrom
			}
			if req.TimeStampTo != 0 {
				timeFilter["$lte"] = req.TimeStampTo
			}
			filter["blockData.timeStamp"] = timeFilter
		}

		blocks, err := db.FindManyBlocksLimited(filter, req.Skip, req.Limit)

		if err != nil {
			// Error in finding blocks
			logger.WS_F_LOGGER.Printf("Error: Error in finding blocks based on request '%v', by api key '%v'", req, apiKey)
			return &ReadDataResult{
				IsSuccess: false,
				Log:       err.Error(),
			}, err
		} else {

			if len(blocks) == 0 {
				// Not found any block with this hash
				logger.WS_S_LOGGER.Printf("Success: Not found any block with details '%v' for api key '%v'", req, apiKey)
				return &ReadDataResult{
					IsSuccess: true,
					Log:       "Not found any block with these details",
				}, nil

			} else {
				// Send founded block
				var grpcBlocks []*Block
				for _, dbBlock := range blocks {
					grpcBlocks = append(grpcBlocks, &Block{
						BlockHash: dbBlock.BlockHash,
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
				}

				logger.WS_S_LOGGER.Printf("Success: Send blocks to api key '%v'. Blocks: \n %v", apiKey, blocks)
				return &ReadDataResult{
					IsSuccess: true,
					Log:       "",
					Data:      grpcBlocks,
				}, nil
			}
		}
	}
}
