package db

import (
	"errors"
	"fmt"
	"reflect"

	"blvchain/core/config"
	"blvchain/core/utils"

	"go.mongodb.org/mongo-driver/bson"
)

// MakeGenesisBlock builds the deterministic first block of the chain.
func MakeGenesisBlock() Block {
	block := Block{
		Boycott: false,
		BlockMeta: BlockMeta{
			PreBlockHash: config.GENESIS_BLOCK_PREHASH,
			TimeStamp:    config.GENESIS_TIMESTAMP,
		},
		BlockData: BlockData{
			SenderUID:    config.GENESIS_SENDER_UID,
			SenderRole:   config.GENESIS_SENDER_ROLE,
			SenderPubKey: config.GENESIS_PUBKEY,
			Signature:    config.GENESIS_SIGNATURE,
			ReceiverUID:  config.GENESIS_RECEIVER_UID,
			ReceiverRole: config.GENESIS_RECEIVER_ROLE,
			Data:         config.GENESIS_DATA,
			TimeStamp:    config.GENESIS_TIMESTAMP,
		},
	}

	BlockHashMaker(&block, config.GENESIS_NODE_UID)
	return block
}

// Genesis_check makes sure the chain has its first block and records its hash in
// config.FIRST_BLOCK_HASH. It is safe to call on every start-up.
func Genesis_check() (bool, error) {
	existing, err := FindAllBlocks(bson.M{"blockMeta.preBlockHash": config.GENESIS_BLOCK_PREHASH})
	if err != nil {
		return false, fmt.Errorf("genesis lookup: %w", err)
	}

	genesis := MakeGenesisBlock()

	if len(existing) == 0 {
		ok, insertErr := InsertOneBlock(genesis)
		if !ok {
			if insertErr != nil {
				return false, fmt.Errorf("insert genesis: %w", insertErr)
			}
			return false, errors.New("insert genesis: unknown error")
		}
	} else {
		// Keep the stored copy: it is authoritative once the chain exists.
		genesis = existing[0]
	}

	config.FIRST_BLOCK_HASH = genesis.ID
	return true, nil
}

// contractRoot flattens the contract metadata into a stable string so it can be
// fed into the hash and the signed message.
func contractRoot(c Contract) string {
	return c.Name +
		c.Version +
		c.Language +
		c.Compiler +
		c.Description +
		c.Checksum +
		c.Author +
		c.License
}

// BlockHashMaker computes the block ID from every field that defines the block.
//
// UseContract and ContractData are part of the digest: without them two blocks
// that differ only in their contract metadata would share an ID.
func BlockHashMaker(b *Block, nodeUID string) {
	b.BlockMeta.NodeUID = nodeUID

	blockMetaRoot := b.BlockMeta.PreBlockHash +
		b.BlockMeta.NodeUID +
		utils.Int64ToStr(b.BlockMeta.TimeStamp)

	blockDataRoot := b.BlockData.SenderUID +
		utils.Int64ToStr(b.BlockData.SenderRole) +
		b.BlockData.SenderPubKey +
		b.BlockData.Signature +
		b.BlockData.ReceiverUID +
		utils.Int64ToStr(b.BlockData.ReceiverRole) +
		b.BlockData.Data +
		b.BlockData.UseContract +
		contractRoot(b.BlockData.ContractData) +
		utils.Int64ToStr(b.BlockData.TimeStamp)

	b.ID = utils.D256C(blockMetaRoot+blockDataRoot, config.DELIUM_CONFIG.BLOCK_HASHING_PATH).String
}

// MessageMaker builds the exact byte string that the sender signs.
func MessageMaker(b BlockData) string {
	return b.SenderUID +
		utils.Int64ToStr(b.SenderRole) +
		b.SenderPubKey +
		b.ReceiverUID +
		utils.Int64ToStr(b.ReceiverRole) +
		b.Data +
		b.UseContract +
		contractRoot(b.ContractData) +
		utils.Int64ToStr(b.TimeStamp)
}

