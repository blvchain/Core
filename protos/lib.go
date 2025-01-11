package protos

import "errors"

func validateAddDataRequest(req *AddDataRequest) error {
	if req.SenderUID == "" {
		return errors.New("senderUID is required")
	}
	if req.SenderRole < 0 {
		return errors.New("senderRole must be bigger than zero")
	}
	if req.SenderPubKey == "" {
		return errors.New("senderPubKey is required")
	}
	if req.SenderSignature == "" {
		return errors.New("senderSignature is required")
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
