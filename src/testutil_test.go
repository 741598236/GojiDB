package main

import (
	"io"
	"os"
	"testing"
)

// 创建临时数据库用于测试
func newTestDB(t *testing.T) (*GojiDB, func()) {
	tempDir := t.TempDir()
	conf := &GojiDBConfig{
		DataPath:            tempDir,
		MaxFileSize:         2 * 1024 * 1024, // 2M
		CompressionType:     SnappyCompression,
		SyncWrites:          false,
		CompactionThreshold: 0.5,
		TTLCheckInterval:    0, // 关闭 TTL 清理 goroutine
		EnableMetrics:       true,
		CacheConfig: CacheConfig{
			BlockCacheSize: 128, // 测试环境使用较小的缓存
			KeyCacheSize:   512, // 测试环境使用较小的缓存
			EnableCache:    true,
		},
		// 测试中禁用智能压缩以确保压缩格式一致性
		SmartCompression: nil,
	}
	db, err := NewGojiDB(conf)
	if err != nil {
		t.Fatalf("newTestDB: %v", err)
	}
	return db, func() { _ = db.Close() }
}

// 捕获 stdout
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old
	return string(out)
}
