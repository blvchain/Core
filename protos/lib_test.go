package protos

import (
	"context"
	"strings"
	"testing"

	"blvchain/core/config"
	"blvchain/core/utils"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestValidateAddDataRequest(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)

	valid := func() *BlockData {
		return signedRequest(t, sender, receiver, "hello", utils.NowTimeInt64UnixMilli())
	}

	cases := []struct {
		name    string
		mutate  func(*BlockData)
		wantErr bool
	}{
		{"valid", func(*BlockData) {}, false},
		{"nil request", nil, true},
		{"empty sender uid", func(r *BlockData) { r.SenderUID = "" }, true},
		{"short sender uid", func(r *BlockData) { r.SenderUID = "abc" }, true},
		{"non hex sender uid", func(r *BlockData) { r.SenderUID = strings.Repeat("z", 32) }, true},
		{"zero sender role", func(r *BlockData) { r.SenderRole = 0 }, true},
		{"negative sender role", func(r *BlockData) { r.SenderRole = -1 }, true},
		{"huge sender role", func(r *BlockData) { r.SenderRole = 10000001 }, true},
		{"short public key", func(r *BlockData) { r.SenderPubKey = "00" }, true},
		{"short signature", func(r *BlockData) { r.Signature = "00" }, true},
		{"empty receiver uid", func(r *BlockData) { r.ReceiverUID = "" }, true},
		{"zero receiver role", func(r *BlockData) { r.ReceiverRole = 0 }, true},
		{"empty data", func(r *BlockData) { r.Data = "" }, true},
		{"oversized data", func(r *BlockData) { r.Data = strings.Repeat("x", 3*1024*1024) }, true},
		{"zero timestamp", func(r *BlockData) { r.TimeStamp = 0 }, true},
		{"timestamp too old", func(r *BlockData) { r.TimeStamp = 1 }, true},
		{"timestamp in seconds", func(r *BlockData) { r.TimeStamp = 1720108080 }, true},
		{"short use contract", func(r *BlockData) { r.UseContract = "abc" }, true},
		{"non hex use contract", func(r *BlockData) { r.UseContract = strings.Repeat("z", 66) }, true},
		{"contract without checksum", func(r *BlockData) {
			r.UseContract = strings.Repeat("a", 66)
			r.ContractBase64 = []byte("not-really-base64-wasm")
		}, false},
		{"contract with bad checksum", func(r *BlockData) {
			r.UseContract = strings.Repeat("a", 66)
			r.ContractBase64 = []byte("AAAA")
			r.ContractData = &Contract{Checksum: "xyz"}
		}, true},
		{"contract payload too large", func(r *BlockData) {
			r.UseContract = strings.Repeat("a", 66)
			r.ContractBase64 = make([]byte, config.MAX_WASM_SIZE+1)
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *BlockData
			if tc.mutate != nil {
				req = valid()
				tc.mutate(req)
			}

			err := validateAddDataRequest(req)
			if tc.wantErr && err == nil {
				t.Fatalf("validateAddDataRequest accepted an invalid request")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateAddDataRequest rejected a valid request: %v", err)
			}
		})
	}
}

func TestValidateReadDataRequest(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ReadDataRequest)
		wantErr bool
	}{
		{"valid sender filter", func(r *ReadDataRequest) { r.SenderUID = strings.Repeat("a", 32) }, false},
		{"valid hash filter", func(r *ReadDataRequest) { r.BlockHash = strings.Repeat("b", 64) }, false},
		{"valid timestamp filter", func(r *ReadDataRequest) {
			r.TimeStampFrom = config.MIN_TIMESTAMP_MS
			r.TimeStampTo = config.MAX_TIMESTAMP_MS
		}, false},
		{"limit zero", func(r *ReadDataRequest) { r.Limit = 0 }, true},
		{"limit too large", func(r *ReadDataRequest) { r.Limit = 101 }, true},
		{"negative skip", func(r *ReadDataRequest) { r.Skip = -1 }, true},
		{"no filters", func(r *ReadDataRequest) {}, true},
		{"short uid", func(r *ReadDataRequest) { r.UID = "abc" }, true},
		{"non hex uid", func(r *ReadDataRequest) { r.UID = strings.Repeat("z", 32) }, true},
		{"short sender uid", func(r *ReadDataRequest) { r.SenderUID = "abc" }, true},
		{"bad sender role", func(r *ReadDataRequest) { r.SenderRole = 10000001 }, true},
		{"short sender pub key", func(r *ReadDataRequest) { r.SenderPubKey = "00" }, true},
		{"short receiver uid", func(r *ReadDataRequest) { r.ReceiverUID = "abc" }, true},
		{"short block hash", func(r *ReadDataRequest) { r.BlockHash = "abc" }, true},
		{"non hex block hash", func(r *ReadDataRequest) { r.BlockHash = strings.Repeat("z", 64) }, true},
		{"short pre hash", func(r *ReadDataRequest) { r.PreBlockHash = "abc" }, true},
		{"short node uid", func(r *ReadDataRequest) { r.NodeUID = "abc" }, true},
		{"timestamp in seconds is rejected", func(r *ReadDataRequest) { r.TimeStampFrom = 1720108080 }, true},
		{"from after to", func(r *ReadDataRequest) {
			r.TimeStampFrom = config.MAX_TIMESTAMP_MS
			r.TimeStampTo = config.MIN_TIMESTAMP_MS
		}, true},
		{"short use contract", func(r *ReadDataRequest) { r.UseContract = "abc" }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &ReadDataRequest{Limit: 10, Skip: 0}
			tc.mutate(req)

			err := validateReadDataRequest(req)
			if tc.wantErr && err == nil {
				t.Fatal("validateReadDataRequest accepted an invalid request")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateReadDataRequest rejected a valid request: %v", err)
			}
		})
	}
}

func TestValidateReadDataRequestRejectsNil(t *testing.T) {
	if err := validateReadDataRequest(nil); err == nil {
		t.Fatal("nil request should be rejected")
	}
}

func TestValidateAuth(t *testing.T) {
	// Each supported metadata key must work.
	for _, key := range []string{"authorization", "x-api-key", "auth"} {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(key, testAPIKey))
		got, err := validateAuth(ctx)
		if err != nil || got != testAPIKey {
			t.Errorf("validateAuth with %q = %q, %v", key, got, err)
		}
	}

	// No metadata at all.
	if _, err := validateAuth(context.Background()); status.Code(err) != codes.Unauthenticated {
		t.Errorf("missing metadata = %v, want Unauthenticated", err)
	}

	// Metadata without an API key.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("other", "value"))
	if _, err := validateAuth(ctx); status.Code(err) != codes.Unauthenticated {
		t.Errorf("missing key = %v, want Unauthenticated", err)
	}

	// Known but disabled key.
	disabled := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "disabled"))
	if _, err := validateAuth(disabled); status.Code(err) != codes.PermissionDenied {
		t.Errorf("disabled key = %v, want PermissionDenied", err)
	}

	// Unknown key.
	unknown := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "nope"))
	if _, err := validateAuth(unknown); status.Code(err) != codes.PermissionDenied {
		t.Errorf("unknown key = %v, want PermissionDenied", err)
	}
}
