package main

import (
	"encoding/binary"
	"testing"
)

func TestDataSizeCalculation(t *testing.T) {
	// 创建一个测试条目，模拟实际写入的数据
	entry := &Entry{
		Key:         []byte("testKey"),
		Value:       []byte("testValue"),
		Timestamp:   123456789,
		TTL:         0,
		KeySize:     7,
		ValueSize:   9,
		Compression: 0,
	}

	// 计算序列化后的总大小
	totalSize := HeaderSize + int(entry.KeySize) + int(entry.ValueSize)
	t.Logf("HeaderSize: %d", HeaderSize)
	t.Logf("KeySize: %d", entry.KeySize)
	t.Logf("ValueSize: %d", entry.ValueSize)
	t.Logf("Total serialized size: %d", totalSize)

	// 序列化条目
	data, err := binarySerializer(entry)
	if err != nil {
		t.Fatalf("Failed to serialize entry: %v", err)
	}

	t.Logf("Actual serialized data length: %d", len(data))

	// 验证CRC
	entry.CRC = 12345 // 设置一个测试CRC
	crcData := make([]byte, 4)
	binary.BigEndian.PutUint32(crcData, entry.CRC)
	t.Logf("First 4 bytes (CRC): %v", crcData)
	showLen := 20
	if len(data) < showLen {
		showLen = len(data)
	}
	t.Logf("Serialized data: %v", data[:showLen])
}

func binarySerializer(e *Entry) ([]byte, error) {
	totalSize := HeaderSize + int(e.KeySize) + int(e.ValueSize)
	buf := make([]byte, totalSize)

	binary.BigEndian.PutUint32(buf[0:4], e.CRC)
	binary.BigEndian.PutUint32(buf[4:8], e.Timestamp)
	binary.BigEndian.PutUint32(buf[8:12], e.TTL)
	binary.BigEndian.PutUint16(buf[12:14], e.KeySize)
	binary.BigEndian.PutUint32(buf[14:18], e.ValueSize)
	buf[18] = byte(e.Compression)
	copy(buf[HeaderSize:HeaderSize+int(e.KeySize)], e.Key)
	copy(buf[HeaderSize+int(e.KeySize):totalSize], e.Value)
	return buf, nil
}