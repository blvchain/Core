package db

type BlockMeta struct {
	PreBlockHash string `bson:"preBlockHash,omitempty"`
	NodeUID      string `bson:"nodeId,omitempty"`
	TimeStamp    int64  `bson:"timeStamp,omitempty"`
}

type BlockData struct {
	SenderUID    string `bson:"senderUid,omitempty"`
	SenderRole   int64  `bson:"senderRole,omitempty"`
	SenderIndex  int64  `bson:"senderIndex,omitempty"`
	SenderPubKey string `bson:"senderPubKey,omitempty"`
	Signature    string `bson:"signature,omitempty"`
	ReceiverUID  string `bson:"receiverUid,omitempty"`
	ReceiverRole int64  `bson:"receiverRole,omitempty"`
	Data         string `bson:"data,omitempty"`
	TimeStamp    int64  `bson:"timeStamp,omitempty"`
}

type Block struct {
	BlockHash string    `bson:"blockHash,omitempty"`
	BlockMeta BlockMeta `bson:"blockMeta,omitempty"`
	BlockData BlockData `bson:"blockData,omitempty"`
}
