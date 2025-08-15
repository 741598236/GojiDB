package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshot(t *testing.T) {
	// 使用临时目录进行测试
	tempDir := t.TempDir()
	conf := &GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  true, // 确保数据同步到磁盘
	}

	db, err := NewGojiDB(conf)
	assert.NoError(t, err)

	// 写入数据并验证
	assert.NoError(t, db.Put("snapKey", []byte("snapVal")))
	
	// 验证数据确实写入了
	val, err := db.Get("snapKey")
	assert.NoError(t, err)
	assert.Equal(t, []byte("snapVal"), val)
	
	// 创建快照前确保数据写入
	db.activeFile.Sync()
	
	// 创建快照
	snap, err := db.CreateSnapshot()
	assert.NoError(t, err)
	assert.NotEmpty(t, snap.ID)

	// 验证快照目录中的文件
	snapDir := filepath.Join(tempDir, "snapshots", snap.ID)
	files, err := filepath.Glob(filepath.Join(snapDir, "*.data"))
	assert.NoError(t, err)
	assert.NotEmpty(t, files, "快照目录应该包含数据文件")
	
	// 删除数据
	assert.NoError(t, db.Delete("snapKey"))
	_, err = db.Get("snapKey")
	assert.Error(t, err)

	// 恢复快照
	assert.NoError(t, db.RestoreFromSnapshot(snap.ID))
	
	// 验证数据恢复
	val, err = db.Get("snapKey")
	assert.NoError(t, err)
	assert.Equal(t, []byte("snapVal"), val)
	
	db.Close()
}
