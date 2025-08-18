package benchmark

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	gojidb "GojiDB"
)

// 综合性能基准测试套件

// ==================== 基础操作测试 ====================

// BenchmarkDelete 基准测试删除操作性能
func BenchmarkDelete(b *testing.B) {
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
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("delete_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("delete_%d", i)
		if err := db.Delete(key); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExists 基准测试键存在性检查
func BenchmarkExists(b *testing.B) {
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
		key := fmt.Sprintf("exists_%d", i)
		value := []byte("value")
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("exists_%d", i%1000)
		_, err := db.Get(key)
		if err != nil && err.Error() != "键不存在" {
			b.Fatal(err)
		}
	}
}

// ==================== 批量操作测试 ====================

// BenchmarkBatchDelete 基准测试批量删除
func BenchmarkBatchDelete(b *testing.B) {
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
	for i := 0; i < b.N; i++ {
		// 预填充数据
		items := make(map[string][]byte)
		keys := make([]string, 0, batchSize)
		for j := 0; j < batchSize; j++ {
			key := fmt.Sprintf("batch_del_%d_%d", i, j)
			items[key] = []byte("value")
			keys = append(keys, key)
		}
		if err := db.BatchPut(items); err != nil {
			b.Fatal(err)
		}

		// 批量删除
		b.StartTimer()
		for _, key := range keys {
			if err := db.Delete(key); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
	}
}

// ==================== TTL相关测试 ====================

// BenchmarkSetTTL 基准测试设置TTL
func BenchmarkSetTTL(b *testing.B) {
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
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("ttl_set_%d", i)
		value := []byte("value")
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("ttl_set_%d", i)
		if err := db.SetTTL(key, time.Minute); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 内存和配置测试 ====================

// BenchmarkMemoryUsage 基准测试内存使用情况
func BenchmarkMemoryUsage(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 64 * 1024 * 1024, // 64MB
		SyncWrites:  false,
		CacheConfig: gojidb.CacheConfig{
			BlockCacheSize: 1024,
			KeyCacheSize:   8192,
			EnableCache:    true,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("mem_%d", i)
		value := make([]byte, 1024) // 1KB
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNoCache 基准测试无缓存配置
func BenchmarkNoCache(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
		CacheConfig: gojidb.CacheConfig{
			EnableCache: false,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("no_cache_%d", i)
		value := []byte("value")
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("no_cache_%d", i%1000)
		_, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 高并发测试 ====================

// BenchmarkHighConcurrentPut 基准测试高并发写入
func BenchmarkHighConcurrentPut(b *testing.B) {
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
			key := fmt.Sprintf("high_concurrent_%d", i)
			value := []byte(fmt.Sprintf("value_%d", i))
			if err := db.Put(key, value); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkConcurrentMixedHighLoad 基准测试高负载并发混合操作
func BenchmarkConcurrentMixedHighLoad(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 64 * 1024 * 1024,
		SyncWrites:  false,
		CacheConfig: gojidb.CacheConfig{
			BlockCacheSize: 1024,
			KeyCacheSize:   4096,
			EnableCache:    true,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充数据
	const preloadCount = 10000
	for i := 0; i < preloadCount; i++ {
		key := fmt.Sprintf("preload_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rand.Seed(time.Now().UnixNano())
		for pb.Next() {
			op := rand.Intn(100)
			keyNum := rand.Intn(preloadCount * 2)
			key := fmt.Sprintf("key_%d", keyNum)

			switch {
			case op < 45: // 45% 写入
				value := []byte(fmt.Sprintf("value_%d", keyNum))
				_ = db.Put(key, value)
			case op < 80: // 35% 读取
				_, _ = db.Get(key)
			case op < 90: // 10% 删除
				_ = db.Delete(key)
			default: // 10% TTL设置
				_ = db.PutWithTTL(key, []byte("ttl_value"), time.Minute)
			}
		}
	})
}

// ==================== 数据完整性测试 ====================

// BenchmarkDataIntegrity 基准测试数据完整性验证
func BenchmarkDataIntegrity(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  true, // 强制同步写入确保数据完整性
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 测试数据完整性
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("integrity_%d", i)
		original := []byte(fmt.Sprintf("original_data_%d", i))

		// 写入
		if err := db.Put(key, original); err != nil {
			b.Fatal(err)
		}

		// 读取验证
		retrieved, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}

		if string(retrieved) != string(original) {
			b.Fatalf("数据完整性检查失败: 期望 %q, 实际 %q", string(original), string(retrieved))
		}
	}
}

// ==================== 性能场景测试 ====================

// BenchmarkRealWorldScenario 基准测试真实世界场景
func BenchmarkRealWorldScenario(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 32 * 1024 * 1024, // 32MB
		SyncWrites:  false,
		CacheConfig: gojidb.CacheConfig{
			BlockCacheSize: 256,
			KeyCacheSize:   1024,
			EnableCache:    true,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 模拟真实场景：80%读取，15%写入，5%删除
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rand.Seed(time.Now().UnixNano())
		counter := 0

		for pb.Next() {
			counter++

			// 使用更真实的键名
			keys := []string{
				fmt.Sprintf("user:%d:profile", counter),
				fmt.Sprintf("session:%d", counter),
				fmt.Sprintf("cache:api:%d", counter),
				fmt.Sprintf("config:feature:%d", counter),
			}

			key := keys[rand.Intn(len(keys))]

			switch op := rand.Intn(100); {
			case op < 80: // 80% 读取
				_, _ = db.Get(key)
			case op < 95: // 15% 写入
				value := []byte(fmt.Sprintf(`{"data":"value_%d","timestamp":%d}`, counter, time.Now().Unix()))
				_ = db.Put(key, value)
			default: // 5% 删除
				_ = db.Delete(key)
			}
		}
	})
}

// ==================== 极端场景测试 ====================

// BenchmarkExtremeValues 基准测试极端值大小
func BenchmarkExtremeValues(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 128 * 1024 * 1024, // 128MB
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	valueSizes := []int{1, 16, 256, 1024, 4096, 16384, 65536, 262144} // 1B 到 256KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("extreme_%d", i)
		size := valueSizes[i%len(valueSizes)]

		value := make([]byte, size)
		for j := range value {
			value[j] = byte(j % 256)
		}

		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEmptyOperations 基准测试空操作和边界条件
func BenchmarkEmptyOperations(b *testing.B) {
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
		// 测试空值
		key := fmt.Sprintf("empty_%d", i)
		if err := db.Put(key, []byte{}); err != nil {
			b.Fatal(err)
		}

		// 测试空键（预期会失败但不影响基准测试）
		_ = db.Put("", []byte("empty_key"))

		// 测试特殊字符键
		specialKey := fmt.Sprintf("special!@#$%%^&*()_%d", i)
		if err := db.Put(specialKey, []byte("special_value")); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 持久化测试 ====================

// BenchmarkPersistence 基准测试数据持久化性能
func BenchmarkPersistence(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 64 * 1024 * 1024,
		SyncWrites:  true, // 强制同步
		EnableWAL:   true, // 启用WAL
		WALFlushInterval: 1 * time.Second,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("persist_%d", i)
		value := []byte(fmt.Sprintf("persistent_value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 恢复测试 ====================

// BenchmarkDBRestart 基准测试数据库重启恢复
func BenchmarkDBRestart(b *testing.B) {
	tempDir := b.TempDir()

	// 预填充数据
	{
		db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
			DataPath:    tempDir,
			MaxFileSize: 1024 * 1024,
			SyncWrites:  true,
		})
		if err != nil {
			b.Fatal(err)
		}

		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("restart_%d", i)
			value := []byte(fmt.Sprintf("value_%d", i))
			if err := db.Put(key, value); err != nil {
				b.Fatal(err)
			}
		}

		db.Close()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
			DataPath:    tempDir,
			MaxFileSize: 1024 * 1024,
			SyncWrites:  false,
		})
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		// 验证数据完整性
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("restart_%d", j)
			_, _ = db.Get(key)
		}

		b.StopTimer()
		db.Close()
	}
}
