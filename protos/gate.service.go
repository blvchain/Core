package protos

import context "context"

func (s *AddDataService) AddData(ctx context.Context, req *AddDataRequest) (*AddDataResult, error) {
	// Add your business logic here
	return &AddDataResult{
		IsSuccess: true,
		Log:       "Data successfully added.",
	}, nil
}

func (s *ReadDataService) ReadData(ctx context.Context, req *ReqadDataRequest) (*ReqadDataResult, error) {
	// Add your business logic here
	return &ReqadDataResult{
		IsSuccess: "true",
		Log:       "Read successful.",
		Data:      "Sample data...",
	}, nil
}
