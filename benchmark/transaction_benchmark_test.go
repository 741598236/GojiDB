package benchmark

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	gojidb "GojiDB"
)

// 事务和并发控制基准测试套件

// ==================== 事务操作测试 ====================

// BenchmarkTransactionPut 基准测试事务写入性能
func BenchmarkTransactionPut(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := db.BeginTransaction()
		if err != nil {
			b.Fatal(err)
		}
		key := fmt.Sprintf("tx_put_%d", i)
		value := []byte(fmt.Sprintf("transaction_value_%d", i))
		
		if err := tx.Put(key, value); err != nil {
			tx.Rollback()
			b.Fatal(err)
		}
		
		if err := tx.Commit(); err != nil {
			tx.Rollback()
			b.Fatal(err)
		}
	}
}

// BenchmarkTransactionBatch 基准测试事务批量操作
func BenchmarkTransactionBatch(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	batchSize := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := db.BeginTransaction()
		if err != nil {
			b.Fatal(err)
		}
		
		for j := 0; j < batchSize; j++ {
			key := fmt.Sprintf("tx_batch_%d_%d", i, j)
			value := []byte(fmt.Sprintf("batch_value_%d_%d", i, j))
			
			if err := tx.Put(key, value); err != nil {
				tx.Rollback()
				b.Fatal(err)
			}
		}
		
		if err := tx.Commit(); err != nil {
			tx.Rollback()
			b.Fatal(err)
		}
	}
}

// BenchmarkTransactionRollback 基准测试事务回滚性能
func BenchmarkTransactionRollback(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := db.BeginTransaction()
		if err != nil {
			b.Fatal(err)
		}
		
		for j := 0; j < 10; j++ {
			key := fmt.Sprintf("rollback_%d_%d", i, j)
			value := []byte("temp_value")
			_ = tx.Put(key, value)
		}
		
		tx.Rollback()
	}
}

// ==================== 并发事务测试 ====================

// BenchmarkConcurrentTransactions 基准测试并发事务处理
func BenchmarkConcurrentTransactions(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tx, err := db.BeginTransaction()
			if err != nil {
				b.Fatal(err)
			}
			
			key := fmt.Sprintf("concurrent_tx_%d", i)
			value := []byte(fmt.Sprintf("concurrent_value_%d", i))
			
			if err := tx.Put(key, value); err != nil {
				tx.Rollback()
				b.Fatal(err)
			}
			
			if err := tx.Commit(); err != nil {
				tx.Rollback()
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkTransactionIsolation 基准测试事务隔离性
func BenchmarkTransactionIsolation(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充共享数据
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("shared_%d", i)
		value := []byte("initial_value")
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		
		// 并发读写相同数据
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				
				tx, err := db.BeginTransaction()
			if err != nil {
				b.Fatal(err)
			}
			
			// 读取
			key := fmt.Sprintf("shared_%d", worker%100)
			_, _ = tx.Get(key)
			
			// 写入
			newValue := []byte(fmt.Sprintf("updated_by_%d", worker))
			_ = tx.Put(key, newValue)
			
			_ = tx.Commit()
			}(j)
		}
		
		wg.Wait()
	}
}

// ==================== 读写冲突测试 ====================

// BenchmarkReadWriteContention 基准测试读写冲突处理
func BenchmarkReadWriteContention(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("contention_%d", i)
		value := []byte("base_value")
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			// 读者和写者混合
			if id%3 == 0 {
				// 读者
				key := fmt.Sprintf("contention_%d", id%1000)
				_, _ = db.Get(key)
			} else {
				// 写者使用事务
				tx, err := db.BeginTransaction()
			if err != nil {
				b.Fatal(err)
			}
				key := fmt.Sprintf("contention_%d", id%1000)
				value := []byte(fmt.Sprintf("updated_%d", id))
				
				_ = tx.Put(key, value)
				_ = tx.Commit()
			}
			id++
		}
	})
}

// ==================== 批量并发测试 ====================

