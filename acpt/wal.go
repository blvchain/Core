package acpt

import (
	"encoding/binary"
	"os"
	"sync"
)

type WAL struct {
	file *os.File
	mu   sync.Mutex
}

func OpenWAL(filename string) (*WAL, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

// Append writes the batch to disk immediately
func (w *WAL) Append(batch []KeyValue) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Simple binary format: [Count][KLen][Key][VLen][Value]...
	if err := binary.Write(w.file, binary.LittleEndian, int32(len(batch))); err != nil {
		return err
	}

	for _, kv := range batch {
		// Write Key Length + Key
		binary.Write(w.file, binary.LittleEndian, int32(len(kv.Key)))
		w.file.Write(kv.Key)
		// Write Value Length + Value
		binary.Write(w.file, binary.LittleEndian, int32(len(kv.Value)))
		w.file.Write(kv.Value)
	}

	// Force sync to disk hardware
	return w.file.Sync()
}
