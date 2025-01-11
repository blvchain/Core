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
	BlockHashMaker(&genesis_block)

	db_genesis_blocks, _ := FindAllBlocks(bson.M{"blockMeta.preBlockHash": config.GENESIS_BLOCK_PREHASH})

	// No genesis block
	if len(db_genesis_blocks) == 0 {
		Block_insert_result, Block_insert_result_err := InsertOneBlock(genesis_block)
		if !Block_insert_result {
			return false, Block_insert_result_err
		}
	}

	return true, nil
}

func BlockHashMaker(b *Block) {
	b.BlockMeta.NodeUID = config.SELF_UID

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

	b.BlockHash = utils.D256(blockMetaRoot+blockDataRoot, config.DELIUM_CONFIG.HASH.DELETE_STEP, config.DELIUM_CONFIG.HASH.REPEAT).String
}

func MessageMaker(b Block) string {
	return b.BlockData.SenderUID +
		utils.Int64ToStr(b.BlockData.SenderRole) +
		b.BlockData.SenderPubKey +
		b.BlockData.Signature +
		b.BlockData.ReceiverUID +
		utils.Int64ToStr(b.BlockData.ReceiverRole) +
		b.BlockData.Data +
		utils.Int64ToStr(b.BlockData.TimeStamp)
}

func BlockValidator(block Block) error {
	testBlock := block

	BlockHashMaker(&testBlock)

	if block.BlockHash != testBlock.BlockHash {
		return errors.New("hash not match")
	}

	message := MessageMaker(block)
	valid, _ := utils.Verify(block.BlockData.SenderPubKey, message, block.BlockData.Signature)

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
