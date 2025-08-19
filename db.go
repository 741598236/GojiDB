package GojiDB

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// GojiDB 是数据库的核心结构体，包含所有数据库组件和状态
// 采用读写锁保证并发安全，支持多线程访问
type GojiDB struct {
	config          *GojiDBConfig       // 数据库配置信息
	keyDir          *ConcurrentMap      // 内存中的键索引，存储键到文件位置的映射
	activeFile      *os.File            // 当前活跃的写入文件
	readFiles       map[uint32]*os.File // 只读文件句柄映射，用于读取历史数据
	activeFileID    uint32              // 当前活跃文件的唯一标识符
	mu              sync.RWMutex        // 读写锁，保护共享资源
	metrics         *Metrics            // 性能指标收集器
	ttlTicker       *time.Ticker        // TTL清理定时器
	stopChan        chan struct{}       // 优雅关闭信号通道
	ttlManager      *TTLManager         // TTL键管理器
	wg              sync.WaitGroup      // 等待组，确保所有goroutine完成
	blockCache      *BlockCache         // 块缓存，提高读取性能
	keyCache        *KeyCache           // 键缓存，加速键查找
	smartCompressor *SmartCompressor    // 智能压缩器
	walManager      *WALManager         // 预写日志管理器
	closed          bool                // 数据库关闭状态标志
}

func NewGojiDB(config *GojiDBConfig) (*GojiDB, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %v", err)
	}
	// 确保快照目录存在
	if err := os.MkdirAll(filepath.Join(config.DataPath, SnapshotDir), 0755); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %v", err)
	}

	// 根据配置决定是否启用缓存
	var blockCache *BlockCache
	var keyCache *KeyCache

	if config.CacheConfig.EnableCache {
		blockCache = NewBlockCache(config.CacheConfig.BlockCacheSize, 4096)
		keyCache = NewKeyCache(config.CacheConfig.KeyCacheSize)
	} else {
		blockCache = NewBlockCache(0, 4096) // 禁用缓存
		keyCache = NewKeyCache(0)           // 禁用缓存
	}

	db := &GojiDB{
		config:    config,
		keyDir:    NewConcurrentMap(16),
		readFiles: make(map[uint32]*os.File),
		metrics: &Metrics{
			StartTime:  time.Now(),
			SystemInfo: getSystemDetails(),
		},
		stopChan:   make(chan struct{}),
		blockCache: blockCache,
		keyCache:   keyCache,
	}

	if err := db.rebuildKeyDir(); err != nil {
		return nil, fmt.Errorf("重建索引失败: %v", err)
	}
	if err := db.openActiveFile(); err != nil {
		return nil, fmt.Errorf("打开活跃文件失败: %v", err)
	}
	if config.EnableTTL {
		db.ttlManager = NewTTLManager(func(key string) {
			db.Delete(key)
		})
	}

	// 初始化智能压缩器
	if config.SmartCompression != nil {
		db.smartCompressor = NewSmartCompressor(config.SmartCompression)
	}

	// 初始化WAL管理器
	if config.EnableWAL {
		wal, err := NewWALManager(config)
		if err != nil {
			return nil, fmt.Errorf("初始化WAL管理器失败: %v", err)
		}
		db.walManager = wal

		// 执行崩溃恢复
		if err := wal.Recover(db); err != nil {
			return nil, fmt.Errorf("WAL恢复失败: %v", err)
		}
	}

	return db, nil
}

func (db *GojiDB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil
	}
	db.closed = true
	db.mu.Unlock()

	if db.ttlTicker != nil {
		db.ttlTicker.Stop()
	}
	if db.ttlManager != nil {
		db.ttlManager.Stop()
	}
	if db.smartCompressor != nil {
		db.smartCompressor.Close()
	}
	if db.walManager != nil {
		db.walManager.Close()
	}

	if db.stopChan != nil {
		close(db.stopChan)
	}
	db.wg.Wait()

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.activeFile != nil {
		db.activeFile.Sync()
		db.activeFile.Close()
		db.activeFile = nil
	}
	for _, f := range db.readFiles {
		f.Close()
	}
	return nil
}

func (db *GojiDB) Put(key string, value []byte) error {
	return db.PutWithTTL(key, value, 0)
}

func (db *GojiDB) PutWithTTL(key string, value []byte, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("键不能为空")
	}
	if db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.TotalWrites, 1)
	}
	compressed, algorithm, err := db.compress(value)
	if err != nil {
		return fmt.Errorf("压缩失败: %v", err)
	}
	entry := &Entry{
		Timestamp:   uint32(time.Now().Unix()),
		KeySize:     uint16(len(key)),
		ValueSize:   uint32(len(compressed)),
		Compression: algorithm,
		Key:         []byte(key),
		Value:       compressed,
	}
	if ttl > 0 {
		entry.TTL = uint32(time.Now().Add(ttl).Unix())
	} else if ttl < 0 {
		// 负TTL立即过期，设置为当前时间
		entry.TTL = uint32(time.Now().Unix())
	}
	entry.CRC = db.calculateCRC(entry)

	db.mu.Lock()
	defer db.mu.Unlock()

	data, err := db.serializeEntry(entry)
	if err != nil {
		return err
	}

	if db.activeFile != nil {
		if stat, err := db.activeFile.Stat(); err == nil && stat.Size()+int64(len(data)) > db.config.MaxFileSize {
			if err := db.rotateActiveFile(); err != nil {
				return fmt.Errorf("轮转文件失败: %v", err)
			}
		}
	}

	pos, err := db.activeFile.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if _, err := db.activeFile.Write(data); err != nil {
		return err
	}
	if db.config.SyncWrites {
		db.activeFile.Sync()
	}

	// 写入WAL记录
	if db.walManager != nil {
		walRecord := &WALRecord{
			Type:      WALPut,
			Timestamp: uint64(time.Now().UnixNano()),
			Key:       key,
			Value:     value,
		}
		if err := db.walManager.WriteRecord(walRecord); err != nil {
			return fmt.Errorf("写入WAL失败: %v", err)
		}
	}

	// 使用新的并发Map写入
	db.keyDir.Set(key, &KeyDir{
		FileID:      db.activeFileID,
		ValueSize:   entry.ValueSize,
		ValuePos:    uint64(pos + HeaderSize + int64(entry.KeySize)),
		Timestamp:   entry.Timestamp,
		TTL:         entry.TTL,
		Compression: entry.Compression,
		Deleted:     false,
	})

	// 更新缓存
	db.keyCache.SetKeyDir(key, &KeyDir{
		FileID:      db.activeFileID,
		ValueSize:   entry.ValueSize,
		ValuePos:    uint64(pos + HeaderSize + int64(entry.KeySize)),
		Timestamp:   entry.Timestamp,
		Compression: entry.Compression,
		Deleted:     false,
	})

	// 使相关块缓存失效
	db.blockCache.InvalidateFile(db.activeFileID)

	// 每1000次写入自动触发后台压缩
	if db.config.EnableMetrics && atomic.LoadInt64(&db.metrics.TotalWrites)%1000 == 0 {
		go func() {
			time.Sleep(100 * time.Millisecond)
			db.Compact()
		}()
	}
	return nil
}

