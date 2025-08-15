package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWALBasicOperations(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024 * 1024, // 1MB
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 测试WAL管理器创建
	wal, err := NewWALManager(config)
	require.NoError(t, err)
	assert.NotNil(t, wal)
	defer wal.Close()

	// 测试基本写入操作
	record := &WALRecord{
		Type:      WALPut,
		Timestamp: uint64(time.Now().UnixNano()),
		Key:       "test_key",
		Value:     []byte("test_value"),
	}

	err = wal.WriteRecord(record)
	assert.NoError(t, err)

	// 验证WAL文件存在
	walFile := filepath.Join(tempDir, "wal.log")
	if _, err := os.Stat(walFile); err == nil {
		// 文件存在
	} else {
		assert.NoError(t, err)
	}
}

func TestWALCrashRecovery(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024 * 1024,
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库并写入一些数据
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	// 写入测试数据
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range testData {
		err := db.Put(k, []byte(v))
		assert.NoError(t, err)
	}

	// 模拟崩溃：关闭数据库但不清理WAL
	db.Close()

	// 重新打开数据库，应该触发恢复
	newDB, err := NewGojiDB(config)
	require.NoError(t, err)
	defer func() {
		if newDB != nil {
			newDB.Close()
		}
	}()

	// 验证数据已恢复
	for k, expected := range testData {
		value, err := newDB.Get(k)
		assert.NoError(t, err)
		assert.Equal(t, expected, string(value))
	}
}

func TestWALTransactionRecovery(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024 * 1024,
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	// 开始事务
	tx, err := db.BeginTransaction()
	require.NoError(t, err)

	// 在事务中执行操作
	err = tx.Put("tx_key1", []byte("tx_value1"))
	assert.NoError(t, err)

	err = tx.Put("tx_key2", []byte("tx_value2"))
	assert.NoError(t, err)

	// 提交事务
	err = tx.Commit()
	assert.NoError(t, err)

	// 模拟崩溃
	db.Close()

	// 重新打开数据库
	newDB, err := NewGojiDB(config)
	require.NoError(t, err)
	defer func() {
		if newDB != nil {
			newDB.Close()
		}
	}()

	// 验证事务数据已恢复
	value1, err := newDB.Get("tx_key1")
	assert.NoError(t, err)
	assert.Equal(t, "tx_value1", string(value1))

	value2, err := newDB.Get("tx_key2")
	assert.NoError(t, err)
	assert.Equal(t, "tx_value2", string(value2))
}

func TestWALRollbackRecovery(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024 * 1024,
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库并写入初始数据
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	err = db.Put("initial_key", []byte("initial_value"))
	assert.NoError(t, err)

	// 开始事务
	tx, err := db.BeginTransaction()
	require.NoError(t, err)

	// 在事务中执行操作
	err = tx.Put("tx_key", []byte("tx_value"))
	assert.NoError(t, err)

	// 回滚事务
	err = tx.Rollback()
	assert.NoError(t, err)

	// 模拟崩溃
	db.Close()

	// 重新打开数据库
	newDB, err := NewGojiDB(config)
	require.NoError(t, err)
	defer func() {
		if newDB != nil {
			newDB.Close()
		}
	}()

	// 验证初始数据存在，事务数据不存在
	value, err := newDB.Get("initial_key")
	assert.NoError(t, err)
	assert.Equal(t, "initial_value", string(value))

	_, err = newDB.Get("tx_key")
	assert.Error(t, err) // 应该返回 ErrKeyNotFound
}

func TestWALFileRotation(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024, // 1KB for testing rotation
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	// 写入大量数据以触发WAL文件轮转
	largeValue := make([]byte, 512)
	for i := 0; i < 10; i++ {
		err := db.Put(fmt.Sprintf("key_%d", i), largeValue)
		assert.NoError(t, err)
	}

	// 验证WAL文件存在
	walFile := filepath.Join(tempDir, "wal.log")
	if _, err := os.Stat(walFile); err != nil {
		assert.NoError(t, err)
	}
}

func TestWALCheckpoint(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024 * 1024,
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	// 写入测试数据
	for i := 0; i < 100; i++ {
		err := db.Put(fmt.Sprintf("key_%d", i), []byte(fmt.Sprintf("value_%d", i)))
		assert.NoError(t, err)
	}

	// 执行检查点
	if db.walManager != nil {
		err := db.walManager.Checkpoint()
		assert.NoError(t, err)
	}

	// 验证WAL文件仍然存在（checkpoint不会删除文件）
	walFile := filepath.Join(tempDir, "wal.log")
	if _, err := os.Stat(walFile); err != nil {
		assert.NoError(t, err)
	}
}

func TestWALDisable(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        false, // 禁用WAL
		WALMaxSize:       1024 * 1024,
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	// 写入数据
	err = db.Put("key", []byte("value"))
	assert.NoError(t, err)

	// 验证没有WAL文件
	walFiles, err := filepath.Glob(filepath.Join(tempDir, "*.wal"))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(walFiles))

	// 验证WAL管理器为nil
	assert.Nil(t, db.walManager)
}

func TestWALConcurrentWrites(t *testing.T) {
	tempDir := setupWALTest(t)
	defer cleanupWALTest(tempDir)

	config := &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALMaxSize:       1024 * 1024,
		WALSyncWrites:    true,
		WALFlushInterval: 1 * time.Second,
	}

	// 创建数据库
	db, err := NewGojiDB(config)
	require.NoError(t, err)
	defer db.Close()

	// 并发写入
	const numGoroutines = 10
	const numWrites = 100

	done := make(chan bool)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numWrites; j++ {
				key := fmt.Sprintf("goroutine_%d_key_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				err := db.Put(key, []byte(value))
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证数据完整性
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numWrites; j++ {
			key := fmt.Sprintf("goroutine_%d_key_%d", i, j)
			expected := fmt.Sprintf("value_%d_%d", i, j)

			value, err := db.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, expected, string(value))
		}
	}
}

// 辅助函数
func setupWALTest(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "gojidb_wal_test")
	require.NoError(t, err)
	return tempDir
}

func cleanupWALTest(tempDir string) {
	os.RemoveAll(tempDir)
}