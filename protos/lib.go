package protos

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"blvchain/core/utils"
	context "context"
	"errors"

	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

func validateAddDataRequest(req *BlockData) error {

	if utils.E_str(req.SenderUID, 32) {
		return errors.New("senderUID is required and must be 32 len string")
	}

	if utils.Bt_int64(req.SenderRole, 0, 10000001) {
		return errors.New("senderRole is required and must be bigger than zero")
	}

	if utils.E_str(req.SenderPubKey, 66) {
		return errors.New("senderPubKey is required and must be 66 len string")
	}

	if utils.E_str(req.Signature, 128) {
		return errors.New("signature is required and must be 128 len string")
	}

	if utils.E_str(req.ReceiverUID, 32) {
		return errors.New("receiverUID is required and must be 32 len string")
	}

	if utils.Bt_int64(req.ReceiverRole, 0, 10000001) {
		return errors.New("receiverRole is required and must be bigger than zero")
	}

	// General data size validation
	if utils.Lt_float(utils.StringSizeInKB(req.Data), utils.StringToFloat64(config.MAX_DATA_SIZE_KB)) {
		errStr := "data is required and must be lesser than " + config.MAX_DATA_SIZE_KB + "KB"
		return errors.New(errStr)
	}

	// If this is a smart contract upload, enforce a 1MB (1024KB) limit for the Wasm file
	if req.UseContract != "" {
		// Validate UseContract identifier length (if required by protocol)
		if utils.E_str(req.UseContract, 66) {
			return errors.New("useContract must be 66 len string")
		}

		// If Data contains the wasm file (base64 encoded), ensure it is <= 1MB
		if req.Data != "" {
			if utils.StringSizeInKB(req.Data) > 1024 {
				return errors.New("wasm file must be lesser than 1024KB")
			}
		}
	}

	if utils.Bt_int64(req.TimeStamp, int64(1262304000000), int64(9262304000000)) {
		return errors.New("timeStamp must be a valid unix format with milliseconds")
	}

	return nil
}

func validateReadDataRequest(req *ReadDataRequest) error {

	if utils.Bt_int64(req.Limit, 0, 101) {
		return errors.New("limit must be between 1-100")
	}

	if req.Skip < 0 {
		return errors.New("skip must be zero or bigger than zero")
	}

	if utils.E_str(req.SenderUID, 32) &&
		utils.E_str(req.UID, 32) &&
		utils.Bt_int64(req.SenderRole, 0, 10000001) &&
		utils.E_str(req.SenderPubKey, 66) &&
		utils.E_str(req.ReceiverUID, 32) &&
		utils.Bt_int64(req.ReceiverRole, 0, 10000001) &&
		utils.E_str(req.BlockHash, 64) &&
		utils.E_str(req.PreBlockHash, 64) &&
		utils.Gt_str(req.NodeUID, 9) &&
		utils.Bt_int64(req.TimeStampFrom, 1262304000, 9262304000) &&
		utils.Bt_int64(req.TimeStampTo, 1262304000, 9262304000) &&
		utils.E_str(req.UseContract, 66) {
		return errors.New("no filters provided in the request / provided filters are not correct")
	}

	return nil
}

func validateAuth(ctx context.Context) (string, error) {
	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.GRPC_F_LOGGER.Println("Missing metadata from ")
		return "", status.Errorf(codes.Unauthenticated, "Missing metadata")
	}

	// Get API key from metadata
	apiKeys := md["auth"]
	if len(apiKeys) == 0 {
		logger.GRPC_F_LOGGER.Println("Missing API key")
		return "", status.Errorf(codes.Unauthenticated, "Missing API key")
	}

	apiKey := apiKeys[0]
	if !config.API_KEY_LIST[apiKey] {
		logger.GRPC_F_LOGGER.Println("Unauthorized client")
		return apiKey, status.Errorf(codes.PermissionDenied, "Unauthorized client")
	}

	return apiKey, nil
}
