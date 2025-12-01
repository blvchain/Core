package db

import (
	"blvchain/core/config"
	"blvchain/core/utils"
	"bytes"
	"encoding/binary"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Genesis_check() (bool, error) {

	// Check for genesis Block
	var genesis_block Block = Block{
		Boycott: false,
		BlockMeta: BlockMeta{
			PreBlockHash: config.GENESIS_BLOCK_PREHASH,
			TimeStamp:    config.GENESIS_TIMESTAMP,
			NodeUID:      config.GENESIS_NODE_UID,
		},
		BlockData: BlockData{
			SenderUID:    config.GENESIS_SENDER_UID,
			SenderPubKey: config.GENESIS_PUBKEY,
			Signature:    config.GENESIS_SIGNATURE,
			ReceiverUID:  config.GENESIS_RECEIVER_UID,
			TimeStamp:    config.GENESIS_TIMESTAMP,
		},
	}

	// creating genesis block hash
	BlockHashMaker(&genesis_block)

	db_genesis_blocks, _ := FindAllBlocks(bson.M{"m.h": config.GENESIS_BLOCK_PREHASH})

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

func BlockHashMaker(b *Block) {
	tsMeta := make([]byte, 8)
	binary.BigEndian.PutUint64(tsMeta, uint64(b.BlockMeta.TimeStamp))

	tsData := make([]byte, 8)
	binary.BigEndian.PutUint64(tsData, uint64(b.BlockData.TimeStamp))

	boycott := make([]byte, 1)
	if b.Boycott {
		boycott[0] = 1
	} else {
		boycott[0] = 0
	}

	parts := [][]byte{
		b.ID.Data,

		boycott,

		b.BlockMeta.PreBlockHash.Data,
		b.BlockMeta.NodeUID.Data,
		tsMeta,

		b.BlockData.SenderUID.Data,
		b.BlockData.SenderPubKey.Data,
		b.BlockData.Signature.Data,
		b.BlockData.ReceiverUID.Data,
		b.BlockData.UseContract.Data,

		b.BlockData.ContractData.Name.Data,
		b.BlockData.ContractData.Version.Data,
		b.BlockData.ContractData.Language.Data,
		b.BlockData.ContractData.Compiler.Data,
		b.BlockData.ContractData.Description.Data,
		b.BlockData.ContractData.Checksum.Data,
		b.BlockData.ContractData.Author.Data,
		b.BlockData.ContractData.License.Data,

		b.BlockData.VC.Name.Data,
		b.BlockData.VC.Description.Data,
		b.BlockData.VC.Checksum.Data,
		b.BlockData.VC.Author.Data,

		tsData,
	}

	total := 0
	for _, p := range parts {
		total += len(p)
	}

	byteData := make([]byte, total)
	offset := 0

	for _, p := range parts {
		copy(byteData[offset:], p)
		offset += len(p)
	}

	blockHash, _ := utils.D256C(utils.ToMongoBinary(byteData), config.DELIUM_CONFIG.BLOCK_HASHING_PATH)

	b.ID = blockHash.Primitive_binary
}

func MessageMaker(b BlockData) primitive.Binary {

	parts := [][]byte{
		b.SenderUID.Data,
		b.SenderPubKey.Data,
		b.ReceiverUID.Data,
		b.UseContract.Data,
		utils.Int64ToBytes(b.TimeStamp),
	}

	total := 0
	for _, p := range parts {
		total += len(p)
	}

	byteData := make([]byte, total)
	offset := 0

	for _, p := range parts {
		copy(byteData[offset:], p)
		offset += len(p)
	}

	return utils.ToMongoBinary(byteData)

}

func BlockValidator(block Block) error {
	testBlock := block

	BlockHashMaker(&testBlock)

	if !bytes.Equal(block.ID.Data, testBlock.ID.Data) {
		return errors.New("hash not match")
	}

	message := MessageMaker(block.BlockData)
	valid, validation_err := utils.Verify(block.BlockData.SenderPubKey.Data, block.BlockData.SenderUID.Data, message.Data, block.BlockData.Signature.Data)

	if !valid {
		return validation_err
	}

	return nil
}
