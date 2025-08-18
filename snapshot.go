package GojiDB

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Snapshot 表示数据库快照的元数据
// 包含快照的基本信息和统计
type Snapshot struct {
	ID        string    `json:"id"`        // 快照唯一标识符
	Timestamp time.Time `json:"timestamp"` // 创建时间
	KeyCount  int       `json:"key_count"` // 活跃键数量
	DataSize  int64     `json:"data_size"` // 数据文件总大小
}

// CreateSnapshot 创建数据库快照
// 复制所有数据文件并记录快照元信息
func (db *GojiDB) CreateSnapshot() (*Snapshot, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	id := fmt.Sprintf("snapshot_%d", time.Now().UnixNano())
	dir := filepath.Join(db.config.DataPath, SnapshotDir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// 收集所有数据文件
	files, err := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	if err != nil {
		return nil, err
	}

	// 复制数据文件并计算总大小
	var totalSize int64
	for _, src := range files {
		dst := filepath.Join(dir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return nil, err
		}
		if st, _ := os.Stat(src); st != nil {
			totalSize += st.Size()
		}
	}

	// 统计活跃键数量（排除已删除的键）
	activeKeyCount := 0
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

// RestoreFromSnapshot 从快照恢复数据库
// 替换当前所有数据文件并重建索引
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

	// 清理现有数据文件
	files, _ := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	for _, f := range files {
		os.Remove(f)
	}

	// 从快照恢复数据文件
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

// ListSnapshots 列出所有可用的快照
// 按创建时间降序排列
func (db *GojiDB) ListSnapshots() ([]*Snapshot, error) {
	dir := filepath.Join(db.config.DataPath, SnapshotDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*Snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	// 收集有效的快照信息
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
	
	// 按创建时间降序排列，最新的在前
	sort.Slice(list, func(i, j int) bool { return list[i].Timestamp.After(list[j].Timestamp) })
	return list, nil
}

// DeleteSnapshot 删除指定ID的快照
// 完全移除快照目录及其所有文件
func (db *GojiDB) DeleteSnapshot(id string) error {
	return os.RemoveAll(filepath.Join(db.config.DataPath, SnapshotDir, id))
}

// copyFile 复制文件内容
// 用于快照创建和恢复时的文件复制
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
