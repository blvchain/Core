package db

import (
	"testing"

	"blvchain/core/config"
)

// FuzzBlockHashMaker ensures the canonical block encoding and hash never panic
// on arbitrary field contents.
func FuzzBlockHashMaker(f *testing.F) {
	f.Add("id", "sender", "pubkey", "signature", "data", int64(1))
	f.Add("", "", "", "", "", int64(0))

	f.Fuzz(func(t *testing.T, id, sender, pubkey, signature, data string, ts int64) {
		block := Block{
			ID: id,
			BlockMeta: BlockMeta{
				PreBlockHash: id,
				NodeUID:      config.SELF_UID,
				TimeStamp:    ts,
			},
			BlockData: BlockData{
				SenderUID:    sender,
				SenderRole:   1,
				SenderPubKey: pubkey,
				Signature:    signature,
				ReceiverUID:  sender,
				ReceiverRole: 1,
				Data:         data,
				TimeStamp:    ts,
			},
		}

		BlockHashMaker(&block, config.SELF_UID)
		if len(block.ID) != 64 {
			t.Fatalf("block ID length = %d, want 64", len(block.ID))
		}
		if err := BlockStructValidator(block); err != nil {
			// Validation errors are fine; the point is that it does not panic.
			return
		}
	})
}