// 批量写入操作
func (db *GojiDB) BatchPut(items map[string][]byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(items) == 0 {
		return nil
	}

	// 预检查文件轮转需求
	totalSize := int64(0)
	for key, value := range items {
		if key == "" {
			continue
		}
		// 预估压缩后大小 + 头部开销
		estimatedSize := int64(len(value)) + HeaderSize + int64(len(key))
		totalSize += estimatedSize
	}

	// 预轮转检查
	if db.activeFile != nil && totalSize > 0 {
		if stat, err := db.activeFile.Stat(); err == nil && stat.Size()+totalSize > db.config.MaxFileSize {
			if err := db.rotateActiveFile(); err != nil {
				return fmt.Errorf("轮转文件失败: %v", err)
			}
		}
	}

	// 获取当前文件位置
	pos, _ := db.activeFile.Seek(0, io.SeekEnd)
	currentPos := pos

	// 使用对象池优化
	entry := GlobalEntryPool.GetEntry()
	defer GlobalEntryPool.PutEntry(entry)

	// 批量写入所有条目
	for key, value := range items {
		if key == "" {
			continue
		}

		// 使用内存池复用压缩缓冲区
		compressed, algorithm, err := db.compress(value)
		if err != nil {
			return fmt.Errorf("压缩失败: %v", err)
		}

		// 重置并复用Entry对象
		entry.Timestamp = uint32(time.Now().Unix())
		entry.KeySize = uint16(len(key))
		entry.ValueSize = uint32(len(compressed))
		entry.Compression = algorithm
		entry.Key = []byte(key)
		entry.Value = compressed
		entry.CRC = db.calculateCRC(entry)

		// 序列化到预分配缓冲区
		data, err := db.serializeEntry(entry)
		if err != nil {
			return err
		}

		// 单次批量写入
		if _, err := db.activeFile.Write(data); err != nil {
			return err
		}

		// 使用KeyDir池优化索引创建
		keyDir := GlobalKeyDirPool.GetKeyDir()
		keyDir.FileID = db.activeFileID
		keyDir.ValueSize = entry.ValueSize
		keyDir.ValuePos = uint64(currentPos + HeaderSize + int64(entry.KeySize))
		keyDir.Timestamp = entry.Timestamp
		keyDir.Compression = entry.Compression
		keyDir.Deleted = false

		db.keyDir.Set(key, keyDir)

		currentPos += int64(len(data))

		if db.config.EnableMetrics {
			atomic.AddInt64(&db.metrics.TotalWrites, 1)
		}
	}

	// 批量同步
	if db.config.SyncWrites {
		db.activeFile.Sync()
	}
	return nil
}

// 批量读取操作 - 内存优化版本
func (db *GojiDB) BatchGet(keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	// 预分配结果map，避免扩容
	results := make(map[string][]byte, len(keys))

	// 批量预读取文件信息，减少系统调用
	fileCache := make(map[uint32]*os.File)
	defer func() {
		// 及时关闭打开的文件
		for _, f := range fileCache {
			if f != nil {
				f.Close()
			}
		}
	}()

	// 使用内存池优化内存分配
	bufferPool := GlobalAdaptivePool

	// 优化的串行处理
	for _, key := range keys {
		if key == "" {
			results[key] = []byte{}
			continue
		}

		info, ok := db.keyDir.Get(key)
		if !ok || info.Deleted {
			results[key] = []byte{}
			continue
		}

		now := uint32(time.Now().Unix())
		if info.TTL > 0 && now >= info.TTL {
			results[key] = []byte{}
			continue
		}

		// 使用内存池获取缓冲区
		dataBuf := bufferPool.GetBuffer(int(info.ValueSize))
		data := *dataBuf

		var err error

		// 使用文件缓存减少重复打开
		if info.FileID == db.activeFileID {
			_, err = db.activeFile.ReadAt(data[:info.ValueSize], int64(info.ValuePos))
		} else {
			f, ok := fileCache[info.FileID]
			if !ok {
				filename := filepath.Join(db.config.DataPath, fmt.Sprintf("%d.data", info.FileID))
				f, err = os.Open(filename)
				if err != nil {
					bufferPool.PutBuffer(dataBuf) // 归还缓冲区
					results[key] = []byte{}
					continue
				}
				fileCache[info.FileID] = f
			}
			_, err = f.ReadAt(data[:info.ValueSize], int64(info.ValuePos))
		}

		if err == nil {
			decompressed, err := db.decompress(data[:info.ValueSize], info.Compression)
			if err == nil {
				results[key] = decompressed
			} else {
				results[key] = []byte{}
			}
		} else {
			results[key] = []byte{}
		}

		// 归还缓冲区
		bufferPool.PutBuffer(dataBuf)

		if db.config.EnableMetrics {
			atomic.AddInt64(&db.metrics.TotalReads, 1)
		}
	}

	return results, nil
}

// Get 根据键获取对应的值
func (db *GojiDB) Get(key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("键不能为空")
	}
	if db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.TotalReads, 1)
	}

	// 尝试从键缓存获取并检查TTL
	if cachedKeyDir, found := db.keyCache.GetKeyDir(key); found {
		if db.checkCachedTTL(cachedKeyDir) {
			go db.expireKey(key)
			return nil, fmt.Errorf("键已过期")
		}

		// 尝试从块缓存获取数据
		if data, ok := db.tryCacheRead(cachedKeyDir); ok {
			return data, nil
		}
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	info, ok := db.keyDir.Get(key)
	if !ok || info.Deleted {
		if db.config.EnableMetrics {
			atomic.AddInt64(&db.metrics.CacheMisses, 1)
		}
		return nil, fmt.Errorf("键不存在")
	}

	// 更新键缓存
	db.keyCache.SetKeyDir(key, info)

	if db.checkTTL(info) {
		go db.expireKey(key)
		return nil, fmt.Errorf("键已过期")
	}

	data, err := db.readData(info)
	if err != nil {
		return nil, err
	}

	result, err := db.decompress(data, info.Compression)
	if err != nil {
		if db.config.EnableMetrics {
			atomic.AddInt64(&db.metrics.CacheMisses, 1)
		}
		return nil, err
	}

	if db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.CacheHits, 1)
	}
	return result, nil
}

