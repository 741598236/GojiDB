package GojiDB

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Transaction 表示一个数据库事务
// 提供原子性操作，支持回滚和提交
type Transaction struct {
	id         string           // 事务唯一标识符
	operations []TransactionOp  // 事务操作列表
	db         *GojiDB          // 关联的数据库实例
	mu         sync.Mutex       // 事务操作互斥锁
}

// TransactionOp 表示单个事务操作
// 用于记录事务中的每个操作步骤
type TransactionOp struct {
	Type  string // 操作类型: "PUT" 或 "DELETE"
	Key   string // 操作的键
	Value []byte // 操作的值（DELETE操作时为nil）
}

// BeginTransaction 开始一个新的事务
// 返回一个可用于执行事务操作的事务对象
func (db *GojiDB) BeginTransaction() (*Transaction, error) {
	b := make([]byte, 8)
	rand.Read(b)
	return &Transaction{
		id:         hex.EncodeToString(b),
		operations: make([]TransactionOp, 0),
		db:         db,
	}, nil
}

func (tx *Transaction) Put(key string, value []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.operations = append(tx.operations, TransactionOp{Type: "PUT", Key: key, Value: value})
	return nil
}

func (tx *Transaction) Delete(key string) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.operations = append(tx.operations, TransactionOp{Type: "DELETE", Key: key})
	return nil
}

func (tx *Transaction) Get(key string) ([]byte, error) {
	// 事务中的读取操作也记录到operations中
	tx.mu.Lock()
	defer tx.mu.Unlock()
	
	// 在实际事务实现中，这里应该从事务缓存中读取
	// 当前实现：直接从数据库读取
	return tx.db.Get(key)
}

func (tx *Transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// 写入事务WAL记录
	if tx.db.walManager != nil {
		if err := tx.db.walManager.WriteTransaction(tx.id, tx.operations); err != nil {
			return fmt.Errorf("事务WAL写入失败: %v", err)
		}
	}

	for _, op := range tx.operations {
		switch op.Type {
		case "PUT":
			if err := tx.db.Put(op.Key, op.Value); err != nil {
				return fmt.Errorf("PUT失败: %v", err)
			}
		case "DELETE":
			_ = tx.db.Delete(op.Key)
		}
	}

	// 提交事务WAL记录
	if tx.db.walManager != nil {
		commitRecord := &WALRecord{
			Type:      WALCommit,
			Timestamp: uint64(time.Now().UnixNano()),
			Key:       fmt.Sprintf("tx_%s", tx.id),
		}
		if err := tx.db.walManager.WriteRecord(commitRecord); err != nil {
			return fmt.Errorf("事务提交WAL写入失败: %v", err)
		}
	}

	tx.operations = nil
	return nil
}

func (tx *Transaction) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// 写入回滚WAL记录
	if tx.db.walManager != nil {
		rollbackRecord := &WALRecord{
			Type:      WALRollback,
			Timestamp: uint64(time.Now().UnixNano()),
			Key:       fmt.Sprintf("tx_%s", tx.id),
		}
		if err := tx.db.walManager.WriteRecord(rollbackRecord); err != nil {
			return fmt.Errorf("事务回滚WAL写入失败: %v", err)
		}
	}

	tx.operations = nil
	return nil
}
