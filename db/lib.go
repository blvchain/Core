package db

import (
	"blvchain/core/config"
	"blvchain/core/utils"
	"errors"
	// "fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

func Genesis_check() (bool, error) {

	// Check for genesis Block
	var genesis_block Block = Block{
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

	// creating genesis block hash
	BlockHashMaker(&genesis_block, config.GENESIS_NODE_UID)

	db_genesis_blocks, _ := FindAllBlocks(bson.M{"blockMeta.preBlockHash": config.GENESIS_BLOCK_PREHASH})

	// No genesis block
	if len(db_genesis_blocks) == 0 {
		Block_insert_result, Block_insert_result_err := InsertOneBlock(genesis_block)
		if !Block_insert_result {
			return false, Block_insert_result_err
		}
	}

	config.FIRST_BLOCK_HASH = genesis_block.ID

	return true, nil
}

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
		utils.Int64ToStr(b.BlockData.TimeStamp)

	b.ID = utils.D256(blockMetaRoot+blockDataRoot, config.DELIUM_CONFIG.HASH.DELETE_STEP, config.DELIUM_CONFIG.HASH.REPEAT).String
}

func MessageMaker(b BlockData) string {
	return b.SenderUID +
		utils.Int64ToStr(b.SenderRole) +
		b.SenderPubKey +
		b.ReceiverUID +
		utils.Int64ToStr(b.ReceiverRole) +
		b.Data +
		utils.Int64ToStr(b.TimeStamp)
}

func BlockValidator(block Block) error {
	testBlock := block

	BlockHashMaker(&testBlock, block.BlockMeta.NodeUID)

	if block.ID != testBlock.ID {
		return errors.New("hash not match")
	}

	message := MessageMaker(block.BlockData)
	valid, validation_err := utils.Verify(block.BlockData.SenderPubKey, block.BlockData.SenderUID, message, block.BlockData.Signature)

	if !valid {
		return validation_err
	}

	return nil
}

func BlockStructValidator(b Block) error {

	// Block
	if utils.E_str(b.ID, 64) {
		return errors.New("_id is required and must be 64 len string")
	}

	if utils.BoolCheck(b.Boycott) {
		return errors.New("boycott is required")
	}

	// Block Meta
	if utils.E_str(b.BlockMeta.PreBlockHash, 64) {
		return errors.New("preBlockHash is required and must be 64 len string")
	}

	if utils.Gt_str(b.BlockMeta.NodeUID, 9) {
		return errors.New("nodeUid is required and must be greater than 9 len string")
	}

	if utils.Bt_int64(b.BlockMeta.TimeStamp, int64(1262304000000), int64(9262304000000)) {
		return errors.New("timeStamp must be a valid unix format with miliseconds")
	}

	// Block Data
	if utils.E_str(b.BlockData.SenderUID, 32) {
		return errors.New("senderUID is required and must be 32 len string")
	}

	if utils.Bt_int64(b.BlockData.SenderRole, 0, 10000001) {
		return errors.New("senderRole is required and must be bigger than zero")
	}

	if utils.E_str(b.BlockData.SenderPubKey, 66) {
		return errors.New("senderPubKey is required and must be 66 len string")
	}

	if utils.E_str(b.BlockData.Signature, 128) {
		return errors.New("signature is required and must be 128 len string")
	}

	if utils.E_str(b.BlockData.ReceiverUID, 32) {
		return errors.New("receiverUID is required and must be 32 len string")
	}

	if utils.Bt_int64(b.BlockData.ReceiverRole, 0, 10000001) {
		return errors.New("receiverRole is required and must be bigger than zero")
	}

	if utils.Lt_float(utils.StringSizeInKB(b.BlockData.Data), utils.StringToFloat64(config.MAX_DATA_SIZE)) {
		errStr := "data is required and must be lesser than " + config.MAX_DATA_SIZE + "KB"
		return errors.New(errStr)
	}

	if utils.Bt_int64(b.BlockData.TimeStamp, int64(1262304000000), int64(9262304000000)) {
		return errors.New("timeStamp must be a valid unix format with miliseconds")
	}

	return nil
}

func AreBlocksIdentical(blocks []Block) bool {

	firstBlock := blocks[0]

	for _, block := range blocks[1:] {
		if !reflect.DeepEqual(firstBlock, block) {
			return false
		}
	}

	return true
}
