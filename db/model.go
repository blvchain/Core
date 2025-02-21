package db

type BlockMeta struct {
	PreBlockHash string `bson:"preBlockHash,omitempty"`
	NodeUID      string `bson:"nodeUid,omitempty"`
	TimeStamp    int64  `bson:"timeStamp,omitempty"`
}

type BlockData struct {
	SenderUID    string `bson:"senderUid,omitempty"`
	SenderRole   int64  `bson:"senderRole,omitempty"`
	SenderPubKey string `bson:"senderPubKey,omitempty"`
	Signature    string `bson:"signature,omitempty"`
	ReceiverUID  string `bson:"receiverUid,omitempty"`
	ReceiverRole int64  `bson:"receiverRole,omitempty"`
	Data         string `bson:"data,omitempty"`
	TimeStamp    int64  `bson:"timeStamp,omitempty"`
}

type Block struct {
	ID        string    `bson:"_id,omitempty"`
	Boycott   bool      `bson:"boycott"`
	BlockMeta BlockMeta `bson:"blockMeta,omitempty"`
	BlockData BlockData `bson:"blockData,omitempty"`
}
