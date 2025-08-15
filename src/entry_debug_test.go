package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntrySerialization(t *testing.T) {
	// 测试条目序列化和反序列化的完整性
	tempDir := t.TempDir()
	conf := &GojiDBConfig{DataPath: tempDir}

	// 创建数据库
	db, err := NewGojiDB(conf)
	assert.NoError(t, err)

	// 创建测试条目
	key := "testKey"
	value := []byte("testValue")
	
	entry := &Entry{
		Timestamp:   1234567890,
		KeySize:     uint16(len(key)),
		ValueSize:   uint32(len(value)),
		Compression: NoCompression,
		Key:         []byte(key),
		Value:       value,
	}
	entry.CRC = db.calculateCRC(entry)

	// 序列化条目
	serialized, err := db.serializeEntry(entry)
	assert.NoError(t, err)
	t.Logf("序列化后大小: %d 字节", len(serialized))
	t.Logf("预期大小: %d 字节", HeaderSize+len(key)+len(value))

	// 手动写入文件
	filename := filepath.Join(tempDir, "test_entry.data")
	file, err := os.Create(filename)
	assert.NoError(t, err)
	_, err = file.Write(serialized)
	assert.NoError(t, err)
	file.Close()

	// 验证文件内容
	info, _ := os.Stat(filename)
	t.Logf("文件实际大小: %d 字节", info.Size())

	// 手动反序列化
	file, err = os.Open(filename)
	assert.NoError(t, err)
	defer file.Close()
	
	// 确保数据库在测试结束前正确关闭
	defer db.Close()

	// 读取头部
	header := make([]byte, HeaderSize)
	_, err = file.Read(header)
	assert.NoError(t, err)

	crc := binary.BigEndian.Uint32(header[0:4])
	timestamp := binary.BigEndian.Uint32(header[4:8])
	ttl := binary.BigEndian.Uint32(header[8:12])
	keySize := binary.BigEndian.Uint16(header[12:14])
	valueSize := binary.BigEndian.Uint32(header[14:18])
	compression := CompressionType(header[18])

	t.Logf("头部解析: CRC=%d, Timestamp=%d, TTL=%d, KeySize=%d, ValueSize=%d, Compression=%d", 
		crc, timestamp, ttl, keySize, valueSize, compression)

	// 读取键和值
	keyData := make([]byte, keySize)
	_, err = file.Read(keyData)
	assert.NoError(t, err)

	valueData := make([]byte, valueSize)
	_, err = file.Read(valueData)
	assert.NoError(t, err)

	t.Logf("键: %s, 值: %s", string(keyData), string(valueData))

	// 使用数据库的readEntry
	file.Seek(0, 0)
	reconstructed := &Entry{
		CRC:         crc,
		Timestamp:   timestamp,
		TTL:         ttl,
		KeySize:     keySize,
		ValueSize:   valueSize,
		Compression: compression,
		Key:         keyData,
		Value:       valueData,
	}

	// 验证CRC
	calculatedCRC := db.calculateCRC(reconstructed)
	t.Logf("原始CRC: %d, 计算CRC: %d", crc, calculatedCRC)
	assert.Equal(t, crc, calculatedCRC)
}