// Delete 删除指定的键及其对应的值
func (db *GojiDB) Delete(key string) error {
	if key == "" {
		return fmt.Errorf("键不能为空")
	}
	db.mu.Lock()
	defer db.mu.Unlock()

	// 检查键是否存在
	_, exists := db.keyDir.Get(key)
	if !exists {
		return fmt.Errorf("键不存在")
	}

	// 使用Entry对象池
	e := GlobalEntryPool.GetEntry()
	defer GlobalEntryPool.PutEntry(e)

	val, _, _ := db.compress([]byte(TombstoneValue))
	e.Timestamp = uint32(time.Now().Unix())
	e.KeySize = uint16(len(key))
	e.ValueSize = uint32(len(val))
	e.Compression = db.config.CompressionType
	e.Key = []byte(key)
	e.Value = val
	e.CRC = db.calculateCRC(e)

	data, _ := db.serializeEntry(e)
	if _, err := db.activeFile.Write(data); err != nil {
		return err
	}
	if db.config.SyncWrites {
		db.activeFile.Sync()
	}

	// 归还序列化缓冲区到内存池
	if len(data) > 0 {
		GlobalAdaptivePool.PutBuffer(&data)
	}

	// 写入WAL记录
	if db.walManager != nil {
		walRecord := &WALRecord{
			Type:      WALDelete,
			Timestamp: uint64(time.Now().UnixNano()),
			Key:       key,
		}
		if err := db.walManager.WriteRecord(walRecord); err != nil {
			return fmt.Errorf("写入WAL失败: %v", err)
		}
	}

	// 标记为已删除
	info, _ := db.keyDir.Get(key)
	info.Deleted = true
	info.Timestamp = e.Timestamp

	// 从 TTL 管理器中移除
	if db.ttlManager != nil {
		db.ttlManager.Remove(key)
	}

	// 从缓存中移除
	db.keyCache.RemoveKey(key)
	db.blockCache.InvalidateFile(db.activeFileID)

	// 更新指标
	if db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.TotalDeletes, 1)
	}

	return nil
}

// ListKeys 返回所有未过期的键列表
func (db *GojiDB) ListKeys() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	now := uint32(time.Now().Unix())
	keys := make([]string, 0, db.keyDir.Size())
	db.keyDir.Range(func(key string, info *KeyDir) bool {
		if !info.Deleted && (info.TTL == 0 || now <= info.TTL) {
			keys = append(keys, key)
		}
		return true
	})
	sort.Strings(keys)
	return keys
}

func (db *GojiDB) SetTTL(key string, ttl time.Duration) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	info, ok := db.keyDir.Get(key)
	if !ok || info.Deleted {
		return fmt.Errorf("键不存在")
	}
	if ttl > 0 {
		info.TTL = uint32(time.Now().Add(ttl).Unix())
	} else {
		info.TTL = 0
	}
	return nil
}

func (db *GojiDB) GetTTL(key string) (time.Duration, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	info, ok := db.keyDir.Get(key)
	if !ok || info.Deleted {
		return 0, fmt.Errorf("键不存在")
	}
	if info.TTL == 0 {
		return -1, nil
	}
	now := uint32(time.Now().Unix())
	if now >= info.TTL {
		return 0, fmt.Errorf("键已过期")
	}
	return time.Duration(int64(info.TTL)-int64(now)) * time.Second, nil
}

// ======== 内部实现 ========

// expireKey 异步删除过期键
func (db *GojiDB) expireKey(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if info, ok := db.keyDir.Get(key); ok && !info.Deleted {
		_ = db.deleteInternal(key)
	}
}

func (db *GojiDB) startTTLCleanup() {
	db.ttlTicker = time.NewTicker(db.config.TTLCheckInterval)
	db.wg.Add(1)
	go func() {
		defer db.wg.Done()
		for {
			select {
			case <-db.ttlTicker.C:
				db.cleanupExpiredKeys()
			case <-db.stopChan:
				return
			}
		}
	}()
}

func (db *GojiDB) cleanupExpiredKeys() {
	db.mu.Lock()
	defer db.mu.Unlock()
	now := uint32(time.Now().Unix())
	expired := make([]string, 0, 16)
	db.keyDir.Range(func(k string, info *KeyDir) bool {
		if !info.Deleted && info.TTL > 0 && now >= info.TTL {
			expired = append(expired, k)
		}
		return true
	})
	for _, k := range expired {
		_ = db.deleteInternal(k)
		if db.config.EnableMetrics {
			atomic.AddInt64(&db.metrics.ExpiredKeyCount, 1)
		}
	}
}

// checkCachedTTL 检查缓存中的键是否过期
func (db *GojiDB) checkCachedTTL(info *KeyDir) bool {
	if info.TTL == 0 {
		return false
	}

	now := uint32(time.Now().Unix())
	expired := now >= info.TTL

	if expired && db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.ExpiredKeyCount, 1)
	}

	return expired
}

// checkTTL 检查键是否过期
func (db *GojiDB) checkTTL(info *KeyDir) bool {
	if info.TTL == 0 {
		return false
	}

	now := uint32(time.Now().Unix())
	expired := now >= info.TTL

	if expired && db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.ExpiredKeyCount, 1)
	}

	return expired
}

// tryCacheRead 尝试从缓存直接读取数据
func (db *GojiDB) tryCacheRead(info *KeyDir) ([]byte, bool) {
	if cachedData, found := db.blockCache.GetBlock(info.FileID, info.ValuePos, info.ValueSize); found {
		result, err := db.decompress(cachedData, info.Compression)
		if err == nil {
			if db.config.EnableMetrics {
				atomic.AddInt64(&db.metrics.CacheHits, 1)
			}
			return result, true
		}
	}
	return nil, false
}

// readData 读取键对应的数据
func (db *GojiDB) readData(info *KeyDir) ([]byte, error) {
	// 计算块对齐的读取位置
	blockStart := (info.ValuePos / uint64(db.blockCache.blockSize)) * uint64(db.blockCache.blockSize)

	// 尝试从块缓存获取
	if cachedData, found := db.blockCache.GetBlock(info.FileID, blockStart, uint32(db.blockCache.blockSize)); found {
		data := db.extractDataFromBlock(cachedData, info, blockStart)
		if data != nil {
			return data, nil
		}
	}

	// 从文件读取
	return db.readFromFileAndCache(info, blockStart)
}

