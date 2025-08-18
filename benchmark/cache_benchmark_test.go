package benchmark

import (
	"fmt"
	"testing"

	gojidb "GojiDB"
)

// BenchmarkCacheHitRate 基准测试缓存命中率
func BenchmarkCacheHitRate(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
		CacheConfig: gojidb.CacheConfig{
			BlockCacheSize: 64,
			KeyCacheSize:   256,
			EnableCache:    true,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("cache_%d", i)
		value := []byte(fmt.Sprintf("cache_value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("cache_%d", i%1000)
		_, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCacheDisabled 基准测试禁用缓存的情况
func BenchmarkCacheDisabled(b *testing.B) {
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
		value := []byte(fmt.Sprintf("no_cache_value_%d", i))
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

// BenchmarkCacheLarge 基准测试大缓存配置
func BenchmarkCacheLarge(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		SyncWrites:  false,
		CacheConfig: gojidb.CacheConfig{
			BlockCacheSize: 512,
			KeyCacheSize:   1024,
			EnableCache:    true,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充数据
	for i := 0; i < 2000; i++ {
		key := fmt.Sprintf("large_cache_%d", i)
		value := []byte(fmt.Sprintf("large_cache_value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("large_cache_%d", i%2000)
		_, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}
