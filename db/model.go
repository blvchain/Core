package db

import "go.mongodb.org/mongo-driver/bson"

type NodeData struct {
	NodeUID   string `bson:"nodeid,omitempty"`
	Signature string `bson:"signature,omitempty"`
}

type Data struct {
	PreDataHash  string   `bson:"predatahash,omitempty"`
	Hash         string   `bson:"hash,omitempty"`
	SenderUID    string   `bson:"senderuid,omitempty"`
	SenderPubKey string   `bson:"senderpubkey,omitempty"`
	Signature    string   `bson:"signature,omitempty"`
	ReceiverUID  string   `bson:"receiveruid"`
	Data         bson.Raw `bson:"data,omitempty"`
	SenderRole   int      `bson:"senderrole,omitempty"`
	ReceiverRole int      `bson:"receiverrole"`
	TimeStamp    int64    `bson:"timestamp,omitempty"`
	NodeData     NodeData `bson:"nodedata,omitempty"`
}

type DnsSeed struct {
	UID string `bson:"uid,omitempty"`
}
