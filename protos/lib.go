package protos

import (
	"context"
	"errors"
	"fmt"

	"blvchain/core/config"
	"blvchain/core/logger"
	"blvchain/core/utils"

	validator "github.com/go-playground/validator/v10"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// maxRole is the exclusive upper bound for sender/receiver roles.
const maxRole int64 = 10000001

// validateAddDataRequest checks the shape of an AddData request.
//
// Simple length / numeric constraints are delegated to go-playground/validator;
// cross-field rules (data size, wasm size, timestamp windows) stay here.
func validateAddDataRequest(req *BlockData) error {
	if req == nil {
		return errors.New("request is required")
	}

	if err := validate.Var(req.GetSenderUID(), "required,len=32,hexadecimal"); err != nil {
		return errors.New("senderUID is required and must be a 32 character hex string")
	}

	// SenderRole: must be > 0 and < 10000001
	if err := validate.Var(req.GetSenderRole(), "gt=0,lt=10000001"); err != nil {
		return errors.New("senderRole is required and must be bigger than zero")
	}

	if err := validate.Var(req.GetSenderPubKey(), "required,len=66,hexadecimal"); err != nil {
		return errors.New("senderPubKey is required and must be a 66 character hex string")
	}

	if err := validate.Var(req.GetSignature(), "required,len=128,hexadecimal"); err != nil {
		return errors.New("signature is required and must be a 128 character hex string")
	}

	if err := validate.Var(req.GetReceiverUID(), "required,len=32,hexadecimal"); err != nil {
		return errors.New("receiverUID is required and must be a 32 character hex string")
	}

	if err := validate.Var(req.GetReceiverRole(), "gt=0,lt=10000001"); err != nil {
		return errors.New("receiverRole is required and must be bigger than zero")
	}

	// Data is required and must be smaller than the configured maximum size.
	maxKB := utils.StringToFloat64(config.MAX_DATA_SIZE_KB)
	if req.GetData() == "" {
		return fmt.Errorf("data is required and must be lesser than %sKB", config.MAX_DATA_SIZE_KB)
	}
	if utils.StringSizeInKB(req.GetData()) >= maxKB {
		return fmt.Errorf("data is required and must be lesser than %sKB", config.MAX_DATA_SIZE_KB)
	}

	// A contract upload carries the WASM binary in ContractBase64. The payload
	// limit is min(MAX_DATA_SIZE_KB, MAX_WASM_SIZE_KB).
	if len(req.GetContractBase64()) > 0 {
		if err := validate.Var(req.GetUseContract(), "required,len=66,hexadecimal"); err != nil {
			return errors.New("useContract is required for a contract upload and must be a 66 character hex string")
		}
		if int64(len(req.GetContractBase64())) > config.MAX_WASM_SIZE {
			return fmt.Errorf("wasm file must be lesser than %dKB", config.MAX_WASM_SIZE_KB)
		}
		if checksum := req.GetContractData().GetChecksum(); checksum != "" {
			if err := validate.Var(checksum, "len=64,hexadecimal"); err != nil {
				return errors.New("contract checksum must be a 64 character hex string")
			}
		}
	} else if req.GetUseContract() != "" {
		if err := validate.Var(req.GetUseContract(), "len=66,hexadecimal"); err != nil {
			return errors.New("useContract must be a 66 character hex string")
		}
	}

	// TimeStamp must be a unix timestamp in milliseconds and, for a *new*
	// submission, close to the node's clock. The wide MIN/MAX window is only a
	// sanity bound for stored blocks; client submissions are held to a skew
	// window so a client cannot backdate or post-date a record arbitrarily
	// (VAL-3). This is not a consensus rule and never a sync mechanism.
	now := utils.NowTimeInt64UnixMilli()
	skew := config.TIMESTAMP_SKEW_MS
	if skew <= 0 {
		skew = config.DEFAULT_TIMESTAMP_SKEW_MS
	}
	if req.GetTimeStamp() < now-skew || req.GetTimeStamp() > now+skew {
		return errors.New("timeStamp must be within the accepted clock-skew window")
	}
	if req.GetTimeStamp() < config.MIN_TIMESTAMP_MS || req.GetTimeStamp() > config.MAX_TIMESTAMP_MS {
		return errors.New("timeStamp must be a valid unix format with milliseconds")
	}

	return nil
}

// validateReadDataRequest checks a ReadData request.
//
// limit and skip are always required. At least one *filter* must be supplied;
// limit/skip alone do not count as a filter.
func validateReadDataRequest(req *ReadDataRequest) error {
	if req == nil {
		return errors.New("request is required")
	}

	if err := validate.Var(req.GetLimit(), "gt=0,lt=101"); err != nil {
		return errors.New("limit must be between 1-100")
	}

	if req.GetSkip() < 0 {
		return errors.New("skip must be zero or bigger than zero")
	}

	provided := false

	// UID
	if req.GetUID() != "" {
		provided = true
		if err := validate.Var(req.GetUID(), "len=32,hexadecimal"); err != nil {
			return errors.New("uid must be a 32 character hex string")
		}
	}

	// SenderUID
	if req.GetSenderUID() != "" {
		provided = true
		if err := validate.Var(req.GetSenderUID(), "len=32,hexadecimal"); err != nil {
			return errors.New("senderUID must be a 32 character hex string")
		}
	}

	// SenderRole (optional if zero)
	if req.GetSenderRole() != 0 {
		provided = true
		if req.GetSenderRole() <= 0 || req.GetSenderRole() >= maxRole {
			return errors.New("senderRole must be bigger than zero and within allowed range")
		}
	}

	// SenderPubKey
	if req.GetSenderPubKey() != "" {
		provided = true
		if err := validate.Var(req.GetSenderPubKey(), "len=66,hexadecimal"); err != nil {
			return errors.New("senderPubKey must be a 66 character hex string")
		}
	}

	// ReceiverUID
	if req.GetReceiverUID() != "" {
		provided = true
		if err := validate.Var(req.GetReceiverUID(), "len=32,hexadecimal"); err != nil {
			return errors.New("receiverUID must be a 32 character hex string")
		}
	}

	// ReceiverRole
	if req.GetReceiverRole() != 0 {
		provided = true
		if req.GetReceiverRole() <= 0 || req.GetReceiverRole() >= maxRole {
			return errors.New("receiverRole must be bigger than zero and within allowed range")
		}
	}

	// BlockHash
	if req.GetBlockHash() != "" {
		provided = true
		if err := validate.Var(req.GetBlockHash(), "len=64,hexadecimal"); err != nil {
			return errors.New("blockHash must be a 64 character hex string")
		}
	}

	// PreBlockHash
	if req.GetPreBlockHash() != "" {
		provided = true
		if err := validate.Var(req.GetPreBlockHash(), "len=64,hexadecimal"); err != nil {
			return errors.New("preBlockHash must be a 64 character hex string")
		}
	}

	// NodeUID: node identifiers are longer than 9 characters (they are not
	// restricted to hex, e.g. "node_1_729ea2f23d456b626df2f27183047912").
	if req.GetNodeUID() != "" {
		provided = true
		if len(req.GetNodeUID()) <= 9 {
			return errors.New("nodeUID must be longer than 9 characters")
		}
	}

	// TimeStampFrom / TimeStampTo are unix milliseconds, matching the values
	// stored in blockData.timeStamp.
	if req.GetTimeStampFrom() != 0 {
		provided = true
		if req.GetTimeStampFrom() < config.MIN_TIMESTAMP_MS || req.GetTimeStampFrom() > config.MAX_TIMESTAMP_MS {
			return errors.New("timeStampFrom must be a valid unix timestamp in milliseconds")
		}
	}
	if req.GetTimeStampTo() != 0 {
		provided = true
		if req.GetTimeStampTo() < config.MIN_TIMESTAMP_MS || req.GetTimeStampTo() > config.MAX_TIMESTAMP_MS {
			return errors.New("timeStampTo must be a valid unix timestamp in milliseconds")
		}
	}
	if req.GetTimeStampFrom() != 0 && req.GetTimeStampTo() != 0 && req.GetTimeStampFrom() > req.GetTimeStampTo() {
		return errors.New("timeStampFrom must not be greater than timeStampTo")
	}

	// UseContract
	if req.GetUseContract() != "" {
		provided = true
		if err := validate.Var(req.GetUseContract(), "len=66,hexadecimal"); err != nil {
			return errors.New("useContract must be a 66 character hex string")
		}
	}

	if !provided {
		return errors.New("no filters provided in the request / provided filters are not correct")
	}

	return nil
}

// validateAuth extracts and checks the API key from the gRPC metadata.
//
// Clients may send the key as "authorization", "x-api-key" or "auth". gRPC
// lower-cases metadata keys, so all lookups are lower-case.
func validateAuth(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.GRPC_F_LOGGER.Println("Missing metadata")
		return "", status.Error(codes.Unauthenticated, "Missing metadata")
	}

	var apiKey string
	for _, key := range []string{"authorization", "x-api-key", "auth"} {
		if values := md.Get(key); len(values) > 0 && values[0] != "" {
			apiKey = values[0]
			break
		}
	}

	if apiKey == "" {
		logger.GRPC_F_LOGGER.Println("Missing API key")
		return "", status.Error(codes.Unauthenticated, "Missing API key")
	}

	if !config.API_KEY_LIST[apiKey] {
		logger.GRPC_F_LOGGER.Printf("Unauthorized client: %s", apiKey)
		return apiKey, status.Error(codes.PermissionDenied, "Unauthorized client")
	}

	return apiKey, nil
}
