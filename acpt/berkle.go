package acpt

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NodeFetcher is an interface to get a node from DB if not in RAM
type NodeFetcher func(hash []byte) (*DBNode, error)

// TreeSession holds the state for one block commit
type TreeSession struct {
	Fetcher     NodeFetcher
	NodesToSave map[string]DBNode // RAM Cache for this batch
}

// ApplyChanges takes the previous root, applies the batch, and returns new Root + Nodes to save
func ApplyChanges(rootHash []byte, batch []KeyValue, fetcher NodeFetcher) ([]byte, map[string]DBNode, error) {
	session := &TreeSession{
		Fetcher:     fetcher,
		NodesToSave: make(map[string]DBNode),
	}

	var currentRootHash = rootHash

	// Apply KV pairs sequentially (or sort them by Key for optimization)
	for _, kv := range batch {
		newRoot, err := session.insert(currentRootHash, kv.Key, kv.Value)
		if err != nil {
			return nil, nil, err
		}
		currentRootHash = newRoot
	}

	return currentRootHash, session.NodesToSave, nil
}

// insert is the recursive function that fixes the "Fetch Sibling" issue
func (s *TreeSession) insert(nodeHash []byte, key, value []byte) ([]byte, error) {
	// 1. Base Case: Empty Tree (or reached a nil child)
	if len(nodeHash) == 0 {
		return s.createLeaf(key, value), nil
	}

	// 2. Fetch the Node (Try Cache first, then DB)
	node, err := s.getNode(nodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch node %x: %v", nodeHash, err)
	}

	// 3. Compare Keys (Binary Search Tree Logic)
	cmp := bytes.Compare(key, node.Key.Data)

	if cmp < 0 {
		// Go LEFT
		newLeft, err := s.insert(node.Left.Data, key, value)
		if err != nil {
			return nil, err
		}

		// Update Left, keep Right (Sibling) same
		node.Left = toBin(newLeft)

	} else if cmp > 0 {
		// Go RIGHT
		newRight, err := s.insert(node.Right.Data, key, value)
		if err != nil {
			return nil, err
		}

		// Update Right, keep Left (Sibling) same
		node.Right = toBin(newRight)

	} else {
		// MATCH: Update Value
		node.ValueHash = toBin(sha256Sum(value))
	}

	// 4. Re-Hash this node (The Path Update)
	// This effectively pulls the unchanged sibling hash into the new parent hash
	return s.saveNode(node), nil
}

// getNode tries to find the node in the current batch cache, otherwise asks DB
func (s *TreeSession) getNode(hash []byte) (*DBNode, error) {
	// Try RAM Cache
	hashStr := string(hash)
	if node, ok := s.NodesToSave[hashStr]; ok {
		// Return a copy to avoid pointer mutation issues
		return &node, nil
	}
	// Try DB (The Fix: Fetching existing data)
	return s.Fetcher(hash)
}

// createLeaf creates a new node
func (s *TreeSession) createLeaf(key, value []byte) []byte {
	node := &DBNode{
		Key:       toBin(key),
		ValueHash: toBin(sha256Sum(value)),
		Height:    0,
	}
	return s.saveNode(node)
}

// saveNode calculates the hash and adds it to the write queue
func (s *TreeSession) saveNode(node *DBNode) []byte {
	// Hash = SHA256( Height + Key + ValueHash + LeftHash + RightHash )
	// This ensures integrity of the entire structure below this node
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, int64(node.Height))
	buf.Write(node.Key.Data)
	buf.Write(node.ValueHash.Data)
	buf.Write(node.Left.Data)
	buf.Write(node.Right.Data)

	hash := sha256Sum(buf.Bytes())
	node.ID = toBin(hash)

	// Store in batch map
	s.NodesToSave[string(hash)] = *node
	return hash
}

// Helpers
func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func toBin(data []byte) primitive.Binary {
	return primitive.Binary{Data: data, Subtype: 0}
}
