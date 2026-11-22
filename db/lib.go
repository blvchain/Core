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

// ErrLegacyDatabase is returned when the database already contains a genesis
// block produced by an older, incompatible hashing profile. The hash cutover
// described in docs/MIGRATION.md changes every block ID, so an old chain cannot
// be read by this build; the operator must back it up and start a fresh
// database (or keep running the previous build).
var ErrLegacyDatabase = errors.New(
	"database was created by an incompatible hashing profile: the genesis block " +
		"does not match this build's genesis. See docs/MIGRATION.md")

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
		// The genesis block is deterministic for a given hashing profile. If the
		// stored one differs, the database belongs to a different (legacy)
		// profile and must not be silently reinterpreted.
		if existing[0].ID != genesis.ID {
			return false, fmt.Errorf("%w (stored %q, computed %q)",
				ErrLegacyDatabase, existing[0].ID, genesis.ID)
		}
		// Keep the stored copy: it is authoritative once the chain exists.
		genesis = existing[0]
	}

	config.FIRST_BLOCK_HASH = genesis.ID
	return true, nil
}

// contractRoot flattens the contract metadata into a stable, unambiguous string
// so it can be fed into the hash and the signed message. Length-prefixing (see
// utils.EncodeFields) means ("ab","c") and ("a","bc") cannot collide.
func contractRoot(c Contract) string {
	return utils.EncodeFields(
		c.Name,
		c.Version,
		c.Language,
		c.Compiler,
		c.Description,
		c.Checksum,
		c.Author,
		c.License,
	)
}

// BlockHashMaker computes the block ID from every field that defines the block.
//
// UseContract and ContractData are part of the digest: without them two blocks
// that differ only in their contract metadata would share an ID. The root is a
// length-prefixed encoding (utils.EncodeFields) hashed with domain-separated
// SHA-256 (utils.BlockDigest), so it is unambiguous and collision resistant.
func BlockHashMaker(b *Block, nodeUID string) {
	b.BlockMeta.NodeUID = nodeUID

	root := utils.EncodeFields(
		b.BlockMeta.PreBlockHash,
		b.BlockMeta.NodeUID,
		utils.Int64ToStr(b.BlockMeta.TimeStamp),
		b.BlockData.SenderUID,
		utils.Int64ToStr(b.BlockData.SenderRole),
		b.BlockData.SenderPubKey,
		b.BlockData.Signature,
		b.BlockData.ReceiverUID,
		utils.Int64ToStr(b.BlockData.ReceiverRole),
		b.BlockData.Data,
		b.BlockData.UseContract,
		contractRoot(b.BlockData.ContractData),
		utils.Int64ToStr(b.BlockData.TimeStamp),
	)

	b.ID = utils.BlockDigest(root)
}

// MessageMaker builds the exact string that the sender signs. Fields are
// length-prefixed so the encoding is unambiguous (see utils.EncodeFields).
func MessageMaker(b BlockData) string {
	return utils.EncodeFields(
		b.SenderUID,
		utils.Int64ToStr(b.SenderRole),
		b.SenderPubKey,
		b.ReceiverUID,
		utils.Int64ToStr(b.ReceiverRole),
		b.Data,
		b.UseContract,
		contractRoot(b.ContractData),
		utils.Int64ToStr(b.TimeStamp),
	)
}

// NodeMessage is the exact string a node signs to attest a block it produced.
// It binds the authenticated metadata (predecessor, node UID, timestamp) to the
// block ID, so the signature cannot be moved to a different block. The genesis
// block is not node-signed.
func NodeMessage(b Block) string {
	return utils.EncodeFields(
		"blvchain/node/v2",
		b.BlockMeta.PreBlockHash,
		b.BlockMeta.NodeUID,
		utils.Int64ToStr(b.BlockMeta.TimeStamp),
		b.ID,
	)
}

// SignBlock computes the block ID and attaches the node's signature over it.
// The signature is deliberately *not* part of the block ID: the ID commits to
// the contents, and the signature attests that ID.
func SignBlock(b *Block, nodePrivateKeyHex string) error {
	if b.BlockMeta.NodeUID == "" {
		b.BlockMeta.NodeUID = config.SELF_UID
	}
	b.BlockMeta.NodeSignature = ""
	BlockHashMaker(b, b.BlockMeta.NodeUID)

	signature, err := utils.Sign(nodePrivateKeyHex, NodeMessage(*b))
	if err != nil {
		return fmt.Errorf("sign block: %w", err)
	}
	b.BlockMeta.NodeSignature = signature
	return nil
}

// BlockValidator checks the structure, the hash chain link and the signatures
// of a block received from a peer.
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

	// NET-3: every non-genesis block must carry a signature from the node it
	// claims to have produced it, verifiable against the key pinned for that
	// node in dns_seed.json. Without this, any peer could attribute a block to
	// an arbitrary NodeUID.
	if block.ID != MakeGenesisBlock().ID {
		if block.BlockMeta.NodeSignature == "" {
			return errors.New("node signature is required")
		}
		nodePubKey, known := config.NodePublicKey(block.BlockMeta.NodeUID)
		if !known {
			return fmt.Errorf("node %q is not a known, keyed seed", block.BlockMeta.NodeUID)
		}
		nodeOK, nodeErr := utils.VerifySignature(nodePubKey, NodeMessage(block), block.BlockMeta.NodeSignature)
		if nodeErr != nil || !nodeOK {
			return fmt.Errorf("node signature verification: %v", nodeErr)
		}
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

	// NodeSignature is optional for the genesis block and required for every
	// other block (enforced by BlockValidator, which knows the genesis ID).
	if b.BlockMeta.NodeSignature != "" &&
		(utils.E_str(b.BlockMeta.NodeSignature, 128) || !utils.IsHex(b.BlockMeta.NodeSignature)) {
		return errors.New("nodeSignature must be a 128 character hex string")
	}

	// Block Data
	if utils.E_str(b.BlockData.SenderUID, 32) || !utils.IsHex(b.BlockData.SenderUID) {
		return errors.New("senderUID is required and must be a 32 character hex string")
	}

	if utils.Bt_int64(b.BlockData.SenderRole, 1, 10000000) {
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

	if utils.Bt_int64(b.BlockData.ReceiverRole, 1, 10000000) {
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
