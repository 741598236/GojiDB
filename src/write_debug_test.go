package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteDataDebug(t *testing.T) {
	// 测试数据写入流程
	tempDir := t.TempDir()
	conf := &GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  true,
	}

	// 创建数据库
	db, err := NewGojiDB(conf)
	assert.NoError(t, err)
	assert.NotNil(t, db.activeFile, "activeFile应该不为nil")

	key := "testKey"
	value := []byte("testValue")

	// 写入数据前检查文件
	files, _ := filepath.Glob(filepath.Join(tempDir, "*.data"))
	t.Logf("写入前文件: %v", files)

	// 写入数据
	err = db.Put(key, value)
	assert.NoError(t, err)
	t.Log("数据写入完成")

	// 检查activeFile状态
	stat, err := db.activeFile.Stat()
	assert.NoError(t, err)
	t.Logf("activeFile大小: %d 字节", stat.Size())

	// 检查数据文件
	files, _ = filepath.Glob(filepath.Join(tempDir, "*.data"))
	t.Logf("写入后文件: %v", files)

	for _, f := range files {
		info, _ := os.Stat(f)
		t.Logf("文件 %s: %d 字节", f, info.Size())

		content, err := os.ReadFile(f)
		assert.NoError(t, err)
		if len(content) > 0 {
			t.Logf("文件内容: %d 字节", len(content))
		} else {
			t.Log("文件为空")
		}
	}

	// 验证数据可以获取
	retrieved, err := db.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrieved)
	t.Logf("获取数据成功: %s", string(retrieved))

	// 关闭数据库后重新验证
	err = db.Close()
	assert.NoError(t, err)

	// 重新打开数据库验证数据
	db2, err := NewGojiDB(conf)
	assert.NoError(t, err)

	retrieved2, err := db2.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrieved2)
	t.Logf("重新打开后获取数据成功: %s", string(retrieved2))

	err = db2.Close()
	assert.NoError(t, err)
}

func TestActiveFileCreation(t *testing.T) {
	// 测试activeFile创建
	tempDir := t.TempDir()
	conf := &GojiDBConfig{DataPath: tempDir}

	// 创建数据库
	db, err := NewGojiDB(conf)
	assert.NoError(t, err)
	assert.NotNil(t, db.activeFile)

	// 检查文件是否存在
	activeFilePath := filepath.Join(tempDir, "1.data")
	if _, err := os.Stat(activeFilePath); os.IsNotExist(err) {
		t.Log("activeFile文件不存在")
	} else {
		info, _ := os.Stat(activeFilePath)
		t.Logf("activeFile文件大小: %d 字节", info.Size())
	}

	err = db.Close()
	assert.NoError(t, err)
}
