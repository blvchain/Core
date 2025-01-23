package protos

import (
	"blvchain/core/logger"
	context "context"
)

func (s *AddDataService) AddData(ctx context.Context, req *AddDataRequest) (*AddDataResult, error) {

	// Check input data
	if err := validateAddDataRequest(req); err != nil {
		// Invalid data
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		return &AddDataResult{
			IsSuccess: false,
			Log:       err.Error(),
		}, nil
	} else {
		// Valid data

		return &AddDataResult{
			IsSuccess: true,
			Log:       "Data successfully added.",
		}, nil

	}

}

func (s *ReadDataService) ReadData(ctx context.Context, req *ReadDataRequest) (*ReadDataResult, error) {

	// Check input data
	if err := validateReadDataRequest(req); err != nil {
		// Invalid data
		logger.GRPC_F_LOGGER.Println("Validation failed:", err)
		return &ReadDataResult{
			IsSuccess: false,
			Log:       err.Error(),
			Data:      "",
		}, nil
	} else {
		// Valid data

		return &ReadDataResult{
			IsSuccess: true,
			Log:       "Read successful.",
			Data:      "Sample data...",
		}, nil

	}

}