// extractDataFromBlock 从块缓存中提取数据
func (db *GojiDB) extractDataFromBlock(blockData []byte, info *KeyDir, blockStart uint64) []byte {
	offsetInBlock := info.ValuePos - blockStart
	if offsetInBlock+uint64(info.ValueSize) <= uint64(len(blockData)) {
		data := make([]byte, info.ValueSize)
		copy(data, blockData[offsetInBlock:offsetInBlock+uint64(info.ValueSize)])
		return data
	}
	return nil
}

// readFromFileAndCache 从文件读取数据并更新缓存
func (db *GojiDB) readFromFileAndCache(info *KeyDir, blockStart uint64) ([]byte, error) {
	data := make([]byte, info.ValueSize)

	// 读取实际数据
	var err error
	if info.FileID == db.activeFileID {
		_, err = db.activeFile.ReadAt(data, int64(info.ValuePos))
	} else {
		filename := filepath.Join(db.config.DataPath, fmt.Sprintf("%d.data", info.FileID))
		f, openErr := os.Open(filename)
		if openErr != nil {
			return nil, openErr
		}
		defer f.Close()
		_, err = f.ReadAt(data, int64(info.ValuePos))
	}

	if err != nil {
		return nil, err
	}

	// 更新块缓存
	db.updateBlockCache(info, blockStart)

	return data, nil
}

// updateBlockCache 更新块缓存
func (db *GojiDB) updateBlockCache(info *KeyDir, blockStart uint64) {
	blockData := make([]byte, db.blockCache.blockSize)

	if info.FileID == db.activeFileID {
		_, _ = db.activeFile.ReadAt(blockData, int64(blockStart))
	} else {
		filename := filepath.Join(db.config.DataPath, fmt.Sprintf("%d.data", info.FileID))
		if f, err := os.Open(filename); err == nil {
			_, _ = f.ReadAt(blockData, int64(blockStart))
			f.Close() // 立即关闭文件
		}
	}

	db.blockCache.SetBlock(info.FileID, blockStart, blockData)
}

func (db *GojiDB) rebuildKeyDir() error {
	files, err := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		db.activeFileID = 1
		return nil
	}
	maxID := uint32(0)
	for _, f := range files {
		id, err := db.parseFileID(f)
		if err == nil && id > maxID {
			maxID = id
		}
	}
	db.activeFileID = maxID

	// 按文件ID数字排序，而不是字符串排序
	sort.Slice(files, func(i, j int) bool {
		id1, err1 := db.parseFileID(files[i])
		id2, err2 := db.parseFileID(files[j])
		if err1 != nil || err2 != nil {
			return files[i] < files[j]
		}
		return id1 < id2
	})

	for _, f := range files {
		if err := db.processFile(f); err != nil {
			fmt.Printf("跳过损坏文件 %s: %v\n", f, err)
		}
	}
	return nil
}

