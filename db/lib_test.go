package db

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"

	"blvchain/core/config"
	"blvchain/core/utils"
)

// identity is a key pair plus the UID the chain derives from it.
type identity struct {
	privHex string
	pubHex  string
	uid     string
}

func newIdentity(t *testing.T) identity {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pubHex := utils.PubKeyHex(&priv.PublicKey)
	return identity{
		privHex: utils.PrivateKeyHex(priv),
		pubHex:  pubHex,
		uid:     utils.Make_UID(pubHex),
	}
}

// makeBlock builds a correctly signed and correctly hashed block.
func makeBlock(t *testing.T, sender identity, receiverUID, data string, preHash string) Block {
	t.Helper()

	blockData := BlockData{
		SenderUID:    sender.uid,
		SenderRole:   1,
		SenderPubKey: sender.pubHex,
		ReceiverUID:  receiverUID,
		ReceiverRole: 1,
		Data:         data,
		TimeStamp:    utils.NowTimeInt64UnixMilli(),
	}

	signature, err := utils.Sign(sender.privHex, MessageMaker(blockData))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	blockData.Signature = signature

	block := Block{
		Boycott: false,
		BlockMeta: BlockMeta{
			PreBlockHash: preHash,
			TimeStamp:    blockData.TimeStamp,
		},
		BlockData: blockData,
	}
	BlockHashMaker(&block, config.SELF_UID)
	return block
}

func TestBlockHashMakerIsDeterministic(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)

	block := makeBlock(t, sender, receiver.uid, "payload", strings.Repeat("a", 64))
	first := block.ID

	// Re-hashing the same block must produce the same ID.
	BlockHashMaker(&block, block.BlockMeta.NodeUID)
	if block.ID != first {
		t.Fatalf("re-hashing changed the ID: %q vs %q", first, block.ID)
	}
	if len(first) != 64 {
		t.Fatalf("block ID must be 64 characters, got %d", len(first))
	}
}

func TestBlockHashMakerCoversEveryField(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	pre := strings.Repeat("a", 64)

	base := makeBlock(t, sender, receiver.uid, "payload", pre)

	variants := map[string]func(b *Block){
		"preBlockHash": func(b *Block) { b.BlockMeta.PreBlockHash = strings.Repeat("b", 64) },
		"nodeUID":      func(b *Block) { b.BlockMeta.NodeUID = "node_2_test" },
		"metaTime":     func(b *Block) { b.BlockMeta.TimeStamp++ },
		"data":         func(b *Block) { b.BlockData.Data = "other" },
		"senderRole":   func(b *Block) { b.BlockData.SenderRole = 2 },
		"receiverUID":  func(b *Block) { b.BlockData.ReceiverUID = strings.Repeat("c", 32) },
		"dataTime":     func(b *Block) { b.BlockData.TimeStamp++ },
		"useContract":  func(b *Block) { b.BlockData.UseContract = strings.Repeat("d", 66) },
		"contractName": func(b *Block) { b.BlockData.ContractData.Name = "counter" },
	}

	for name, mutate := range variants {
		tampered := base
		mutate(&tampered)
		BlockHashMaker(&tampered, tampered.BlockMeta.NodeUID)

		if tampered.ID == base.ID {
			t.Errorf("changing %s did not change the block ID", name)
		}
	}
}

func TestBlockValidatorAcceptsValidBlock(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "hello chain", strings.Repeat("0", 64))

	if err := BlockValidator(block); err != nil {
		t.Fatalf("BlockValidator rejected a valid block: %v", err)
	}
}

func TestBlockValidatorRejectsTamperedData(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "hello chain", strings.Repeat("0", 64))

	block.BlockData.Data = "tampered"
	if err := BlockValidator(block); err == nil {
		t.Fatal("BlockValidator accepted a block with modified data")
	}
}

func TestBlockValidatorRejectsBadSignature(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "hello chain", strings.Repeat("0", 64))

	block.BlockData.Signature = strings.Repeat("0", 128)
	if err := BlockValidator(block); err == nil {
		t.Fatal("BlockValidator accepted a zero signature")
	}
}

func TestBlockValidatorRejectsWrongNodeUID(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "hello chain", strings.Repeat("0", 64))

	block.BlockMeta.NodeUID = "node_2_test"
	if err := BlockValidator(block); err == nil {
		t.Fatal("BlockValidator accepted a block whose node UID does not match its hash")
	}
}

func TestBlockValidatorRejectsMalformedBlock(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "hello chain", strings.Repeat("0", 64))

	cases := map[string]func(b *Block){
		"short id":         func(b *Block) { b.ID = "abc" },
		"bad pre hash":     func(b *Block) { b.BlockMeta.PreBlockHash = "xyz" },
		"bad sender uid":   func(b *Block) { b.BlockData.SenderUID = "not-hex" },
		"bad sender role":  func(b *Block) { b.BlockData.SenderRole = 0 },
		"bad public key":   func(b *Block) { b.BlockData.SenderPubKey = strings.Repeat("0", 66) },
		"bad timestamp":    func(b *Block) { b.BlockData.TimeStamp = 1 },
		"bad use contract": func(b *Block) { b.BlockData.UseContract = "short" },
		"bad checksum":     func(b *Block) { b.BlockData.ContractData.Checksum = "abc" },
	}

	for name, mutate := range cases {
		tampered := block
		mutate(&tampered)
		if err := BlockValidator(tampered); err == nil {
			t.Errorf("%s: BlockValidator accepted a malformed block", name)
		}
	}
}