// BlockValidator checks the structure, the hash chain link and the signature of
// a block received from a peer.
func BlockValidator(block Block) error {
	if err := BlockStructValidator(block); err != nil {
		return err
	}

	// Recompute the hash with the recorded node UID.
	testBlock := block
	BlockHashMaker(&testBlock, block.BlockMeta.NodeUID)
	if block.ID != testBlock.ID {
		return errors.New("hash not match")
	}

	message := MessageMaker(block.BlockData)
	valid, err := utils.Verify(block.BlockData.SenderPubKey, block.BlockData.SenderUID, message, block.BlockData.Signature)
	if err != nil {
		return fmt.Errorf("signature verification: %w", err)
	}
	if !valid {
		return errors.New("signature is not valid")
	}

	return nil
}

// BlockStructValidator checks the shape of a block: required fields, lengths,
// hexadecimal encodings and timestamp windows.
func BlockStructValidator(b Block) error {

	// Block
	if utils.E_str(b.ID, 64) || !utils.IsHex(b.ID) {
		return errors.New("_id is required and must be a 64 character hex string")
	}

	// Block Meta
	if utils.E_str(b.BlockMeta.PreBlockHash, 64) || !utils.IsHex(b.BlockMeta.PreBlockHash) {
		return errors.New("preBlockHash is required and must be a 64 character hex string")
	}

	if utils.Gt_str(b.BlockMeta.NodeUID, 9) {
		return errors.New("nodeUid is required and must be longer than 9 characters")
	}

	if utils.Bt_int64(b.BlockMeta.TimeStamp, config.MIN_TIMESTAMP_MS, config.MAX_TIMESTAMP_MS) {
		return errors.New("timeStamp must be a valid unix format with milliseconds")
	}

	// Block Data
	if utils.E_str(b.BlockData.SenderUID, 32) || !utils.IsHex(b.BlockData.SenderUID) {
		return errors.New("senderUID is required and must be a 32 character hex string")
	}

	if utils.Bt_int64(b.BlockData.SenderRole, 0, 10000001) {
		return errors.New("senderRole is required and must be bigger than zero")
	}

	if utils.E_str(b.BlockData.SenderPubKey, 66) || !utils.IsHex(b.BlockData.SenderPubKey) {
		return errors.New("senderPubKey is required and must be a 66 character hex string")
	}

	if utils.E_str(b.BlockData.Signature, 128) || !utils.IsHex(b.BlockData.Signature) {
		return errors.New("signature is required and must be a 128 character hex string")
	}

	if utils.E_str(b.BlockData.ReceiverUID, 32) || !utils.IsHex(b.BlockData.ReceiverUID) {
		return errors.New("receiverUID is required and must be a 32 character hex string")
	}

	if utils.Bt_int64(b.BlockData.ReceiverRole, 0, 10000001) {
		return errors.New("receiverRole is required and must be bigger than zero")
	}

	if b.BlockData.Data == "" {
		return errors.New("data is required")
	}
	if utils.Lt_float(utils.StringSizeInKB(b.BlockData.Data), utils.StringToFloat64(config.MAX_DATA_SIZE_KB)) {
		errStr := "data is required and must be lesser than " + config.MAX_DATA_SIZE_KB + "KB"
		return errors.New(errStr)
	}

	if utils.Bt_int64(b.BlockData.TimeStamp, config.MIN_TIMESTAMP_MS, config.MAX_TIMESTAMP_MS) {
		return errors.New("timeStamp must be a valid unix format with milliseconds")
	}

	if b.BlockData.UseContract != "" && (utils.E_str(b.BlockData.UseContract, 66) || !utils.IsHex(b.BlockData.UseContract)) {
		return errors.New("useContract must be a 66 character hex string")
	}

	if b.BlockData.ContractData.Checksum != "" && (utils.E_str(b.BlockData.ContractData.Checksum, 64) || !utils.IsHex(b.BlockData.ContractData.Checksum)) {
		return errors.New("contract checksum must be a 64 character hex string")
	}

	return nil
}

// AreBlocksIdentical reports whether every block in the slice is deep-equal to
// the first one. An empty slice is vacuously true.
func AreBlocksIdentical(blocks []Block) bool {
	if len(blocks) == 0 {
		return true
	}

	firstBlock := blocks[0]
	for _, block := range blocks[1:] {
		if !reflect.DeepEqual(firstBlock, block) {
			return false
		}
	}
	return true
}