func (db *GojiDB) processFile(filename string) error {
	id, err := db.parseFileID(filename)
	if err != nil {
		return err
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	fmt.Printf("处理文件 %s: ID=%d, 大小=%d\n", filename, id, stat.Size())

	r := bufio.NewReader(f)
	offset := int64(0)
	entryCount := 0

	for {
		e, sz, err := db.readEntry(r)
		if err == io.EOF {
			fmt.Printf("文件 %s 处理完成: 找到 %d 个有效条目\n", filename, entryCount)
			break
		}
		if err != nil {
			fmt.Printf("读取条目错误: %v, 跳过 %d 字节\n", err, sz)
			offset += int64(sz)
			continue
		}

		if !db.verifyCRC(e) {
			fmt.Printf("CRC验证失败, 跳过条目\n")
			offset += int64(sz)
			continue
		}

		key := string(e.Key)
		valuePos := offset + HeaderSize + int64(e.KeySize)
		val, _ := db.decompress(e.Value, e.Compression)
		isTomb := string(val) == TombstoneValue

		// 调试输出已移除: key=%s, valueSize=%d, timestamp=%d, tombstone=%v
		// 原调试代码: fmt.Printf(...)

		if old, ok := db.keyDir.Get(key); !ok || e.Timestamp >= old.Timestamp {
			db.keyDir.Set(key, &KeyDir{
				FileID:      id,
				ValueSize:   e.ValueSize,
				ValuePos:    uint64(valuePos),
				Timestamp:   e.Timestamp,
				TTL:         e.TTL,
				Compression: e.Compression,
				Deleted:     isTomb,
			})
			// 调试输出已移除: 更新keyDir: key=%s, fileID=%d
			// 原调试代码: fmt.Printf(...)
			entryCount++
		}
		offset += int64(sz)
	}
	return nil
}

func (db *GojiDB) parseFileID(filename string) (uint32, error) {
	base := filepath.Base(filename)
	name := strings.TrimSuffix(base, ".data")
	id, err := strconv.ParseUint(name, 10, 32)
	return uint32(id), err
}

func (db *GojiDB) openActiveFile() error {
	filename := filepath.Join(db.config.DataPath, fmt.Sprintf("%d.data", db.activeFileID))
	if db.activeFile != nil {
		db.activeFile.Close()
		db.activeFile = nil
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, DefaultFilePerm)
	if err != nil {
		return err
	}
	db.activeFile = f
	return nil
}

func (db *GojiDB) rotateActiveFile() error {
	if db.activeFile != nil {
		db.activeFile.Sync()
		db.activeFile.Close()
		readFile, err := os.Open(filepath.Join(db.config.DataPath, fmt.Sprintf("%d.data", db.activeFileID)))
		if err == nil {
			db.readFiles[db.activeFileID] = readFile
		} else {
			// 如果打开失败，确保关闭文件句柄
			if readFile != nil {
				readFile.Close()
			}
		}
	}
	db.activeFileID++
	return db.openActiveFile()
}

func (db *GojiDB) verifyCRC(e *Entry) bool {
	return db.calculateCRC(e) == e.CRC
}

func (db *GojiDB) deleteInternal(key string) error {
	info, ok := db.keyDir.Get(key)
	if !ok || info.Deleted {
		return fmt.Errorf("键不存在")
	}
	val, _, _ := db.compress([]byte(TombstoneValue))
	e := &Entry{
		Timestamp:   uint32(time.Now().Unix()),
		KeySize:     uint16(len(key)),
		ValueSize:   uint32(len(val)),
		Compression: db.config.CompressionType,
		Key:         []byte(key),
		Value:       val,
	}
	e.CRC = db.calculateCRC(e)
	data, _ := db.serializeEntry(e)
	if _, err := db.activeFile.Write(data); err != nil {
		return err
	}
	if db.config.SyncWrites {
		db.activeFile.Sync()
	}
	info.Deleted = true
	info.Timestamp = e.Timestamp
	return nil
}

// Compact 执行数据合并压缩，清理已删除和过期的键
func (db *GojiDB) Compact() error {
	if db.config.EnableMetrics {
		atomic.AddInt64(&db.metrics.CompactionCount, 1)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// 1. 创建临时合并文件
	compactFileName, compactFile, err := db.createCompactFile()
	if err != nil {
		return err
	}
	defer func() {
		compactFile.Close()
		os.Remove(compactFileName) // 清理可能的残留文件
	}()

	// 2. 收集活跃键并重写数据
	activeKeys := db.collectActiveKeys()
	newKeyDir, _, err := db.rewriteActiveKeys(compactFile, activeKeys)
	if err != nil {
		return err
	}

	// 3. 完成合并：同步文件、清理旧文件、更新状态
	return db.finalizeCompaction(compactFileName, newKeyDir)
}

// createCompactFile 创建用于合并的临时文件
func (db *GojiDB) createCompactFile() (string, *os.File, error) {
	compactFileName := filepath.Join(db.config.DataPath, fmt.Sprintf("compact_%d.data", time.Now().UnixNano()))
	compactFile, err := os.Create(compactFileName)
	if err != nil {
		return "", nil, fmt.Errorf("创建合并文件失败: %v", err)
	}
	return compactFileName, compactFile, nil
}

// collectActiveKeys 收集所有未删除且未过期的键
func (db *GojiDB) collectActiveKeys() map[string]*KeyDir {
	activeKeys := make(map[string]*KeyDir)
	now := uint32(time.Now().Unix())

	db.keyDir.Range(func(key string, kd *KeyDir) bool {
		if !kd.Deleted && (kd.TTL == 0 || now <= kd.TTL) {
			activeKeys[key] = kd
		}
		return true
	})

	return activeKeys
}

// rewriteActiveKeys 将活跃键的数据重写到新文件
func (db *GojiDB) rewriteActiveKeys(compactFile *os.File, activeKeys map[string]*KeyDir) (map[string]*KeyDir, int64, error) {
	newKeyDir := make(map[string]*KeyDir)
	offset := int64(0)

	for key, oldKD := range activeKeys {
		data, err := db.readKeyData(oldKD)
		if err != nil {
			continue // 跳过读取失败的键
		}

		newData, compressionType, err := db.processKeyData(data, oldKD)
		if err != nil {
			continue // 跳过重写失败的键
		}

		entry := db.createEntry(key, newData, oldKD, compressionType)
		serialized, err := db.serializeEntry(entry)
		if err != nil {
			continue // 跳过序列化失败的键
		}

		if _, err := compactFile.Write(serialized); err != nil {
			continue // 跳过写入失败的键
		}

		newKeyDir[key] = &KeyDir{
			FileID:      1, // 新文件ID固定为1
			ValueSize:   entry.ValueSize,
			ValuePos:    uint64(offset + HeaderSize + int64(entry.KeySize)),
			Timestamp:   entry.Timestamp,
			TTL:         entry.TTL,
			Compression: entry.Compression,
			Deleted:     false,
		}
		offset += int64(len(serialized))
	}

	return newKeyDir, offset, nil
}

// readKeyData 从文件读取键对应的数据
func (db *GojiDB) readKeyData(kd *KeyDir) ([]byte, error) {
	data := make([]byte, kd.ValueSize)
	var err error

	if kd.FileID == db.activeFileID {
		_, err = db.activeFile.ReadAt(data, int64(kd.ValuePos))
	} else {
		filename := filepath.Join(db.config.DataPath, fmt.Sprintf("%d.data", kd.FileID))
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		_, err = f.ReadAt(data, int64(kd.ValuePos))
	}

	return data, err
}

// processKeyData 处理键数据：解压缩并重新压缩
func (db *GojiDB) processKeyData(data []byte, oldKD *KeyDir) ([]byte, CompressionType, error) {
	// 解压缩数据
	val, err := db.decompress(data, oldKD.Compression)
	if err != nil {
		// 解压缩失败，使用原始数据
		val = data
	}

	// 重新压缩数据
	newCompressed, compressionType, err := db.compress(val)
	if err != nil {
		// 压缩失败，使用原始数据
		newCompressed = val
		compressionType = NoCompression
	}

	return newCompressed, compressionType, nil
}

// createEntry 创建新的条目
func (db *GojiDB) createEntry(key string, data []byte, oldKD *KeyDir, compressionType CompressionType) *Entry {
	entry := &Entry{
		Timestamp:   oldKD.Timestamp,
		TTL:         oldKD.TTL,
		KeySize:     uint16(len(key)),
		ValueSize:   uint32(len(data)),
		Compression: compressionType,
		Key:         []byte(key),
		Value:       data,
	}
	entry.CRC = db.calculateCRC(entry)
	return entry
}

// finalizeCompaction 完成合并过程
func (db *GojiDB) finalizeCompaction(compactFileName string, newKeyDir map[string]*KeyDir) error {
	// 清理旧文件和状态
	if err := db.cleanupOldFiles(compactFileName); err != nil {
		return err
	}

	// 重命名合并文件
	newName := filepath.Join(db.config.DataPath, "1.data")
	if err := os.Rename(compactFileName, newName); err != nil {
		return fmt.Errorf("重命名合并文件失败: %v", err)
	}

	// 更新数据库状态
	return db.updateDatabaseState(newKeyDir)
}

// cleanupOldFiles 清理旧的文件和句柄
func (db *GojiDB) cleanupOldFiles(compactFileName string) error {
	// 关闭所有文件句柄
	if db.activeFile != nil {
		_ = db.activeFile.Close()
		db.activeFile = nil
	}

	for _, f := range db.readFiles {
		_ = f.Close()
	}
	db.readFiles = make(map[uint32]*os.File)

	// 删除旧的数据文件
	oldFiles, err := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	if err != nil {
		return err
	}

	compactBase := filepath.Base(compactFileName)
	for _, f := range oldFiles {
		if filepath.Base(f) != compactBase {
			_ = os.Remove(f)
		}
	}

	return nil
}

// updateDatabaseState 更新数据库状态
func (db *GojiDB) updateDatabaseState(newKeyDir map[string]*KeyDir) error {
	// 更新元数据
	db.activeFileID = 1
	db.keyDir = NewConcurrentMap(16)

	for key, kd := range newKeyDir {
		db.keyDir.Set(key, kd)
	}

	return db.openActiveFile()
}

// UI辅助函数

// printFancyBox 打印带边框的文本框
func printFancyBox(title string, content []string, color string, width int) {
	if width < 50 {
		width = 50
	}

	// 打印顶部边框
	fmt.Printf("%s┌", color)
	for i := 0; i < width-2; i++ {
		fmt.Print("─")
	}
	fmt.Printf("┐%s\n", ColorReset)

	// 打印标题行
	titleRealLen := calculateRealLength(title)
	titlePadding := width - titleRealLen - 2
	leftPadding := titlePadding / 2
	rightPadding := titlePadding - leftPadding

	fmt.Printf("%s│%s", color, ColorReset)
	for i := 0; i < leftPadding; i++ {
		fmt.Print(" ")
	}
	fmt.Print(title)
	for i := 0; i < rightPadding; i++ {
		fmt.Print(" ")
	}
	fmt.Printf("%s│%s\n", color, ColorReset)

	// 打印分隔线
	fmt.Printf("%s├", color)
	for i := 0; i < width-2; i++ {
		fmt.Print("─")
	}
	fmt.Printf("┤%s\n", ColorReset)

	// 打印内容行
	for _, line := range content {
		// 计算这一行的实际显示长度
		lineRealLen := calculateRealLength(line)

		// 计算需要填充的空格数量
		// width-2 是总可用空间，-1 是左边空格，lineRealLen 是内容长度
		paddingNeeded := width - 2 - 1 - lineRealLen

		// 确保padding不为负数
		if paddingNeeded < 0 {
			paddingNeeded = 0
		}

		// 打印：左边框 + 空格 + 内容 + 填充空格 + 右边框
		fmt.Printf("%s│%s %s", color, ColorReset, line)
		for i := 0; i < paddingNeeded; i++ {
			fmt.Print(" ")
		}
		fmt.Printf("%s│%s\n", color, ColorReset)
	}

	// 打印底部边框
	fmt.Printf("%s└", color)
	for i := 0; i < width-2; i++ {
		fmt.Print("─")
	}
	fmt.Printf("┘%s\n", ColorReset)
}

// calculateRealLength 计算字符串的实际显示宽度
func calculateRealLength(s string) int {
	length := 0
	inEscape := false
	i := 0

	for i < len(s) {
		if i+1 < len(s) && s[i] == '\x1b' && s[i+1] == '[' {
			// 跳过ANSI转义序列
			inEscape = true
			i += 2
			continue
		}

		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			i++
			continue
		}

		// 获取当前字符的UTF-8编码
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError {
			i++
			continue
		}

		// 计算字符的显示宽度
		width := getRuneWidth(r)
		length += width
		i += size
	}

	return length
}

// getRuneWidth 获取字符的显示宽度
func getRuneWidth(r rune) int {
	// ASCII字符
	if r < 0x80 {
		return 1
	}

	// 常见的emoji和符号字符
	if isEmoji(r) {
		return 2 // 大多数emoji占2个字符宽度
	}

	// 中文、日文、韩文等宽字符
	if isWideChar(r) {
		return 2
	}

	// 其他Unicode字符
	return 1
}

// isEmoji 判断是否为emoji字符
func isEmoji(r rune) bool {
	// 使用unicode标准库检查emoji相关类别
	// 主要emoji范围：U+1F600 - U+1F64F (表情符号)
	// U+1F300 - U+1F5FF (杂项符号)
	// U+1F680 - U+1F6FF (交通符号)
	// 区域指示符号：U+1F1E6 - U+1F1FF
	return (r >= 0x1F600 && r <= 0x1F64F) ||
		(r >= 0x1F300 && r <= 0x1F5FF) ||
		(r >= 0x1F680 && r <= 0x1F6FF) ||
		(r >= 0x1F1E6 && r <= 0x1F1FF)
}

// isWideChar 判断是否为全角字符
func isWideChar(r rune) bool {
	// 核心CJK字符范围：统一汉字 + 扩展A + 全角符号
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK统一汉字
		(r >= 0x3400 && r <= 0x4DBF) || // CJK扩展A
		(r >= 0x3000 && r <= 0x303F) || // CJK符号
		(r >= 0xFF00 && r <= 0xFFEF) // 全角ASCII
}

// 高级表格打印函数
func printAdvancedTable(headers []string, rows [][]string, color string) {
	if len(headers) == 0 || len(rows) == 0 {
		return
	}

	// 计算每列的最大宽度
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header) + 2 // 加上边距
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				cellLen := len(removeColorCodes(cell)) + 2
				if cellLen > colWidths[i] {
					colWidths[i] = cellLen
				}
			}
		}
	}

	// 打印顶部边框
	fmt.Printf("\n%s%s", color, BoxTopLeft)
	for i, width := range colWidths {
		fmt.Printf("%s", strings.Repeat(BoxHorizontal, width))
		if i < len(colWidths)-1 {
			fmt.Printf("%s", BoxTeeDown)
		}
	}
	fmt.Printf("%s%s\n", BoxTopRight, ColorReset)

	// 打印表头
	fmt.Printf("%s%s", color, BoxVertical)
	for i, header := range headers {
		fmt.Printf(" %s%-*s", ColorBold+ColorWhite, colWidths[i]-2, header)
		fmt.Printf("%s%s", ColorReset+color, BoxVertical)
	}
	fmt.Printf("%s\n", ColorReset)

	// 打印分隔线
	fmt.Printf("%s%s", color, BoxTeeRight)
	for i, width := range colWidths {
		fmt.Printf("%s", strings.Repeat(BoxHorizontal, width))
		if i < len(colWidths)-1 {
			fmt.Printf("%s", BoxCross)
		}
	}
	fmt.Printf("%s%s\n", BoxTeeLeft, ColorReset)

	// 打印数据行
	for _, row := range rows {
		fmt.Printf("%s%s", color, BoxVertical)
		for i, cell := range row {
			if i < len(colWidths) {
				removeColorCodes(cell)
				fmt.Printf(" %-*s", colWidths[i]-2, cell)
				fmt.Printf("%s%s", color, BoxVertical)
			}
		}
		fmt.Printf("%s\n", ColorReset)
	}

	// 打印底部边框
	fmt.Printf("%s%s", color, BoxBottomLeft)
	for i, width := range colWidths {
		fmt.Printf("%s", strings.Repeat(BoxHorizontal, width))
		if i < len(colWidths)-1 {
			fmt.Printf("%s", BoxTeeUp)
		}
	}
	fmt.Printf("%s%s\n", BoxBottomRight, ColorReset)
}

