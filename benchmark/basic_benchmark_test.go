package benchmark

import (
	"fmt"
	"testing"
	"time"

	"gojidb"
)

// BenchmarkPut 基准测试单个Put操作
func BenchmarkPut(b *testing.B) {
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
		key := fmt.Sprintf("key_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGet 基准测试单个Get操作
func BenchmarkGet(b *testing.B) {
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
		key := fmt.Sprintf("key_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		_, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBatchPut 基准测试批量Put操作
func BenchmarkBatchPut(b *testing.B) {
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
		items := make(map[string][]byte)
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("batch_%d_%d", i, j)
			value := []byte(fmt.Sprintf("batch_value_%d_%d", i, j))
			items[key] = value
		}
		if err := db.BatchPut(items); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBatchGet 基准测试批量Get操作
func BenchmarkBatchGet(b *testing.B) {
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
	items := make(map[string][]byte)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("batch_key_%d", i)
		value := []byte(fmt.Sprintf("batch_value_%d", i))
		items[key] = value
	}
	if err := db.BatchPut(items); err != nil {
		b.Fatal(err)
	}

	keys := make([]string, 0, 1000)
	for key := range items {
		keys = append(keys, key)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.BatchGet(keys)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPutWithTTL 基准测试带TTL的Put操作
func BenchmarkPutWithTTL(b *testing.B) {
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
		key := fmt.Sprintf("ttl_%d", i)
		value := []byte(fmt.Sprintf("ttl_value_%d", i))
		if err := db.PutWithTTL(key, value, time.Minute); err != nil {
			b.Fatal(err)
		}
	}
}