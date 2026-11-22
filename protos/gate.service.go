package protos

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/utils"
	"blvchain/core/ws"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

func (s *AddDataService) AddData(ctx context.Context, req *BlockData) (*AddDataResult, error) {
	// ------------------------------------------------------------------
	// 1. Authenticate
	// ------------------------------------------------------------------
	apiKey, authErr := validateAuth(ctx)
	if authErr != nil {
		return &AddDataResult{IsSuccess: false, Log: authErr.Error()}, authErr
	}

	// ------------------------------------------------------------------
	// 2. Validate the request shape
	// ------------------------------------------------------------------
	if err := validateAddDataRequest(req); err != nil {
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		return &AddDataResult{IsSuccess: false, Log: err.Error()},
			status.Error(codes.InvalidArgument, "Invalid input data")
	}

	// ------------------------------------------------------------------
	// 3. Verify the sender's signature
	// ------------------------------------------------------------------
	message := db.MessageMaker(blockDataFromRequest(req))
	valid, verifyErr := utils.Verify(req.GetSenderPubKey(), req.GetSenderUID(), message, req.GetSignature())
	if verifyErr != nil || !valid {
		if verifyErr == nil {
			verifyErr = errors.New("signature is not valid")
		}
		logger.GRPC_F_LOGGER.Printf("WARNING!!!: Signature validation failed for %v from api key %v: %v", req.GetSenderUID(), apiKey, verifyErr)
		return &AddDataResult{IsSuccess: false, Log: verifyErr.Error()}, nil
	}

	// ------------------------------------------------------------------
	// 4. Find the sender's previous block to link the chain
	// ------------------------------------------------------------------
	preBlockID := config.FIRST_BLOCK_HASH
	lastBlocks, findErr := db.FindManyBlocksLimited(bson.M{"blockData.senderUid": req.GetSenderUID()}, 0, 1)
	if findErr != nil {
		logger.INTERNAL_LOGGER.Printf("Error: could not load the last block of sender %v (api key %v): %v", req.GetSenderUID(), apiKey, findErr)
		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
	}
	if len(lastBlocks) > 0 {
		preBlockID = lastBlocks[0].ID
	}

	if preBlockID == "" {
		logger.INTERNAL_LOGGER.Println("Error: genesis hash is not set; refusing to accept blocks")
		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
	}

	// ------------------------------------------------------------------
	// 5. Store an uploaded smart contract, if any
	// ------------------------------------------------------------------
	contractData := db.Contract{}
	if req.GetContractData() != nil {
		contractData = db.Contract{
			Name:        req.GetContractData().GetName(),
			Version:     req.GetContractData().GetVersion(),
			Language:    req.GetContractData().GetLanguage(),
			Compiler:    req.GetContractData().GetCompiler(),
			Description: req.GetContractData().GetDescription(),
			Checksum:    req.GetContractData().GetChecksum(),
			Author:      req.GetContractData().GetAuthor(),
			License:     req.GetContractData().GetLicense(),
		}
	}
	if len(req.GetContractBase64()) > 0 {
		storedChecksum, storeErr := storeContract(req.GetContractBase64(), contractData.Checksum)
		if storeErr != nil {
			logger.GRPC_F_LOGGER.Printf("Contract upload rejected for api key %v: %v", apiKey, storeErr)
			return &AddDataResult{IsSuccess: false, Log: storeErr.Error()}, nil
		}
		contractData.Checksum = storedChecksum
		logger.SC_S_LOGGER.Printf("Success: contract %v stored for api key %v", storedChecksum, apiKey)
	}

	// ------------------------------------------------------------------
	// 6. Build the block
	// ------------------------------------------------------------------
	block := db.Block{
		Boycott: false,
		BlockMeta: db.BlockMeta{
			PreBlockHash: preBlockID,
			NodeUID:      config.SELF_UID,
			TimeStamp:    utils.NowTimeInt64UnixMilli(),
		},
		BlockData: db.BlockData{
			SenderUID:    req.GetSenderUID(),
			SenderRole:   req.GetSenderRole(),
			SenderPubKey: req.GetSenderPubKey(),
			Signature:    req.GetSignature(),
			ReceiverUID:  req.GetReceiverUID(),
			ReceiverRole: req.GetReceiverRole(),
			Data:         req.GetData(),
			UseContract:  req.GetUseContract(),
			ContractData: contractData,
			TimeStamp:    req.GetTimeStamp(),
		},
	}

	nodePrivKey, keyErr := config.LoadNodePrivateKey()
	if keyErr != nil {
		logger.INTERNAL_LOGGER.Printf("Error: no node key configured: %v", keyErr)
		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
	}

	db.BlockHashMaker(&block, block.BlockMeta.NodeUID)
	if err := db.SignBlock(&block, nodePrivKey); err != nil {
		logger.INTERNAL_LOGGER.Printf("Error: could not sign block '%v': %v", block.ID, err)
		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
	}

	// ------------------------------------------------------------------
	// 7. Persist
	//
	// A retried request carries the same signature, so look it up before doing
	// any work: re-submitting a block must not create a second one.
	// ------------------------------------------------------------------
	existing, findErr := db.FindAllBlocks(bson.M{
		"blockData.senderUid": req.GetSenderUID(),
		"blockData.signature": req.GetSignature(),
	})
	if findErr != nil {
		logger.INTERNAL_LOGGER.Printf("Error: could not check for a duplicate of '%v': %v", req.GetSenderUID(), findErr)
		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
	}

	if len(existing) > 0 {
		logger.GRPC_S_LOGGER.Printf("Block '%v' already exists (api key %v); treating the request as a successful retry", existing[0].ID, apiKey)
		return &AddDataResult{IsSuccess: true, Log: "Block already exists.", BlockHash: existing[0].ID}, nil
	}

	ok, insertErr := db.InsertOneBlock(block)
	if !ok {
		// A concurrent retry may have won the race. Treat it as success.
		existing, lookupErr := db.FindAllBlocks(bson.M{"_id": block.ID})
		if lookupErr == nil && len(existing) > 0 {
			return &AddDataResult{IsSuccess: true, Log: "Block already exists.", BlockHash: block.ID}, nil
		}

		logger.INTERNAL_LOGGER.Printf("Error: could not add block '%v' from user '%v': %v", block.ID, req.GetSenderUID(), insertErr)
		return &AddDataResult{IsSuccess: false, Log: "Internal server error"}, nil
	}

	logger.GRPC_S_LOGGER.Printf("Success: Block '%v' added from user '%v' (api key %v)", block.ID, req.GetSenderUID(), apiKey)

	// Replicate to the peers. Failures are logged by the ws package and do not
	// roll back the local write.
	ws.ClientManagerVar.AddBlockToServers(block)

	return &AddDataResult{
		IsSuccess: true,
		Log:       "Data successfully added.",
		BlockHash: block.ID,
	}, nil
}

