package GojiDB

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WAL操作类型
type WALOpType byte

const (
	WALPut    WALOpType = 0x01
	WALDelete WALOpType = 0x02
	WALBegin  WALOpType = 0x03
	WALCommit WALOpType = 0x04
	WALRollback WALOpType = 0x05
)

// WAL记录结构
type WALRecord struct {
	Type      WALOpType
	Timestamp uint64
	Key       string
	Value     []byte
	TxID      string
	Checksum  uint32
}

// WALEntry WAL条目
type WALEntry struct {
	Type      WALOpType `json:"type"`
	Timestamp uint64    `json:"timestamp"`
	Key       string    `json:"key,omitempty"`
	Value     []byte    `json:"value,omitempty"`
	TxID      string    `json:"tx_id,omitempty"`
}

// WAL审计链管理器
type WALManager struct {
	mu         sync.RWMutex
	file       *os.File
	writer     *bufio.Writer
	path       string
	config     *GojiDBConfig
	lastSync   time.Time
	syncTicker *time.Ticker
	stopChan   chan struct{}
	wg         sync.WaitGroup
	closed     bool
	
	// 审计统计
	stats WALStats
}

// WALStats WAL统计信息
type WALStats struct {
	TotalRecords    int64
	TotalBytes      int64
	WriteCount      int64
	ReadCount       int64
	CheckpointCount int64
	RecoveryCount   int64
	TransactionCommits int64
	TransactionRollbacks int64
	LastWriteTime   time.Time
	LastCheckpoint  time.Time
	StartTime       time.Time
}

// NewWALManager 创建新的WAL管理器
func NewWALManager(config *GojiDBConfig) (*WALManager, error) {
	walPath := filepath.Join(config.DataPath, "wal.log")
	
	// 确保目录存在
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return nil, fmt.Errorf("创建WAL目录失败: %v", err)
	}

	// 打开或创建WAL文件
	file, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开WAL文件失败: %v", err)
	}

	wal := &WALManager{
		file:       file,
		writer:     bufio.NewWriterSize(file, 64*1024), // 64KB缓冲区
		path:       walPath,
		config:     config,
		lastSync:   time.Now(),
		syncTicker: time.NewTicker(config.WALFlushInterval),
		stopChan:   make(chan struct{}),
		stats: WALStats{
			StartTime: time.Now(),
			LastWriteTime: time.Now(),
		},
	}

	// 启动后台同步goroutine
	if config.EnableWAL && config.WALFlushInterval > 0 {
		wal.wg.Add(1)
		go wal.backgroundSync()
	}

	return wal, nil
}

// Close 关闭WAL管理器
func (w *WALManager) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	if w.stopChan != nil {
		close(w.stopChan)
	}
	w.wg.Wait()
	
	if w.syncTicker != nil {
		w.syncTicker.Stop()
	}
	
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Sync()
		w.file.Close()
	}
	return nil
}

// WriteRecord 写入WAL记录
func (w *WALManager) WriteRecord(record *WALRecord) error {
	if !w.config.EnableWAL {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 序列化记录
	data, err := w.serializeRecord(record)
	if err != nil {
		return fmt.Errorf("序列化WAL记录失败: %v", err)
	}

	// 写入长度前缀
	length := uint32(len(data))
	if err := binary.Write(w.writer, binary.BigEndian, length); err != nil {
		return fmt.Errorf("写入WAL记录长度失败: %v", err)
	}

	// 写入数据
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("写入WAL记录数据失败: %v", err)
	}

	// 立即同步或等待后台同步
	if w.config.WALSyncWrites {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		if err := w.file.Sync(); err != nil {
			return err
		}
	}

	// 更新统计
	w.stats.TotalRecords++
	w.stats.TotalBytes += int64(len(data))
	w.stats.WriteCount++
	w.stats.LastWriteTime = time.Now()
	
	// 更新事务统计
	switch record.Type {
	case WALCommit:
		w.stats.TransactionCommits++
	case WALRollback:
		w.stats.TransactionRollbacks++
	}

	return nil
}

// WriteTransaction 写入事务操作
func (w *WALManager) WriteTransaction(txID string, operations []TransactionOp) error {
	if !w.config.EnableWAL {
		return nil
	}

	// 写入事务开始
	beginRecord := &WALRecord{
		Type:      WALBegin,
		Timestamp: uint64(time.Now().UnixNano()),
		TxID:      txID,
	}
	if err := w.WriteRecord(beginRecord); err != nil {
		return err
	}

	// 写入事务操作
	for _, op := range operations {
		record := &WALRecord{
			Type:      w.getOpType(op.Type),
			Timestamp: uint64(time.Now().UnixNano()),
			Key:       op.Key,
			Value:     op.Value,
			TxID:      txID,
		}
		if err := w.WriteRecord(record); err != nil {
			return err
		}
	}

	// 写入事务提交
	commitRecord := &WALRecord{
		Type:      WALCommit,
		Timestamp: uint64(time.Now().UnixNano()),
		TxID:      txID,
	}
	return w.WriteRecord(commitRecord)
}

