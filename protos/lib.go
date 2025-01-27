package protos

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	context "context"
	"errors"

	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

func validateAddDataRequest(req *BlockData) error {

	if req.SenderUID == "" {
		return errors.New("senderUID is required")
	}

	if req.SenderRole < 0 {
		return errors.New("senderRole must be bigger than zero")
	}

	if req.SenderPubKey == "" {
		return errors.New("senderPubKey is required")
	}

	if req.Signature == "" {
		return errors.New("signature is required")
	}

	if req.ReceiverUID == "" {
		return errors.New("receiverUID is required")
	}

	if req.ReceiverRole < 0 {
		return errors.New("receiverRole must be bigger than zero")
	}

	if req.Data == "" {
		return errors.New("data is required")
	}

	if req.TimeStamp <= 0 {
		return errors.New("timeStamp must be bigger than zero")
	}

	return nil
}

func validateReadDataRequest(req *ReadDataRequest) error {

	if req.Limit < 0 || req.Limit > 100 {
		return errors.New("limit must be between 1-100")
	}

	if req.Skip < 0 {
		return errors.New("skip must be zero or bigger than zero")
	}

	if req.SenderUID == "" &&
		req.SenderRole == 0 &&
		req.ReceiverUID == "" &&
		req.ReceiverRole == 0 &&
		req.BlockHash == "" &&
		req.PreBlockHash == "" &&
		req.TimeStampFrom == 0 &&
		req.TimeStampTo == 0 {
		return errors.New("no filters provided in the request")
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