// blockDataFromRequest converts the wire type into the storage type.
func blockDataFromRequest(req *BlockData) db.BlockData {
	contractData := db.Contract{}
	if req.GetContractData() != nil {
		contractData = db.Contract{
			Name:        req.GetContractData().GetName(),
			Version:     req.GetContractData().GetVersion(),
			Language:    req.GetContractData().GetLanguage(),
			Compiler:    req.GetContractData().GetCompiler(),
			Description: req.GetContractData().GetDescription(),
			Checksum:    req.GetContractData().GetChecksum(),
			Author:      req.GetContractData().GetAuthor(),
			License:     req.GetContractData().GetLicense(),
		}
	}

	return db.BlockData{
		SenderUID:    req.GetSenderUID(),
		SenderRole:   req.GetSenderRole(),
		SenderPubKey: req.GetSenderPubKey(),
		Signature:    req.GetSignature(),
		ReceiverUID:  req.GetReceiverUID(),
		ReceiverRole: req.GetReceiverRole(),
		Data:         req.GetData(),
		UseContract:  req.GetUseContract(),
		ContractData: contractData,
		TimeStamp:    req.GetTimeStamp(),
	}
}

// storeContract validates and persists an uploaded WASM binary. It returns the
// SHA-256 checksum that the file was stored under.
//
// When the client supplies a checksum it must match the bytes it sent; the
// checksum (not a client-chosen filename) is used as the on-disk name, so a
// caller cannot write outside the contract directory.
func storeContract(encoded []byte, expectedChecksum string) (string, error) {
	wasmBytes, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", errors.New("invalid wasm data")
	}
	if int64(len(wasmBytes)) > config.MAX_WASM_SIZE {
		return "", fmt.Errorf("wasm file must be lesser than %dKB", config.MAX_WASM_SIZE_KB)
	}

	checksum := utils.SHA256Hex(wasmBytes)
	if expectedChecksum != "" && !strings.EqualFold(expectedChecksum, checksum) {
		logger.SC_F_LOGGER.Printf("Contract checksum mismatch: client sent %v, computed %v", expectedChecksum, checksum)
		return "", errors.New("Checksum not match")
	}

	if err := os.MkdirAll(config.SMART_CONTRACT_DIR, 0o755); err != nil {
		return "", fmt.Errorf("internal: could not create contract directory: %w", err)
	}

	path := filepath.Join(config.SMART_CONTRACT_DIR, checksum+".wasm")
	if err := os.WriteFile(path, wasmBytes, 0o644); err != nil {
		return "", fmt.Errorf("internal: could not store contract: %w", err)
	}

	return checksum, nil
}