// 移除颜色代码以计算实际长度
func removeColorCodes(s string) string {
	result := s
	colorCodes := []string{ColorReset, ColorRed, ColorGreen, ColorYellow,
		ColorBlue, ColorPurple, ColorCyan, ColorWhite, ColorBold, ColorDim}

	for _, code := range colorCodes {
		result = strings.ReplaceAll(result, code, "")
	}
	return result
}

// getCompressionName 获取压缩类型名称
func getCompressionName(compression CompressionType) string {
	switch compression {
	case NoCompression:
		return "无压缩"
	case SnappyCompression:
		return "Snappy"
	case ZSTDCompression:
		return "ZSTD"
	case GzipCompression:
		return "Gzip"
	default:
		return "未知"
	}
}

// getBoolString 获取布尔值字符串
func getBoolString(b bool) string {
	if b {
		return "✅ 启用"
	}
	return "❌ 禁用"
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f秒", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1f分钟", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f小时", d.Hours())
	} else {
		return fmt.Sprintf("%.1f天", d.Hours()/24)
	}
}

// PrintDatabaseOverview 打印数据库概览
func (db *GojiDB) PrintDatabaseOverview() {
	keys := db.ListKeys()
	totalKeys := len(keys)

	// 统计过期键
	expiredKeys := 0
	deletedKeys := 0
	now := uint32(time.Now().Unix())

	db.mu.RLock()
	db.keyDir.Range(func(key string, keyDir *KeyDir) bool {
		if keyDir.Deleted {
			deletedKeys++
		} else if keyDir.TTL > 0 && now > keyDir.TTL {
			expiredKeys++
		}
		return true
	})
	totalStoredKeys := db.keyDir.Size()
	db.mu.RUnlock()

	// 计算数据文件大小
	var totalSize int64
	files, err := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	if err == nil {
		for _, file := range files {
			if stat, err := os.Stat(file); err == nil {
				totalSize += stat.Size()
			}
		}
	}

	// 计算运行时间
	uptime := time.Since(db.metrics.StartTime)

	content := []string{
		fmt.Sprintf("🗂️  数据目录:      %s%s%s", ColorCyan, db.config.DataPath, ColorReset),
		fmt.Sprintf("📁  活跃文件ID:    %s%d%s", ColorYellow, db.activeFileID, ColorReset),
		fmt.Sprintf("🔑  活跃键数量:    %s%d%s", ColorGreen, totalKeys, ColorReset),
		fmt.Sprintf("📦  总存储键数:    %s%d%s", ColorBlue, totalStoredKeys, ColorReset),
		fmt.Sprintf("🗑️  已删除键数:    %s%d%s", ColorRed, deletedKeys, ColorReset),
		fmt.Sprintf("⏰  已过期键数:    %s%d%s", ColorYellow, expiredKeys, ColorReset),
		fmt.Sprintf("💾  数据文件大小:  %s%s%s", ColorPurple, formatSize(totalSize), ColorReset),
		fmt.Sprintf("🗜️  压缩类型:      %s%s%s", ColorGreen, db.getCurrentCompressionType(), ColorReset),
		fmt.Sprintf("📊  监控状态:      %s%s%s", ColorCyan, getBoolString(db.config.EnableMetrics), ColorReset),
		fmt.Sprintf("⏱️  运行时间:      %s%s%s", ColorBlue, formatDuration(uptime), ColorReset),
	}

	printFancyBox("📊 数据库状态概览", content, ColorGreen, 70)
}