func TestBlockStructValidatorAcceptsGenesisShape(t *testing.T) {
	genesis := MakeGenesisBlock()
	if err := BlockStructValidator(genesis); err != nil {
		t.Fatalf("genesis block should pass structural validation: %v", err)
	}
}

func TestBlockStructValidatorRejectsBadData(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver.uid, "payload", strings.Repeat("0", 64))

	if err := BlockStructValidator(block); err != nil {
		t.Fatalf("valid block failed structural validation: %v", err)
	}

	empty := block
	empty.BlockData.Data = ""
	if err := BlockStructValidator(empty); err == nil {
		t.Error("a block without data should fail structural validation")
	}
}

func TestAreBlocksIdentical(t *testing.T) {
	if !AreBlocksIdentical(nil) {
		t.Error("an empty slice should be reported as identical")
	}

	block := Block{ID: "a"}
	if !AreBlocksIdentical([]Block{block, block}) {
		t.Error("identical blocks were reported as different")
	}
	if AreBlocksIdentical([]Block{block, {ID: "b"}}) {
		t.Error("different blocks were reported as identical")
	}
}

func TestGenesisCheckCreatesGenesisOnce(t *testing.T) {
	SetCollection(newTestCollection())
	t.Cleanup(ResetCollection)
	config.FIRST_BLOCK_HASH = ""

	ok, err := Genesis_check()
	if err != nil || !ok {
		t.Fatalf("Genesis_check() = %v, %v", ok, err)
	}
	if config.FIRST_BLOCK_HASH == "" || len(config.FIRST_BLOCK_HASH) != 64 {
		t.Fatalf("FIRST_BLOCK_HASH = %q", config.FIRST_BLOCK_HASH)
	}

	if got := collectionCount(t); got != 1 {
		t.Fatalf("genesis check inserted %d blocks, want 1", got)
	}

	// A second call must not insert another block.
	firstHash := config.FIRST_BLOCK_HASH
	if ok, err := Genesis_check(); err != nil || !ok {
		t.Fatalf("second Genesis_check() = %v, %v", ok, err)
	}
	if got := collectionCount(t); got != 1 {
		t.Fatalf("second genesis check inserted another block, total %d", got)
	}
	if config.FIRST_BLOCK_HASH != firstHash {
		t.Fatalf("FIRST_BLOCK_HASH changed from %q to %q", firstHash, config.FIRST_BLOCK_HASH)
	}
}

func TestGenesisCheckPropagatesErrors(t *testing.T) {
	SetCollection(newFailingCollection())
	t.Cleanup(ResetCollection)

	if _, err := Genesis_check(); err == nil {
		t.Fatal("Genesis_check should report a storage failure")
	}
}

func TestGenesisBlockIsDeterministic(t *testing.T) {
	first := MakeGenesisBlock()
	second := MakeGenesisBlock()
	if first.ID != second.ID {
		t.Fatalf("genesis is not deterministic: %q vs %q", first.ID, second.ID)
	}
	if first.BlockData.Data != config.GENESIS_DATA {
		t.Fatalf("genesis data = %q", first.BlockData.Data)
	}
}

func TestMessageMakerIncludesContractFields(t *testing.T) {
	base := BlockData{
		SenderUID:    strings.Repeat("a", 32),
		SenderRole:   1,
		SenderPubKey: strings.Repeat("b", 66),
		ReceiverUID:  strings.Repeat("c", 32),
		ReceiverRole: 1,
		Data:         "payload",
		TimeStamp:    1720108080000,
	}

	withContract := base
	withContract.UseContract = strings.Repeat("d", 66)
	if MessageMaker(base) == MessageMaker(withContract) {
		t.Fatal("MessageMaker ignores UseContract")
	}

	withMeta := base
	withMeta.ContractData.Name = "counter"
	if MessageMaker(base) == MessageMaker(withMeta) {
		t.Fatal("MessageMaker ignores contract metadata")
	}
}

func TestMessageMakerPinsTheWireFormat(t *testing.T) {
	// The signed message is a plain concatenation of the fields. This test pins
	// that format so any future change is deliberate (and breaking, because
	// peers would reject the new signatures).
	data := BlockData{
		SenderUID:    strings.Repeat("a", 32),
		SenderRole:   1,
		SenderPubKey: strings.Repeat("b", 66),
		ReceiverUID:  strings.Repeat("c", 32),
		ReceiverRole: 2,
		Data:         "payload",
		TimeStamp:    1720108080000,
	}

	want := strings.Repeat("a", 32) + "1" + strings.Repeat("b", 66) +
		strings.Repeat("c", 32) + "2" + "payload" + "1720108080000"

	if got := MessageMaker(data); got != want {
		t.Fatalf("MessageMaker = %q, want %q", got, want)
	}
}
