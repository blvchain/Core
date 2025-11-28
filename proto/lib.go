package proto

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"blvchain/core/utils"
	context "context"
	"errors"
	"fmt"

	validator "github.com/go-playground/validator/v10"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// validateAddDataRequest validates BlockData using go-playground/validator for
// straightforward checks (length / numeric bounds) and keeps manual checks for
// things like data size, wasm size, and timestamp windows.
func validateAddDataRequest(req *BlockData) error {

	// Basic string length and required checks using validator.Var
	if err := validate.Var(req.SenderUID, "required,len=32"); err != nil {
		return errors.New("senderUID is required and must be 32 len string")
	}

	if err := validate.Var(req.SenderPubKey, "required,len=66"); err != nil {
		return errors.New("senderPubKey is required and must be 66 len string")
	}

	if err := validate.Var(req.Signature, "required,len=128"); err != nil {
		return errors.New("signature is required and must be 128 len string")
	}

	if err := validate.Var(req.ReceiverUID, "required,len=32"); err != nil {
		return errors.New("receiverUID is required and must be 32 len string")
	}

	// If this is a smart contract upload, enforce a 1MB (1024KB) limit for the Wasm file
	if req.UseContract != "" {
		// Validate UseContract identifier length
		if err := validate.Var(req.UseContract, "len=66"); err != nil {
			return errors.New("useContract must be 66 len string")
		}
	}

	// TimeStamp: must be a valid unix ms timestamp between provided boundaries
	// (1262304000000 .. 9262304000000)
	if req.TimeStamp == 0 {
		return errors.New("timeStamp must be a valid unix format with milliseconds")
	}
	if req.TimeStamp < int64(1262304000000) || req.TimeStamp > int64(9262304000000) {
		return errors.New("timeStamp must be a valid unix format with milliseconds")
	}

	return nil
}

// validateReadDataRequest validates ReadDataRequest.
// limit and skip are required; other fields are optional. If an optional
// filter is provided it must be valid (wrong lengths / ranges return errors).
// If no optional filters are provided, return an error as well.
func validateReadDataRequest(req *ReadDataRequest) error {

	// limit must be between 1 and 100 inclusive (original said 1-100)
	if err := validate.Var(req.Limit, "gt=0,lt=101"); err != nil {
		return errors.New("limit must be between 1-100")
	}

	if req.Skip < 0 {
		return errors.New("skip must be zero or bigger than zero")
	}

	// Track whether the client provided at least one filter
	provided := false

	// skip limit
	if req.Skip >= 0 && req.Limit > 0 {
		provided = true
	}

	// UID
	if req.UID != "" {
		provided = true
		if len(req.UID) != 32 {
			return errors.New("uid must be 32 len string")
		}
	}

	// SenderUID
	if req.SenderUID != "" {
		provided = true
		if len(req.SenderUID) != 32 {
			return errors.New("senderUID must be 32 len string")
		}
	}

	// SenderPubKey
	if req.SenderPubKey != "" {
		provided = true
		if len(req.SenderPubKey) != 66 {
			return errors.New("senderPubKey must be 66 len string")
		}
	}

	// ReceiverUID
	if req.ReceiverUID != "" {
		provided = true
		if len(req.ReceiverUID) != 32 {
			return errors.New("receiverUID must be 32 len string")
		}
	}

	// BlockHash
	if req.BlockHash != "" {
		provided = true
		if len(req.BlockHash) != 64 {
			return errors.New("blockHash must be 64 len string")
		}
	}

	// PreBlockHash
	if req.PreBlockHash != "" {
		provided = true
		if len(req.PreBlockHash) != 64 {
			return errors.New("preBlockHash must be 64 len string")
		}
	}

	// NodeUID (original used utils.Gt_str(req.NodeUID, 9) — require length > 9)
	if req.NodeUID != "" {
		provided = true
		if len(req.NodeUID) <= 9 {
			return errors.New("nodeUID must be longer than 9 characters")
		}
	}

	// TimeStampFrom / TimeStampTo: original used 1262304000..9262304000 (seconds) bounds
	if req.TimeStampFrom != 0 {
		provided = true
		if req.TimeStampFrom < 1262304000 || req.TimeStampFrom > 9262304000 {
			return errors.New("timeStampFrom must be a valid unix format in seconds")
		}
	}
	if req.TimeStampTo != 0 {
		provided = true
		if req.TimeStampTo < 1262304000 || req.TimeStampTo > 9262304000 {
			return errors.New("timeStampTo must be a valid unix format in seconds")
		}
	}

	// UseContract
	if req.UseContract != "" {
		provided = true
		if len(req.UseContract) != 66 {
			return errors.New("useContract must be 66 len string")
		}
	}

	if !provided {
		return errors.New("no filters provided in the request / provided filters are not correct")
	}

	return nil
}

// validateAuth remains mostly the same, using metadata to get the API key.
func validateAuth(ctx context.Context) (string, error) {
	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.GRPC_F_LOGGER.Println("Missing metadata from ")
		fmt.Println("Error: see log/gRPC/fail folder for details.")
		return "", status.Errorf(codes.Unauthenticated, "Missing metadata")
	}

	// Get API key from metadata
	apiKeys := md["auth"]
	if len(apiKeys) == 0 {
		logger.GRPC_F_LOGGER.Println("Missing API key")
		fmt.Println("Error: see log/gRPC/fail folder for details.")
		return "", status.Errorf(codes.Unauthenticated, "Missing API key")
	}

	apiKey := apiKeys[0]
	if !config.API_KEY_LIST[apiKey] {
		logger.GRPC_F_LOGGER.Printf("unauthorized client: %s", apiKey)
		fmt.Println("Error: see log/gRPC/fail folder for details.")
		return apiKey, status.Errorf(codes.PermissionDenied, "Unauthorized client")
	}

	return apiKey, nil
}
