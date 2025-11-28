package acpt

import "go.mongodb.org/mongo-driver/bson/primitive"

// KeyValue represents a raw data input
type KeyValue struct {
	Key   []byte
	Value []byte
}

// DBNode is the Merkle Tree Node stored in MongoDB
type DBNode struct {
	ID        primitive.Binary `bson:"_id"`         // Hash of this node
	Key       primitive.Binary `bson:"k,omitempty"` // The Search Key (for BST logic)
	ValueHash primitive.Binary `bson:"v,omitempty"` // Only for Leaf/Node with value

	// Children Hashes
	Left  primitive.Binary `bson:"l,omitempty"`
	Right primitive.Binary `bson:"r,omitempty"`

	// Height helps balance the tree (AVL logic) - optional but good for prod
	Height int `bson:"h"`
}

// GlobalState stores the latest Root Hash to persistent storage
type GlobalState struct {
	ID       string           `bson:"_id"` // constant "current_root"
	RootHash primitive.Binary `bson:"root_hash"`
}

// DBData is the raw value lookup
type DBData struct {
	Key   primitive.Binary `bson:"_id"`
	Value primitive.Binary `bson:"val"`
}