// PrintDataVisualization 打印数据可视化
func (db *GojiDB) PrintDataVisualization() {
	keys := db.ListKeys()
	if len(keys) == 0 {
		fmt.Printf("\n%s📊 数据可视化%s\n", ColorBold+ColorBlue, ColorReset)
		fmt.Printf("%s数据库为空，没有数据可以显示%s\n", ColorYellow, ColorReset)
		return
	}

	fmt.Printf("\n%s%s 数据可视化 %s%s\n", ColorBold+ColorBlue,
		strings.Repeat("═", 30), strings.Repeat("═", 30), ColorReset)

	// 按键类型分组统计
	keyTypes := make(map[string]int)
	keyTypeSizes := make(map[string]int64)

	db.mu.RLock()
	for _, key := range keys {
		if keyDir, exists := db.keyDir.Get(key); exists && !keyDir.Deleted {
			// 根据键的前缀分类
			parts := strings.Split(key, ":")
			keyType := "其他"
			if len(parts) > 1 {
				keyType = parts[0]
			}

			keyTypes[keyType]++
			keyTypeSizes[keyType] += int64(keyDir.ValueSize)
		}
	}
	db.mu.RUnlock()

	// 显示键类型分布
	fmt.Printf("\n%s🔑 键类型分布:%s\n", ColorBold+ColorGreen, ColorReset)

	maxCount := 0
	for _, count := range keyTypes {
		if count > maxCount {
			maxCount = count
		}
	}

	// 防止除以零错误
	if maxCount == 0 {
		fmt.Printf("  %s没有有效的键类型数据%s\n", ColorYellow, ColorReset)
		return
	}

	for keyType, count := range keyTypes {
		barLength := (count * 40) / maxCount
		if barLength == 0 && count > 0 {
			barLength = 1
		}

		bar := strings.Repeat("█", barLength) + strings.Repeat("░", 40-barLength)
		percentage := float64(count) / float64(len(keys)) * 100

		fmt.Printf("  %-12s %s%s%s %s%3d%s (%s%.1f%%%s)\n",
			keyType, ColorCyan, bar, ColorReset,
			ColorYellow, count, ColorReset,
			ColorGreen, percentage, ColorReset)
	}

	// 显示数据大小分布
	fmt.Printf("\n%s💾 数据大小分布:%s\n", ColorBold+ColorPurple, ColorReset)

	maxSize := int64(0)
	for _, size := range keyTypeSizes {
		if size > maxSize {
			maxSize = size
		}
	}

	// 防止除以零错误
	if maxSize == 0 {
		fmt.Printf("  %s没有有效的数据大小信息%s\n", ColorYellow, ColorReset)
		return
	}

	for keyType, size := range keyTypeSizes {
		barLength := int((size * 40) / maxSize)
		if barLength == 0 && size > 0 {
			barLength = 1
		}

		bar := strings.Repeat("█", barLength) + strings.Repeat("░", 40-barLength)

		fmt.Printf("  %-12s %s%s%s %s%8s%s\n",
			keyType, ColorPurple, bar, ColorReset,
			ColorYellow, formatSize(size), ColorReset)
	}
}