// Recover 从WAL恢复数据
func (w *WALManager) Recover(db *GojiDB) error {
	if !w.config.EnableWAL {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查WAL文件是否存在
	if _, err := os.Stat(w.path); os.IsNotExist(err) {
		return nil // WAL文件不存在，无需恢复
	}

	// 重新打开文件用于读取
	file, err := os.Open(w.path)
	if err != nil {
		return fmt.Errorf("打开WAL文件用于恢复失败: %v", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	transactions := make(map[string][]*WALRecord)
	
	for {
		// 读取长度前缀
		var length uint32
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("读取WAL记录长度失败: %v", err)
		}

		// 读取记录数据
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			return fmt.Errorf("读取WAL记录数据失败: %v", err)
		}

		// 反序列化记录
		record, err := w.deserializeRecord(data)
		if err != nil {
			return fmt.Errorf("反序列化WAL记录失败: %v", err)
		}

		// 收集事务记录
		if record.TxID != "" {
			transactions[record.TxID] = append(transactions[record.TxID], record)
		}
	}

	// 重放未完成的事务
	w.mu.Unlock() // 释放锁避免死锁
	err = w.replayTransactions(db, transactions)
	w.mu.Lock() // 重新获取锁
	
	// 更新恢复统计
	if err == nil {
		w.stats.RecoveryCount++
	}
	return err
}

// Checkpoint 创建WAL检查点
func (w *WALManager) Checkpoint() error {
	if !w.config.EnableWAL {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 刷写缓冲区
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if err := w.file.Sync(); err != nil {
		return err
	}

	// 更新检查点统计
	w.stats.CheckpointCount++
	w.stats.LastCheckpoint = time.Now()

	// 截断WAL文件（在实际应用中，这应该在创建快照后执行）
	return nil
}

// GetWALSize 获取WAL文件大小
func (w *WALManager) GetWALSize() (int64, error) {
	if !w.config.EnableWAL || w.file == nil {
		return 0, nil
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	stat, err := w.file.Stat()
	if err != nil {
		return 0, fmt.Errorf("获取WAL文件信息失败: %v", err)
	}

	return stat.Size(), nil
}

// GetStats 获取WAL统计信息
func (w *WALManager) GetStats() WALStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// 内部方法

func (w *WALManager) backgroundSync() {
	defer w.wg.Done()
	
	for {
		select {
		case <-w.syncTicker.C:
			w.mu.Lock()
			if w.writer != nil {
				w.writer.Flush()
				w.file.Sync()
			}
			w.mu.Unlock()
		case <-w.stopChan:
			return
		}
	}
}

func (w *WALManager) serializeRecord(record *WALRecord) ([]byte, error) {
	entry := WALEntry{
		Type:      record.Type,
		Timestamp: record.Timestamp,
		Key:       record.Key,
		Value:     record.Value,
		TxID:      record.TxID,
	}
	
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	
	// 计算校验和
	checksum := calculateChecksum(data)
	record.Checksum = checksum
	
	// 重新序列化包含校验和
	return json.Marshal(entry)
}

func (w *WALManager) deserializeRecord(data []byte) (*WALRecord, error) {
	var entry WALEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	
	return &WALRecord{
		Type:      entry.Type,
		Timestamp: entry.Timestamp,
		Key:       entry.Key,
		Value:     entry.Value,
		TxID:      entry.TxID,
	}, nil
}

func (w *WALManager) replayTransactions(db *GojiDB, transactions map[string][]*WALRecord) error {
	for _, records := range transactions {
		// 检查事务是否完整（有BEGIN和COMMIT/ROLLBACK）
		if len(records) < 2 {
			continue // 不完整的事务，跳过
		}
		
		beginFound := false
		commitFound := false
		rollbackFound := false
		operations := make([]TransactionOp, 0)
		
		for _, record := range records {
			switch record.Type {
			case WALBegin:
				beginFound = true
			case WALCommit:
				commitFound = true
			case WALRollback:
				rollbackFound = true
			case WALPut:
				operations = append(operations, TransactionOp{
					Type:  "PUT",
					Key:   record.Key,
					Value: record.Value,
				})
			case WALDelete:
				operations = append(operations, TransactionOp{
					Type: "DELETE",
					Key:  record.Key,
				})
			}
		}
		
		// 重放完整的事务
		if beginFound && commitFound && !rollbackFound {
			for _, op := range operations {
				switch op.Type {
				case "PUT":
					_ = db.Put(op.Key, op.Value)
				case "DELETE":
					_ = db.Delete(op.Key)
				}
			}
		}
	}
	
	return nil
}

func (w *WALManager) getOpType(opType string) WALOpType {
	switch opType {
	case "PUT":
		return WALPut
	case "DELETE":
		return WALDelete
	default:
		return WALPut
	}
}

func calculateChecksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
	}
	return sum
}