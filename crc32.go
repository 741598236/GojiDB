package gojidb

import (
	"hash/crc32"
)

// CRC32 CRC32计算器包装器
type CRC32 struct {
	table *crc32.Table
}

// NewCRC32 创建新的CRC32计算器
func NewCRC32() *CRC32 {
	return &CRC32{
		table: crc32.IEEETable,
	}
}

// Reset 重置计算器
func (c *CRC32) Reset() {
	// CRC32是无状态的，不需要重置
}

// Sum32 计算数据的CRC32校验和
func (c *CRC32) Sum32(data []byte) uint32 {
	return crc32.Checksum(data, c.table)
}

// Update 更新CRC值
func (c *CRC32) Update(crc uint32, data []byte) uint32 {
	return crc32.Update(crc, c.table, data)
}

// Checksum 计算数据的校验和
func (c *CRC32) Checksum(data []byte) uint32 {
	return c.Sum32(data)
}