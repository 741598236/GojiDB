package benchmark

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"gojidb"
)

// BenchmarkConcurrentPut 基准测试并发Put操作
func BenchmarkConcurrentPut(b *testing.B) {
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
			key := fmt.Sprintf("concurrent_%d_%d", b.N, i)
			value := []byte(fmt.Sprintf("concurrent_value_%d_%d", b.N, i))
			if err := db.Put(key, value); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkConcurrentGet 基准测试并发Get操作
func BenchmarkConcurrentGet(b *testing.B) {
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
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent_get_%d", i)
			value := []byte(fmt.Sprintf("concurrent_value_%d", i))
			db.Put(key, value)
		}(i)
	}
	wg.Wait()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("concurrent_get_%d", i%1000)
			_, err := db.Get(key)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkConcurrentMixed 基准测试并发混合操作
func BenchmarkConcurrentMixed(b *testing.B) {
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
			operation := i % 4
			switch operation {
			case 0: // Put
				key := fmt.Sprintf("mixed_%d_%d", b.N, i)
				value := []byte(fmt.Sprintf("mixed_value_%d_%d", b.N, i))
				if err := db.Put(key, value); err != nil {
					b.Fatal(err)
				}
			case 1: // Get
				key := fmt.Sprintf("mixed_%d_%d", b.N, i%100)
				_, _ = db.Get(key)
			case 2: // Delete
				key := fmt.Sprintf("mixed_%d_%d", b.N, i%100)
				_ = db.Delete(key)
			case 3: // Put with TTL
				key := fmt.Sprintf("mixed_ttl_%d_%d", b.N, i)
				value := []byte(fmt.Sprintf("mixed_ttl_value_%d_%d", b.N, i))
				if err := db.PutWithTTL(key, value, time.Minute); err != nil {
					b.Fatal(err)
				}
			}
			i++
		}
	})
}