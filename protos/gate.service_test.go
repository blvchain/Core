package protos

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/utils"

	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func allBlocks(t *testing.T) []db.Block {
	t.Helper()

	blocks, err := db.FindAllBlocks(bson.M{})
	if err != nil {
		t.Fatalf("FindAllBlocks: %v", err)
	}
	return blocks
}

func TestAddDataStoresBlock(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("1", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	req := signedRequest(t, sender, receiver, "hello world", utils.NowTimeInt64UnixMilli())

	service := &AddDataService{}
	result, err := service.AddData(authContext(testAPIKey), req)
	if err != nil {
		t.Fatalf("AddData: %v", err)
	}
	if !result.IsSuccess {
		t.Fatalf("AddData failed: %s", result.Log)
	}
	if result.BlockHash == "" {
		t.Fatal("AddData returned an empty block hash")
	}

	blocks := allBlocks(t)
	if len(blocks) != 1 {
		t.Fatalf("stored %d blocks, want 1", len(blocks))
	}

	stored := blocks[0]
	if stored.ID != result.BlockHash {
		t.Fatalf("stored ID %q, response %q", stored.ID, result.BlockHash)
	}
	if stored.BlockMeta.PreBlockHash != config.FIRST_BLOCK_HASH {
		t.Fatalf("preBlockHash = %q, want the genesis hash %q", stored.BlockMeta.PreBlockHash, config.FIRST_BLOCK_HASH)
	}
	if stored.BlockMeta.NodeUID != config.SELF_UID {
		t.Fatalf("nodeUID = %q, want %q", stored.BlockMeta.NodeUID, config.SELF_UID)
	}
	if stored.BlockData.Data != "hello world" {
		t.Fatalf("data = %q", stored.BlockData.Data)
	}
	if err := db.BlockValidator(stored); err != nil {
		t.Fatalf("the stored block does not validate: %v", err)
	}
}

func TestAddDataLinksToThePreviousBlock(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("2", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	service := &AddDataService{}

	first, err := service.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, "first", utils.NowTimeInt64UnixMilli()))
	if err != nil || !first.IsSuccess {
		t.Fatalf("first AddData = %+v, %v", first, err)
	}

	second, err := service.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, "second", utils.NowTimeInt64UnixMilli()))
	if err != nil || !second.IsSuccess {
		t.Fatalf("second AddData = %+v, %v", second, err)
	}

	blocks := allBlocks(t)
	if len(blocks) != 2 {
		t.Fatalf("stored %d blocks, want 2", len(blocks))
	}

	var firstBlock, secondBlock db.Block
	for _, block := range blocks {
		if block.ID == first.BlockHash {
			firstBlock = block
		}
		if block.ID == second.BlockHash {
			secondBlock = block
		}
	}

	if secondBlock.BlockMeta.PreBlockHash != firstBlock.ID {
		t.Fatalf("second block links to %q, want %q", secondBlock.BlockMeta.PreBlockHash, firstBlock.ID)
	}
}

func TestAddDataRejectsInvalidSignature(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("3", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	req := signedRequest(t, sender, receiver, "hello", utils.NowTimeInt64UnixMilli())
	req.Signature = strings.Repeat("0", 128)

	service := &AddDataService{}
	result, err := service.AddData(authContext(testAPIKey), req)
	if err != nil {
		t.Fatalf("AddData returned a gRPC error: %v", err)
	}
	if result.IsSuccess {
		t.Fatal("AddData accepted an invalid signature")
	}
	if len(allBlocks(t)) != 0 {
		t.Fatal("a block was stored despite the invalid signature")
	}
}

func TestAddDataRejectsTamperedData(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("4", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	req := signedRequest(t, sender, receiver, "original", utils.NowTimeInt64UnixMilli())
	req.Data = "tampered"

	service := &AddDataService{}
	result, err := service.AddData(authContext(testAPIKey), req)
	if err != nil {
		t.Fatalf("AddData returned a gRPC error: %v", err)
	}
	if result.IsSuccess {
		t.Fatal("AddData accepted a block whose data no longer matches the signature")
	}
}

func TestAddDataRejectsInvalidRequest(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	req := signedRequest(t, sender, receiver, "hello", utils.NowTimeInt64UnixMilli())
	req.Data = ""

	service := &AddDataService{}
	_, err := service.AddData(authContext(testAPIKey), req)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("AddData error = %v, want InvalidArgument", err)
	}
}

func TestAddDataRequiresAuthentication(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	req := signedRequest(t, sender, receiver, "hello", utils.NowTimeInt64UnixMilli())

	service := &AddDataService{}

	if _, err := service.AddData(context.Background(), req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing metadata = %v, want Unauthenticated", err)
	}

	unauthorized := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "unknown"))
	if _, err := service.AddData(unauthorized, req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unknown key = %v, want PermissionDenied", err)
	}
}

