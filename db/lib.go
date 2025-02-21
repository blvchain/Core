package db

import (
	"blvchain/core/config"
	"blvchain/core/utils"
	"errors"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

func Genesis_check() (bool, error) {

	// Check for genesis Block
	var genesis_block Block = Block{
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
	valid, _ := utils.Verify(block.BlockData.SenderPubKey, message, block.BlockData.SenderUID, block.BlockData.Signature)

	if !valid {
		return errors.New("not valid signature")
	}

	return nil
}

func StructValidator(s interface{}) error {
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return errors.New("provided value is not a struct")
	}

	// Iterate through fields
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := val.Type().Field(i)

		// Check for zero value
		if field.IsZero() {
			return fmt.Errorf("field '%s' is zero or empty", fieldType.Name)
		}
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
