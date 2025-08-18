package GojiDB

import (
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	key       string
	value     []byte
	fileID    uint32
	valuePos  uint64
	valueSize uint32
	timestamp uint32
	accessed  time.Time
}

// LRUCache LRU缓存实现
type LRUCache struct {
	capacity int
	items    map[string]*list.Element
	lru      *list.List
	mu       sync.RWMutex
	hits     uint64
	misses   uint64
}

// NewLRUCache 创建新的LRU缓存
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get 从缓存获取数据
func (c *LRUCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if elem, found := c.items[key]; found {
		item := elem.Value.(*CacheItem)
		item.accessed = time.Now()
		c.lru.MoveToFront(elem)
		c.hits++
		return item.value, true
	}
	c.misses++
	return nil, false
}

// Set 设置缓存数据
func (c *LRUCache) Set(key string, value []byte, fileID uint32, valuePos uint64, valueSize uint32, timestamp uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, found := c.items[key]; found {
		c.lru.Remove(elem)
		delete(c.items, key)
	}

	if c.lru.Len() >= c.capacity {
		c.evictOldest()
	}

	item := &CacheItem{
		key:       key,
		value:     value,
		fileID:    fileID,
		valuePos:  valuePos,
		valueSize: valueSize,
		timestamp: timestamp,
		accessed:  time.Now(),
	}
	elem := c.lru.PushFront(item)
	c.items[key] = elem
}

// evictOldest 驱逐最老的缓存项
func (c *LRUCache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		item := elem.Value.(*CacheItem)
		c.lru.Remove(elem)
		delete(c.items, item.key)
	}
}

// Remove 移除缓存项
func (c *LRUCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, found := c.items[key]; found {
		c.lru.Remove(elem)
		delete(c.items, key)
	}
}

// Clear 清空缓存
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lru = list.New()
}

// Stats 获取缓存统计
func (c *LRUCache) Stats() (hits, misses uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}

// BlockCache 块级缓存
type BlockCache struct {
	cache     *LRUCache
	blockSize int
	maxBlocks int
	mu        sync.RWMutex
}

// NewBlockCache 创建新的块缓存
func NewBlockCache(maxBlocks int, blockSize int) *BlockCache {
	return &BlockCache{
		cache:     NewLRUCache(maxBlocks),
		blockSize: blockSize,
		maxBlocks: maxBlocks,
	}
}

// GetBlock 获取数据块
func (bc *BlockCache) GetBlock(fileID uint32, offset uint64, size uint32) ([]byte, bool) {
	key := bc.generateBlockKey(fileID, offset)
	return bc.cache.Get(key)
}

// SetBlock 设置数据块
func (bc *BlockCache) SetBlock(fileID uint32, offset uint64, data []byte) {
	key := bc.generateBlockKey(fileID, offset)
	bc.cache.Set(key, data, fileID, offset, uint32(len(data)), uint32(time.Now().Unix()))
}

// generateBlockKey 生成块缓存键
func (bc *BlockCache) generateBlockKey(fileID uint32, offset uint64) string {
	var buf [12]byte
	binary.BigEndian.PutUint32(buf[0:4], fileID)
	binary.BigEndian.PutUint64(buf[4:12], offset)
	hash := sha256.Sum256(buf[:])
	return string(hash[:])
}

// InvalidateFile 使文件相关的缓存失效
func (bc *BlockCache) InvalidateFile(fileID uint32) {
	// 简化实现：清空整个缓存
	bc.cache.Clear()
}

// KeyCache 键索引缓存
type KeyCache struct {
	cache *LRUCache
	mu    sync.RWMutex
}

// NewKeyCache 创建新的键缓存
func NewKeyCache(capacity int) *KeyCache {
	return &KeyCache{
		cache: NewLRUCache(capacity),
	}
}

// GetKeyDir 获取键的KeyDir信息
func (kc *KeyCache) GetKeyDir(key string) (*KeyDir, bool) {
	if data, found := kc.cache.Get(key); found {
		// 简化：将KeyDir序列化为字节数组存储
		// 实际应用中需要更高效的序列化方式
		return &KeyDir{
			FileID:    binary.BigEndian.Uint32(data[0:4]),
			ValuePos:  binary.BigEndian.Uint64(data[4:12]),
			ValueSize: binary.BigEndian.Uint32(data[12:16]),
		}, true
	}
	return nil, false
}

// SetKeyDir 设置键的KeyDir信息
func (kc *KeyCache) SetKeyDir(key string, keyDir *KeyDir) {
	var buf [16]byte
	binary.BigEndian.PutUint32(buf[0:4], keyDir.FileID)
	binary.BigEndian.PutUint64(buf[4:12], keyDir.ValuePos)
	binary.BigEndian.PutUint32(buf[12:16], keyDir.ValueSize)
	kc.cache.Set(key, buf[:], keyDir.FileID, keyDir.ValuePos, keyDir.ValueSize, uint32(time.Now().Unix()))
}

// RemoveKey 移除键的缓存
func (kc *KeyCache) RemoveKey(key string) {
	kc.cache.Remove(key)
}

// CacheStats 缓存统计信息
type CacheStats struct {
	BlockCacheHits   uint64
	BlockCacheMisses uint64
	KeyCacheHits     uint64
	KeyCacheMisses   uint64
	CacheSize        int
	CacheCapacity    int
}