func TestAddDataIsIdempotent(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("5", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	req := signedRequest(t, sender, receiver, "hello", utils.NowTimeInt64UnixMilli())

	service := &AddDataService{}

	first, err := service.AddData(authContext(testAPIKey), req)
	if err != nil || !first.IsSuccess {
		t.Fatalf("first AddData = %+v, %v", first, err)
	}

	second, err := service.AddData(authContext(testAPIKey), req)
	if err != nil || !second.IsSuccess {
		t.Fatalf("retry AddData = %+v, %v", second, err)
	}
	if second.BlockHash != first.BlockHash {
		t.Fatalf("retry returned a different hash: %q vs %q", second.BlockHash, first.BlockHash)
	}
	if got := len(allBlocks(t)); got != 1 {
		t.Fatalf("retry stored %d blocks, want 1", got)
	}
}

func TestAddDataStoresContract(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("6", 64)

	dir := t.TempDir()
	originalDir := config.SMART_CONTRACT_DIR
	config.SMART_CONTRACT_DIR = dir
	t.Cleanup(func() { config.SMART_CONTRACT_DIR = originalDir })

	sender := newIdentity(t)
	receiver := newIdentity(t)

	// Minimal WASM header.
	wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	checksum := utils.SHA256Hex(wasm)

	blockData := db.BlockData{
		SenderUID:    sender.uid,
		SenderRole:   1,
		SenderPubKey: sender.pubHex,
		ReceiverUID:  receiver.uid,
		ReceiverRole: 1,
		Data:         "contract-payload",
		UseContract:  strings.Repeat("a", 66),
		ContractData: db.Contract{Name: "counter", Checksum: checksum},
		TimeStamp:    utils.NowTimeInt64UnixMilli(),
	}

	signature, err := utils.Sign(sender.privHex, db.MessageMaker(blockData))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	req := &BlockData{
		SenderUID:      sender.uid,
		SenderRole:     1,
		SenderPubKey:   sender.pubHex,
		Signature:      signature,
		ReceiverUID:    receiver.uid,
		ReceiverRole:   1,
		Data:           "contract-payload",
		UseContract:    strings.Repeat("a", 66),
		ContractBase64: []byte(base64.StdEncoding.EncodeToString(wasm)),
		ContractData:   &Contract{Name: "counter", Checksum: checksum},
		TimeStamp:      blockData.TimeStamp,
	}

	service := &AddDataService{}
	result, err := service.AddData(authContext(testAPIKey), req)
	if err != nil {
		t.Fatalf("AddData: %v", err)
	}
	if !result.IsSuccess {
		t.Fatalf("AddData failed: %s", result.Log)
	}

	storedPath := filepath.Join(dir, checksum+".wasm")
	content, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("contract file was not stored at %s: %v", storedPath, err)
	}
	if string(content) != string(wasm) {
		t.Fatal("the stored contract differs from the uploaded bytes")
	}

	blocks := allBlocks(t)
	if len(blocks) != 1 {
		t.Fatalf("stored %d blocks, want 1", len(blocks))
	}
	if blocks[0].BlockData.ContractData.Checksum != checksum {
		t.Fatalf("contract checksum = %q, want %q", blocks[0].BlockData.ContractData.Checksum, checksum)
	}
}

