package db

type NodeData struct {
	NodeUID   string `bson:"nodeid,omitempty"`
	Signature string `bson:"signature,omitempty"`
}

type Data struct {
	PreDataHash  string   `bson:"predatahash,omitempty"`
	Hash         string   `bson:"hash,omitempty"`
	SenderUID    string   `bson:"senderuid,omitempty"`
	SenderIndex  int64    `bson:"senderindex,omitempty"`
	SenderPubKey string   `bson:"senderpubkey,omitempty"`
	Signature    string   `bson:"signature,omitempty"`
	ReceiverUID  string   `bson:"receiveruid"`
	Data         string   `bson:"data,omitempty"`
	DataHash     string   `bson:"datahash,omitempty"`
	MessageHash  string   `bson:"messagehash,omitempty"`
	SenderRole   int64    `bson:"senderrole,omitempty"`
	ReceiverRole int64    `bson:"receiverrole"`
	TimeStamp    int64    `bson:"timestamp,omitempty"`
	NodeData     NodeData `bson:"nodedata,omitempty"`
}