// BenchmarkConcurrentBatchOperations 基准测试并发批量操作
func BenchmarkConcurrentBatchOperations(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		workerID := 0
		for pb.Next() {
			// 批量写入
			batch := make(map[string][]byte)
			for i := 0; i < 50; i++ {
				key := fmt.Sprintf("batch_concurrent_%d_%d", workerID, i)
				value := []byte(fmt.Sprintf("batch_value_%d_%d", workerID, i))
				batch[key] = value
			}
			
			if err := db.BatchPut(batch); err != nil {
				b.Fatal(err)
			}
			
			// 批量读取
			keys := make([]string, 0, 50)
			for key := range batch {
				keys = append(keys, key)
			}
			_, _ = db.BatchGet(keys)
			
			workerID++
		}
	})
}

// ==================== 锁竞争测试 ====================

// BenchmarkLockContention 基准测试锁竞争场景
func BenchmarkLockContention(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 热点键值对
	hotKeys := []string{"hot_key_1", "hot_key_2", "hot_key_3", "hot_key_4", "hot_key_5"}
	for _, key := range hotKeys {
		if err := db.Put(key, []byte("initial")); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			key := hotKeys[id%len(hotKeys)]
			
			// 高并发更新相同键
			if id%2 == 0 {
				// 直接更新
				value := []byte(fmt.Sprintf("direct_%d", id))
				_ = db.Put(key, value)
			} else {
				// 事务更新
			tx, err := db.BeginTransaction()
			if err != nil {
				b.Fatal(err)
			}
				current, _ := tx.Get(key)
				newValue := []byte(fmt.Sprintf("tx_%d_%s", id, string(current)))
				_ = tx.Put(key, newValue)
				_ = tx.Commit()
			}
			id++
		}
	})
}

// ==================== TTL并发测试 ====================

// BenchmarkConcurrentTTL 基准测试并发TTL操作
func BenchmarkConcurrentTTL(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			key := fmt.Sprintf("ttl_concurrent_%d", id)
			
			// 带TTL的并发写入
			value := []byte(fmt.Sprintf("ttl_value_%d", id))
			duration := time.Duration(rand.Intn(60)+1) * time.Minute
			
			_ = db.PutWithTTL(key, value, duration)
			
			// TTL更新
			_ = db.SetTTL(key, duration+time.Minute)
			
			id++
		}
	})
}

// ==================== 性能对比测试 ====================

// BenchmarkTransactionVsDirect 基准测试事务与直接操作的性能对比
func BenchmarkTransactionVsDirect(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.Run("DirectOperations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("direct_%d_%d", i, j)
				value := []byte(fmt.Sprintf("value_%d_%d", i, j))
				_ = db.Put(key, value)
			}
		}
	})

	b.Run("TransactionOperations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tx, err := db.BeginTransaction()
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("tx_%d_%d", i, j)
				value := []byte(fmt.Sprintf("value_%d_%d", i, j))
				_ = tx.Put(key, value)
			}
			_ = tx.Commit()
		}
	})
}

// ==================== 压力测试 ====================

// BenchmarkStressTest 基准测试极端压力场景
func BenchmarkStressTest(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			// 混合所有操作类型
			switch id % 5 {
			case 0: // 直接写入
				key := fmt.Sprintf("stress_%d", id)
				value := []byte(fmt.Sprintf("stress_value_%d", id))
				_ = db.Put(key, value)
			case 1: // 事务写入
				tx, err := db.BeginTransaction()
			if err != nil {
				b.Fatal(err)
			}
				key := fmt.Sprintf("stress_tx_%d", id)
				value := []byte(fmt.Sprintf("stress_tx_value_%d", id))
				_ = tx.Put(key, value)
				_ = tx.Commit()
			case 2: // 读取
				key := fmt.Sprintf("stress_%d", id%1000)
				_, _ = db.Get(key)
			case 3: // 删除
				key := fmt.Sprintf("stress_%d", id%1000)
				_ = db.Delete(key)
			case 4: // TTL操作
				key := fmt.Sprintf("stress_ttl_%d", id)
				value := []byte(fmt.Sprintf("stress_ttl_value_%d", id))
				_ = db.PutWithTTL(key, value, time.Minute)
			}
			id++
		}
	})
}