func TestAddDataRejectsContractChecksumMismatch(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("7", 64)

	dir := t.TempDir()
	originalDir := config.SMART_CONTRACT_DIR
	config.SMART_CONTRACT_DIR = dir
	t.Cleanup(func() { config.SMART_CONTRACT_DIR = originalDir })

	sender := newIdentity(t)
	receiver := newIdentity(t)

	wasm := []byte{0x00, 0x61, 0x73, 0x6d}
	wrongChecksum := utils.SHA256Hex([]byte("something else"))

	blockData := db.BlockData{
		SenderUID:    sender.uid,
		SenderRole:   1,
		SenderPubKey: sender.pubHex,
		ReceiverUID:  receiver.uid,
		ReceiverRole: 1,
		Data:         "payload",
		UseContract:  strings.Repeat("a", 66),
		ContractData: db.Contract{Checksum: wrongChecksum},
		TimeStamp:    utils.NowTimeInt64UnixMilli(),
	}

	signature, err := utils.Sign(sender.privHex, db.MessageMaker(blockData))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	req := &BlockData{
		SenderUID:      sender.uid,
		SenderRole:     1,
		SenderPubKey:   sender.pubHex,
		Signature:      signature,
		ReceiverUID:    receiver.uid,
		ReceiverRole:   1,
		Data:           "payload",
		UseContract:    strings.Repeat("a", 66),
		ContractBase64: []byte(base64.StdEncoding.EncodeToString(wasm)),
		ContractData:   &Contract{Checksum: wrongChecksum},
		TimeStamp:      blockData.TimeStamp,
	}

	service := &AddDataService{}
	result, err := service.AddData(authContext(testAPIKey), req)
	if err != nil {
		t.Fatalf("AddData: %v", err)
	}
	if result.IsSuccess {
		t.Fatal("AddData accepted a contract whose checksum does not match its bytes")
	}
	if len(allBlocks(t)) != 0 {
		t.Fatal("a block was stored despite the checksum mismatch")
	}
}

func TestAddDataRejectsTimestampOutsideSkewWindow(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("b", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	service := &AddDataService{}

	now := utils.NowTimeInt64UnixMilli()
	skew := config.TIMESTAMP_SKEW_MS
	if skew <= 0 {
		skew = config.DEFAULT_TIMESTAMP_SKEW_MS
	}

	cases := map[string]int64{
		"backdated": now - skew - 60_000,
		"postdated": now + skew + 60_000,
	}
	for name, ts := range cases {
		t.Run(name, func(t *testing.T) {
			req := signedRequest(t, sender, receiver, "hello", ts)
			result, err := service.AddData(authContext(testAPIKey), req)
			// Validation failures surface as an InvalidArgument gRPC error; a
			// successful call must never accept the timestamp.
			if err == nil && result.IsSuccess {
				t.Fatalf("AddData accepted a %s timestamp", name)
			}
		})
	}

	if blocks := allBlocks(t); len(blocks) != 0 {
		t.Fatalf("%d blocks were stored for rejected timestamps", len(blocks))
	}
}

func TestReadDataReturnsStoredBlocks(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("8", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	add := &AddDataService{}
	if _, err := add.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, "one", utils.NowTimeInt64UnixMilli())); err != nil {
		t.Fatalf("AddData: %v", err)
	}
	if _, err := add.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, "two", utils.NowTimeInt64UnixMilli())); err != nil {
		t.Fatalf("AddData: %v", err)
	}

	read := &ReadDataService{}
	result, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 10, Skip: 0, SenderUID: sender.uid})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if !result.IsSuccess {
		t.Fatalf("ReadData failed: %s", result.Log)
	}
	if len(result.Data) != 2 {
		t.Fatalf("ReadData returned %d blocks, want 2", len(result.Data))
	}
	for _, block := range result.Data {
		if block.BlockData.SenderUID != sender.uid {
			t.Fatalf("unexpected sender %q", block.BlockData.SenderUID)
		}
		if block.BlockData.ContractData == nil {
			t.Fatal("ReadData did not populate ContractData")
		}
	}
}

