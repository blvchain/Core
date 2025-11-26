package acpt

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// 100 MB Limit per log file
	MaxLogSize = 100 * 1024 * 1024
)

type WALManager struct {
	dir         string
	currentFile *os.File
	currentID   int
	currentSize int64
	mu          sync.Mutex
}

func NewWALManager(dir string) (*WALManager, error) {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	wm := &WALManager{
		dir: dir,
	}

	// Start fresh with a new log file or rotate immediately
	// In a real recovery scenario, you would scan for the last file here.
	// For now, we start a new segment.
	if err := wm.rotateLog(); err != nil {
		return nil, err
	}

	return wm, nil
}

// rotateLog closes current file and opens next one (e.g., wal_0005.log)
func (w *WALManager) rotateLog() error {
	if w.currentFile != nil {
		w.currentFile.Sync()
		w.currentFile.Close()
	}

	w.currentID++
	filename := fmt.Sprintf("wal_%06d.log", w.currentID)
	path := filepath.Join(w.dir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	w.currentFile = f
	w.currentSize = 0

	fmt.Printf("[WAL] Rotated to new segment: %s\n", filename)
	return nil
}

// Append writes a batch of updates to disk sequentially
func (w *WALManager) Append(batch []KeyValue) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. Calculate Batch Size
	var batchSize int64
	for _, kv := range batch {
		// 4 bytes (key len) + Key + 4 bytes (val len) + Value
		batchSize += int64(8 + len(kv.Key) + len(kv.Value))
	}

	// 2. Check Rotation needed?
	if w.currentSize+batchSize > MaxLogSize {
		if err := w.rotateLog(); err != nil {
			return err
		}
	}

	// 3. Write Data
	for _, kv := range batch {
		// Write Key Length
		if err := binary.Write(w.currentFile, binary.LittleEndian, int32(len(kv.Key))); err != nil {
			return err
		}
		// Write Key
		if _, err := w.currentFile.Write(kv.Key); err != nil {
			return err
		}
		// Write Value Length
		if err := binary.Write(w.currentFile, binary.LittleEndian, int32(len(kv.Value))); err != nil {
			return err
		}
		// Write Value
		if _, err := w.currentFile.Write(kv.Value); err != nil {
			return err
		}
	}

	w.currentSize += batchSize

	// Sync ensures data hits the physical disk
	return w.currentFile.Sync()
}

// TruncateOldSegments deletes files that are safe to remove.
// It keeps a safety buffer of the last 2 files.
func (w *WALManager) TruncateOldSegments(currentID int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// BUFFER RULE: Keep current file and 2 previous files.
	// Example: If writing to 10, safe to delete 7 and below.
	safeThreshold := currentID - 2

	if safeThreshold < 1 {
		return nil
	}

	files, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, "wal_") && strings.HasSuffix(name, ".log") {
			var id int
			_, err := fmt.Sscanf(name, "wal_%06d.log", &id)
			if err == nil {
				if id < safeThreshold {
					path := filepath.Join(w.dir, name)
					fmt.Printf("[WAL] Cleanup: Deleting old segment %s\n", name)
					os.Remove(path)
				}
			}
		}
	}
	return nil
}

func (w *WALManager) GetCurrentSegmentID() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentID
}
