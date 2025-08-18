package gojidb

import (
	"sync"
)

// BufferPool 内存池管理器
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool 创建新的内存池
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, size)
				return &buf
			},
		},
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() *[]byte {
	buf := p.pool.Get().(*[]byte)
	return buf
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf *[]byte) {
	if len(*buf) == p.size {
		p.pool.Put(buf)
	}
}

// AdaptiveBufferPool 自适应内存池
type AdaptiveBufferPool struct {
	pools []*BufferPool
	sizes []int
}

// NewAdaptiveBufferPool 创建自适应内存池
func NewAdaptiveBufferPool() *AdaptiveBufferPool {
	sizes := []int{1024, 4096, 16384, 65536, 262144, 1048576}
	pools := make([]*BufferPool, len(sizes))
	for i, size := range sizes {
		pools[i] = NewBufferPool(size)
	}

	return &AdaptiveBufferPool{
		pools: pools,
		sizes: sizes,
	}
}

// GetBuffer 根据需求获取最优缓冲区
func (p *AdaptiveBufferPool) GetBuffer(size int) *[]byte {
	for i, poolSize := range p.sizes {
		if size <= poolSize {
			return p.pools[i].Get()
		}
	}

	// 如果需求太大，直接分配
	buf := make([]byte, size)
	return &buf
}

// PutBuffer 归还缓冲区
func (p *AdaptiveBufferPool) PutBuffer(buf *[]byte) {
	size := len(*buf)
	for i, poolSize := range p.sizes {
		if size <= poolSize {
			p.pools[i].Put(buf)
			return
		}
	}
	// 如果是直接分配的，让GC回收
}

// CRC32Calculator CRC32计算器对象池
type CRC32Calculator struct {
	pool sync.Pool
}

// NewCRC32Calculator 创建CRC计算器池
func NewCRC32Calculator() *CRC32Calculator {
	return &CRC32Calculator{
		pool: sync.Pool{
			New: func() interface{} {
				return NewCRC32()
			},
		},
	}
}

// GetCalculator 获取CRC计算器
func (p *CRC32Calculator) GetCalculator() *CRC32 {
	return p.pool.Get().(*CRC32)
}

// PutCalculator 归还CRC计算器
func (p *CRC32Calculator) PutCalculator(crc *CRC32) {
	crc.Reset()
	p.pool.Put(crc)
}

// SerializationBuffer 序列化缓冲区对象池
type SerializationBuffer struct {
	pool sync.Pool
}

// NewSerializationBuffer 创建序列化缓冲区池
func NewSerializationBuffer() *SerializationBuffer {
	return &SerializationBuffer{
		pool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 0, 1024)
				return &buf
			},
		},
	}
}

// GetBuffer 获取序列化缓冲区
func (p *SerializationBuffer) GetBuffer() *[]byte {
	return p.pool.Get().(*[]byte)
}

// PutBuffer 归还序列化缓冲区
func (p *SerializationBuffer) PutBuffer(buf *[]byte) {
	*buf = (*buf)[:0]
	p.pool.Put(buf)
}

// MemoryAlign 内存对齐工具
func MemoryAlign(size int) int {
	const align = 64
	return (size + align - 1) &^ (align - 1)
}

// AlignedBuffer 对齐的缓冲区分配
func AlignedBuffer(size int) []byte {
	alignedSize := MemoryAlign(size)
	return make([]byte, alignedSize)
}

// GlobalPools 全局优化内存池
var (
	GlobalAdaptivePool = NewAdaptiveBufferPool()
	GlobalCRC32Pool    = NewCRC32Calculator()
	GlobalSerBufPool   = NewSerializationBuffer()
	GlobalEntryPool    = NewEntryPool()
	GlobalKeyDirPool   = NewKeyDirPool()
)

// StringPool 字符串对象池
type StringPool struct {
	pool sync.Pool
}

// NewStringPool 创建字符串池
func NewStringPool() *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(string)
			},
		},
	}
}

// GetString 获取字符串指针
func (p *StringPool) Get() *string {
	return p.pool.Get().(*string)
}

// PutString 归还字符串指针
func (p *StringPool) Put(s *string) {
	*s = "" // 清空内容
	p.pool.Put(s)
}

// EntryPool Entry对象池
type EntryPool struct {
	pool sync.Pool
}

// NewEntryPool 创建Entry对象池
func NewEntryPool() *EntryPool {
	return &EntryPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Entry{}
			},
		},
	}
}

// GetEntry 获取Entry对象
func (p *EntryPool) GetEntry() *Entry {
	return p.pool.Get().(*Entry)
}

// PutEntry 归还Entry对象
func (p *EntryPool) PutEntry(e *Entry) {
	e.Key = nil
	e.Value = nil
	e.Timestamp = 0
	e.TTL = 0
	e.KeySize = 0
	e.ValueSize = 0
	e.Compression = 0
	e.CRC = 0
	p.pool.Put(e)
}

// KeyDirPool KeyDir对象池
type KeyDirPool struct {
	pool sync.Pool
}

// NewKeyDirPool 创建KeyDir对象池
func NewKeyDirPool() *KeyDirPool {
	return &KeyDirPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &KeyDir{}
			},
		},
	}
}

// GetKeyDir 获取KeyDir对象
func (p *KeyDirPool) GetKeyDir() *KeyDir {
	return p.pool.Get().(*KeyDir)
}

// PutKeyDir 归还KeyDir对象
func (p *KeyDirPool) PutKeyDir(kd *KeyDir) {
	kd.FileID = 0
	kd.ValueSize = 0
	kd.ValuePos = 0
	kd.Timestamp = 0
	kd.TTL = 0
	kd.Compression = 0
	kd.Deleted = false
	p.pool.Put(kd)
}