func (s *ReadDataService) ReadData(ctx context.Context, req *ReadDataRequest) (*ReadDataResult, error) {
	// ------------------------------------------------------------------
	// 1. Authenticate
	// ------------------------------------------------------------------
	apiKey, authErr := validateAuth(ctx)
	if authErr != nil {
		logger.GRPC_F_LOGGER.Println("Auth failed:", authErr)
		return &ReadDataResult{IsSuccess: false, Log: authErr.Error()}, authErr
	}

	// ------------------------------------------------------------------
	// 2. Validate the request shape
	// ------------------------------------------------------------------
	if err := validateReadDataRequest(req); err != nil {
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		return &ReadDataResult{IsSuccess: false, Log: err.Error()},
			status.Error(codes.InvalidArgument, "Invalid input data")
	}

	// ------------------------------------------------------------------
	// 3. Build the Mongo filter
	// ------------------------------------------------------------------
	filter := []bson.M{{"boycott": false}}

	if req.GetUID() != "" {
		filter = append(filter, bson.M{"$or": []bson.M{
			{"blockData.senderUid": req.GetUID()},
			{"blockData.receiverUid": req.GetUID()},
		}})
	}
	if req.GetSenderUID() != "" {
		filter = append(filter, bson.M{"blockData.senderUid": req.GetSenderUID()})
	}
	if req.GetSenderRole() != 0 {
		filter = append(filter, bson.M{"blockData.senderRole": req.GetSenderRole()})
	}
	if req.GetSenderPubKey() != "" {
		filter = append(filter, bson.M{"blockData.senderPubKey": req.GetSenderPubKey()})
	}
	if req.GetReceiverUID() != "" {
		filter = append(filter, bson.M{"blockData.receiverUid": req.GetReceiverUID()})
	}
	if req.GetReceiverRole() != 0 {
		filter = append(filter, bson.M{"blockData.receiverRole": req.GetReceiverRole()})
	}
	if req.GetBlockHash() != "" {
		filter = append(filter, bson.M{"_id": req.GetBlockHash()})
	}
	if req.GetPreBlockHash() != "" {
		filter = append(filter, bson.M{"blockMeta.preBlockHash": req.GetPreBlockHash()})
	}
	if req.GetNodeUID() != "" {
		filter = append(filter, bson.M{"blockMeta.nodeUid": req.GetNodeUID()})
	}
	if req.GetTimeStampFrom() != 0 || req.GetTimeStampTo() != 0 {
		timeFilter := bson.M{}
		if req.GetTimeStampFrom() != 0 {
			timeFilter["$gte"] = req.GetTimeStampFrom()
		}
		if req.GetTimeStampTo() != 0 {
			timeFilter["$lte"] = req.GetTimeStampTo()
		}
		filter = append(filter, bson.M{"blockData.timeStamp": timeFilter})
	}
	if req.GetUseContract() != "" {
		filter = append(filter, bson.M{"blockData.useContract": req.GetUseContract()})
	}

	blocks, findErr := db.FindManyBlocksLimited(bson.M{"$and": filter}, req.GetSkip(), req.GetLimit())
	if findErr != nil {
		logger.GRPC_F_LOGGER.Printf("Error: could not read blocks for request '%v' (api key '%v'): %v", req, apiKey, findErr)
		return &ReadDataResult{IsSuccess: false, Log: findErr.Error()}, findErr
	}

	if len(blocks) == 0 {
		logger.GRPC_S_LOGGER.Printf("Success: no block matched '%v' for api key '%v'", req, apiKey)
		return &ReadDataResult{
			IsSuccess: true,
			Log:       "Not found any block with these details",
		}, nil
	}

	// ------------------------------------------------------------------
	// 4. Verify every block before returning it
	// ------------------------------------------------------------------
	grpcBlocks := make([]*Block, 0, len(blocks))
	for _, dbBlock := range blocks {
		// Re-validate on read: a record whose contents, client signature or
		// node attestation no longer matches its ID must not be served.
		if err := db.BlockValidator(dbBlock); err != nil {
			logger.GRPC_F_LOGGER.Printf("WARNING!!!: Block %v failed validation: %v", dbBlock.ID, err)
			continue
		}

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
				UseContract:  dbBlock.BlockData.UseContract,
				ContractData: &Contract{
					Name:        dbBlock.BlockData.ContractData.Name,
					Version:     dbBlock.BlockData.ContractData.Version,
					Language:    dbBlock.BlockData.ContractData.Language,
					Compiler:    dbBlock.BlockData.ContractData.Compiler,
					Description: dbBlock.BlockData.ContractData.Description,
					Checksum:    dbBlock.BlockData.ContractData.Checksum,
					Author:      dbBlock.BlockData.ContractData.Author,
					License:     dbBlock.BlockData.ContractData.License,
				},
				TimeStamp: dbBlock.BlockData.TimeStamp,
			},
		})
	}

	logger.GRPC_S_LOGGER.Printf("Success: sent %v block(s) to api key '%v'", len(grpcBlocks), apiKey)

	return &ReadDataResult{
		IsSuccess: true,
		Log:       "",
		Data:      grpcBlocks,
	}, nil
}