func TestReadDataFilters(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("9", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	other := newIdentity(t)

	add := &AddDataService{}
	for _, data := range []string{"one", "two"} {
		if _, err := add.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, data, utils.NowTimeInt64UnixMilli())); err != nil {
			t.Fatalf("AddData: %v", err)
		}
	}
	if _, err := add.AddData(authContext(testAPIKey), signedRequest(t, other, receiver, "three", utils.NowTimeInt64UnixMilli())); err != nil {
		t.Fatalf("AddData: %v", err)
	}

	read := &ReadDataService{}

	bySender, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 10, SenderUID: sender.uid})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if len(bySender.Data) != 2 {
		t.Fatalf("sender filter returned %d blocks, want 2", len(bySender.Data))
	}

	byReceiver, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 10, ReceiverUID: receiver.uid})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if len(byReceiver.Data) != 3 {
		t.Fatalf("receiver filter returned %d blocks, want 3", len(byReceiver.Data))
	}

	byUID, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 10, UID: sender.uid})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if len(byUID.Data) != 2 {
		t.Fatalf("UID filter returned %d blocks, want 2", len(byUID.Data))
	}
}

func TestReadDataTimestampFilterUsesMilliseconds(t *testing.T) {
	resetStore(t)
	config.FIRST_BLOCK_HASH = strings.Repeat("a", 64)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	// Client submissions must be near the node's clock (VAL-3), so use recent
	// timestamps and filter on a narrow window around them.
	now := utils.NowTimeInt64UnixMilli()
	first := now - 10000
	second := now - 5000

	add := &AddDataService{}
	if _, err := add.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, "one", first)); err != nil {
		t.Fatalf("AddData: %v", err)
	}
	if _, err := add.AddData(authContext(testAPIKey), signedRequest(t, sender, receiver, "two", second)); err != nil {
		t.Fatalf("AddData: %v", err)
	}

	read := &ReadDataService{}

	// A range expressed in *seconds* must be rejected by validation, because
	// the stored timestamps are milliseconds.
	if _, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{
		Limit:         10,
		SenderUID:     sender.uid,
		TimeStampFrom: 1720108080,
		TimeStampTo:   1720108090,
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("seconds-based range = %v, want InvalidArgument", err)
	}

	millisRange, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{
		Limit:         10,
		SenderUID:     sender.uid,
		TimeStampFrom: first - 1000,
		TimeStampTo:   second + 1000,
	})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if len(millisRange.Data) != 2 {
		t.Fatalf("milliseconds range returned %d blocks, want 2", len(millisRange.Data))
	}

	// A narrow window around the first block returns exactly that block.
	narrow, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{
		Limit:         10,
		SenderUID:     sender.uid,
		TimeStampFrom: first - 1000,
		TimeStampTo:   first + 1000,
	})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if len(narrow.Data) != 1 {
		t.Fatalf("narrow range returned %d blocks, want 1", len(narrow.Data))
	}
}

func TestReadDataNotFound(t *testing.T) {
	resetStore(t)

	read := &ReadDataService{}
	result, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 10, SenderUID: strings.Repeat("f", 32)})
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	if !result.IsSuccess {
		t.Fatalf("ReadData should succeed with an empty result, log: %s", result.Log)
	}
	if len(result.Data) != 0 {
		t.Fatalf("ReadData returned %d blocks, want 0", len(result.Data))
	}
}

func TestReadDataValidatesAndAuthenticates(t *testing.T) {
	resetStore(t)
	read := &ReadDataService{}

	if _, err := read.ReadData(context.Background(), &ReadDataRequest{Limit: 10, SenderUID: strings.Repeat("a", 32)}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing metadata = %v, want Unauthenticated", err)
	}

	if _, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 0, SenderUID: strings.Repeat("a", 32)}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("limit 0 = %v, want InvalidArgument", err)
	}

	if _, err := read.ReadData(authContext(testAPIKey), &ReadDataRequest{Limit: 10}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("no filters = %v, want InvalidArgument", err)
	}
}
