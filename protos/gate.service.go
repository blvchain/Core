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
			sender_last_blocks, sender_last_blocks_err := db.FindManyBlocksLimited(bson.M{"senderuid": req.SenderUID}, config.ZERO_STRING, config.ONE_STRING)

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

		// message ReadDataRequest {
		// string Method = 1;
		// string Filter = 2;
		// int32 Limit = 3;
		// int32 Skisp = 4;
		// }

		return &ReadDataResult{
			IsSuccess: true,
			Log:       "Read successful.",
			// Data:      "Sample data...",
		}, nil
	}

}