// PrintFullDatabaseStatus 打印完整数据库状态
func (db *GojiDB) PrintFullDatabaseStatus() {
	// 显示数据库概览
	db.PrintDatabaseOverview()

	// 显示数据可视化
	db.PrintDataVisualization()

	// 文件信息
	files, err := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	if err == nil && len(files) > 0 {
		fmt.Printf("\n%s📁 数据文件详情:%s\n", ColorBold+ColorBlue, ColorReset)

		headers := []string{"文件ID", "文件大小", "修改时间", "状态"}
		var rows [][]string

		sort.Strings(files)
		for _, file := range files {
			if stat, err := os.Stat(file); err == nil {
				fileID, _ := db.parseFileID(file)
				status := "只读"
				statusColor := ColorDim
				if fileID == db.activeFileID {
					status = "活跃"
					statusColor = ColorGreen
				}

				rows = append(rows, []string{
					fmt.Sprintf("%s%d%s", ColorYellow, fileID, ColorReset),
					formatSize(stat.Size()),
					stat.ModTime().Format("01-02 15:04"),
					fmt.Sprintf("%s%s%s", statusColor, status, ColorReset),
				})
			}
		}

		printAdvancedTable(headers, rows, ColorBlue)
	}

	// 显示性能指标
	if db.config.EnableMetrics {
		db.PrintMetrics()
	}
}

// PrintMetrics 打印性能指标
func (db *GojiDB) PrintMetrics() {
	metrics := db.GetMetrics()
	uptime := time.Since(metrics.StartTime)

	content := []string{
		fmt.Sprintf("📖 总读取次数:     %s%s%s", ColorGreen, formatNumber(metrics.TotalReads), ColorReset),
		fmt.Sprintf("📝 总写入次数:     %s%s%s", ColorBlue, formatNumber(metrics.TotalWrites), ColorReset),
		fmt.Sprintf("🗑️ 总删除次数:     %s%s%s", ColorRed, formatNumber(metrics.TotalDeletes), ColorReset),
		fmt.Sprintf("🎯 缓存命中:       %s%s%s", ColorGreen, formatNumber(metrics.CacheHits), ColorReset),
		fmt.Sprintf("❌ 缓存未命中:     %s%s%s", ColorYellow, formatNumber(metrics.CacheMisses), ColorReset),
		fmt.Sprintf("🔄 合并次数:       %s%s%s", ColorPurple, formatNumber(metrics.CompactionCount), ColorReset),
		fmt.Sprintf("⏰ 过期键清理:     %s%s%s", ColorCyan, formatNumber(metrics.ExpiredKeyCount), ColorReset),
		fmt.Sprintf("🗜️ 压缩比:         %s%.4f%s", ColorBlue, float64(metrics.CompressionRatio)/100.0, ColorReset),
		fmt.Sprintf("⏱️ 运行时间:       %s%s%s", ColorPurple, formatDuration(uptime), ColorReset),
	}

	// 计算缓存命中率
	totalAccess := metrics.CacheHits + metrics.CacheMisses
	if totalAccess > 0 {
		hitRate := float64(metrics.CacheHits) / float64(totalAccess) * 100
		content = append(content, fmt.Sprintf("📊 缓存命中率:     %s%.2f%%%s", ColorGreen, hitRate, ColorReset))
	}

	// 计算QPS
	if uptime.Seconds() > 0 {
		totalOps := metrics.TotalReads + metrics.TotalWrites + metrics.TotalDeletes
		qps := float64(totalOps) / uptime.Seconds()
		content = append(content, fmt.Sprintf("⚡ 平均QPS:        %s%.2f%s", ColorYellow, qps, ColorReset))
	}

	printFancyBox("📊 性能指标", content, ColorPurple, 70)
}

// getCurrentCompressionType 获取当前实际使用的压缩类型
func (db *GojiDB) getCurrentCompressionType() string {
	if db.smartCompressor != nil {
		// 获取智能压缩器的统计信息
		stats := db.smartCompressor.GetStats()
		return db.getCompressionNameFromString(stats.Algorithm)
	}
	// 如果没有智能压缩器，使用配置中的类型
	return getCompressionName(db.config.CompressionType)
}

// getCompressionNameFromString 从字符串获取压缩类型名称
func (db *GojiDB) getCompressionNameFromString(algorithm string) string {
	switch algorithm {
	case "auto":
		return "智能压缩"
	case "snappy":
		return "Snappy"
	case "zstd":
		return "ZSTD"
	case "gzip":
		return "Gzip"
	case "":
		return getCompressionName(db.config.CompressionType)
	default:
		return algorithm
	}
}

// formatNumber 格式化数字显示
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else if n < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else {
		return fmt.Sprintf("%.1fB", float64(n)/1000000000)
	}
}

func (db *GojiDB) GetMetrics() *Metrics {
	if !db.config.EnableMetrics {
		return nil
	}

	keyCount := int64(db.keyDir.Size())

	// 计算数据总大小
	var dataSize int64
	files, _ := filepath.Glob(filepath.Join(db.config.DataPath, "*.data"))
	for _, file := range files {
		if stat, err := os.Stat(file); err == nil {
			dataSize += stat.Size()
		}
	}

	return &Metrics{
		TotalWrites:      atomic.LoadInt64(&db.metrics.TotalWrites),
		TotalReads:       atomic.LoadInt64(&db.metrics.TotalReads),
		TotalDeletes:     atomic.LoadInt64(&db.metrics.TotalDeletes),
		KeyCount:         keyCount,
		DataSize:         dataSize,
		CompressionRatio: atomic.LoadInt64(&db.metrics.CompressionRatio),
		CacheHits:        atomic.LoadInt64(&db.metrics.CacheHits),
		CacheMisses:      atomic.LoadInt64(&db.metrics.CacheMisses),
		ExpiredKeyCount:  atomic.LoadInt64(&db.metrics.ExpiredKeyCount),
		CompactionCount:  atomic.LoadInt64(&db.metrics.CompactionCount),
		StartTime:        db.metrics.StartTime,
		SystemInfo:       db.metrics.SystemInfo,
	}
}
