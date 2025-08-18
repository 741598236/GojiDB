package gojidb

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Snapshot struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	KeyCount  int       `json:"key_count"`
	DataSize  int64     `json:"data_size"`
}

func (db *GojiDB) CreateSnapshot() (*Snapshot, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	id := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())
	dir := filepath.Join(db.config.DataPath, SnapshotDir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	if err != nil {
		return nil, err
	}

	var totalSize int64
	activeKeyCount := 0
	for _, src := range files {
		dst := filepath.Join(dir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return nil, err
		}
		if st, _ := os.Stat(src); st != nil {
			totalSize += st.Size()
		}
	}

	db.keyDir.Range(func(key string, kd *KeyDir) bool {
		if !kd.Deleted {
			activeKeyCount++
		}
		return true
	})

	snap := &Snapshot{
		ID:        id,
		Timestamp: time.Now(),
		KeyCount:  activeKeyCount,
		DataSize:  totalSize,
	}
	meta, _ := json.Marshal(snap)
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), meta, DefaultFilePerm); err != nil {
		return nil, err
	}
	return snap, nil
}

func (db *GojiDB) RestoreFromSnapshot(id string) error {
	dir := filepath.Join(db.config.DataPath, SnapshotDir, id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("快照不存在: %s", id)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.activeFile != nil {
		db.activeFile.Close()
		db.activeFile = nil
	}
	for _, f := range db.readFiles {
		f.Close()
	}
	db.readFiles = make(map[uint32]*os.File)

	files, _ := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	for _, f := range files {
		os.Remove(f)
	}

	snapFiles, _ := filepath.Glob(filepath.Join(dir, "*.data"))
	for _, src := range snapFiles {
		dst := filepath.Join(db.config.DataPath, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	db.keyDir = NewConcurrentMap(16)
	if err := db.rebuildKeyDir(); err != nil {
		return err
	}
	return db.openActiveFile()
}

func (db *GojiDB) ListSnapshots() ([]*Snapshot, error) {
	dir := filepath.Join(db.config.DataPath, SnapshotDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*Snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	var list []*Snapshot
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaFile := filepath.Join(dir, e.Name(), "snapshot.json")
		b, err := os.ReadFile(metaFile)
		if err != nil {
			continue
		}
		var s Snapshot
		if json.Unmarshal(b, &s) == nil {
			list = append(list, &s)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Timestamp.After(list[j].Timestamp) })
	return list, nil
}

func (db *GojiDB) DeleteSnapshot(id string) error {
	return os.RemoveAll(filepath.Join(db.config.DataPath, SnapshotDir, id))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
