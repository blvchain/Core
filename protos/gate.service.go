package protos

import (
	context "context"
	"fmt"
)

func (s *AddDataService) AddData(ctx context.Context, req *AddDataRequest) (*AddDataResult, error) {
	fmt.Println(req.NodeUID)
	return &AddDataResult{
		IsSuccess: true,
		Log:       "Data successfully added.",
	}, nil
}

func (s *ReadDataService) ReadData(ctx context.Context, req *ReadDataRequest) (*ReadDataResult, error) {
	// Add your business logic here
	return &ReadDataResult{
		IsSuccess: "true",
		Log:       "Read successful.",
		Data:      "Sample data...",
	}, nil
}
