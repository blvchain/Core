package db

import "go.mongodb.org/mongo-driver/bson/primitive"

type BlockMeta struct {
	PreBlockHash primitive.Binary `bson:"h,omitempty"`
	NodeUID      primitive.Binary `bson:"u,omitempty"`
	TimeStamp    int64            `bson:"t,omitempty"`
}

type Contract struct {
	Name        primitive.Binary `bson:"n,omitempty"`
	Version     primitive.Binary `bson:"v,omitempty"`
	Language    primitive.Binary `bson:"l,omitempty"`
	Compiler    primitive.Binary `bson:"cm,omitempty"`
	Description primitive.Binary `bson:"d,omitempty"`
	Checksum    primitive.Binary `bson:"c,omitempty"`
	Author      primitive.Binary `bson:"a,omitempty"`
	License     primitive.Binary `bson:"l,omitempty"`
}

type VerifiableCredential struct {
	Name        primitive.Binary `bson:"n,omitempty"`
	Description primitive.Binary `bson:"d,omitempty"`
	Checksum    primitive.Binary `bson:"c,omitempty"`
	Author      primitive.Binary `bson:"a,omitempty"`
}

type BlockData struct {
	SenderUID    primitive.Binary     `bson:"su,omitempty"`
	SenderPubKey primitive.Binary     `bson:"sp,omitempty"`
	Signature    primitive.Binary     `bson:"g,omitempty"`
	ReceiverUID  primitive.Binary     `bson:"ru,omitempty"`
	UseContract  primitive.Binary     `bson:"uc,omitempty"`
	ContractData Contract             `bson:"cd,omitempty"`
	VC           VerifiableCredential `bson:"vc,omitempty"`
	TimeStamp    int64                `bson:"t,omitempty"`
}

// Main block struct
type Block struct {
	ID        primitive.Binary `bson:"_id,omitempty"`
	Boycott   bool             `bson:"y"`
	BlockMeta BlockMeta        `bson:"m,omitempty"`
	BlockData BlockData        `bson:"d,omitempty"`
}
