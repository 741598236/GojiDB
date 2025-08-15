package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIHelp(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	out := captureStdout(func() { db.showHelp() })
	assert.Contains(t, out, "put/set <key> <value> [ttl]")
}

func TestCLIBasic(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// 测试基本的CLI功能不panic
	assert.NotPanics(t, func() {
		// 这里我们测试各个处理函数而不是整个交互模式
		db.handlePut([]string{"put", "test", "value"})
		db.handleGet([]string{"get", "test"})
		db.handleList([]string{"list"})
		db.handleStats()
	})
}

func TestCLIStats(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// 添加一些测试数据
	db.handlePut([]string{"put", "test1", "value1"})
	db.handlePut([]string{"put", "test2", "value2"})
	db.handleGet([]string{"get", "test1"})
	db.handleGet([]string{"get", "test2"})
	db.handleDelete([]string{"delete", "test1"})

	// 捕获并验证统计信息输出
	out := captureStdout(func() { db.handleStats() })

	// 验证基本信息部分
	assert.Contains(t, out, "📊 GojiDB 智能统计面板")
	assert.Contains(t, out, "数据概览")
	assert.Contains(t, out, "总键数:")
	assert.Contains(t, out, "运行时间:")
	assert.Contains(t, out, "总读取:")
	assert.Contains(t, out, "总写入:")
	assert.Contains(t, out, "总删除:")
	assert.Contains(t, out, "合并次数:")

	// 验证性能指标部分
	assert.Contains(t, out, "性能指标")
	assert.Contains(t, out, "缓存命中:")
	assert.Contains(t, out, "未命中:")
	assert.Contains(t, out, "命中率:")
	assert.Contains(t, out, "QPS:")
	assert.Contains(t, out, "过期键:")
	assert.Contains(t, out, "压缩比:")

	// 验证输出格式
	lines := strings.Split(out, "\n")
	assert.GreaterOrEqual(t, len(lines), 10, "输出应该至少有10行")

	// 验证包含表格边框
	borderCount := 0
	for _, line := range lines {
		if strings.Contains(line, "┌") || strings.Contains(line, "└") ||
			strings.Contains(line, "├") || strings.Contains(line, "│") {
			borderCount++
		}
	}
	assert.GreaterOrEqual(t, borderCount, 4, "输出应该包含至少4个表格边框字符")
}

// TestCLICacheDisabled 测试禁用缓存时的CLI行为
func TestCLICacheDisabled(t *testing.T) {
	tempDir := t.TempDir()
	conf := &GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: 1024 * 1024,
		CacheConfig: CacheConfig{
			BlockCacheSize: 64,
			KeyCacheSize:   256,
			EnableCache:    false,
		},
	}
	db, err := NewGojiDB(conf)
	assert.NoError(t, err)
	defer db.Close()

	// 验证缓存已禁用
	assert.Equal(t, 0, db.blockCache.cache.capacity)
	assert.Equal(t, 0, db.keyCache.cache.capacity)

	// 验证CLI命令仍能正常工作
	assert.NotPanics(t, func() {
		db.handlePut([]string{"put", "test", "value"})
		db.handleGet([]string{"get", "test"})
		db.handleStats()
	})

	// 验证统计信息包含缓存指标
	out := captureStdout(func() { db.handleStats() })
	assert.Contains(t, out, "缓存命中:")
	assert.Contains(t, out, "未命中:")
}
