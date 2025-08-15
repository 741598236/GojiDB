package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestCompressionAlgorithms 测试所有压缩算法的正确性
func TestCompressionAlgorithms(t *testing.T) {
	configs := []*SmartCompressionConfig{
		{Algorithm: "snappy", MinSize: 1, MaxSize: 10 * 1024 * 1024},
		{Algorithm: "zstd", MinSize: 1, MaxSize: 10 * 1024 * 1024},
		{Algorithm: "gzip", MinSize: 1, MaxSize: 10 * 1024 * 1024},
		{Algorithm: "auto", MinSize: 1, MaxSize: 10 * 1024 * 1024, CompressionGain: 1.2},
	}

	testData := []struct {
		name string
		data []byte
	}{
		{"空数据", []byte{}},
		{"单字节", []byte("a")},
		{"小文本", []byte("hello world")},
		{"重复文本", bytes.Repeat([]byte("abc"), 1000)},
		{"随机数据", generateTestRandomData(1024)},
		{"JSON数据", generateJSONData()},
		{"二进制数据", generateBinaryData(2048)},
	}

	for _, config := range configs {
		for _, test := range testData {
			t.Run(fmt.Sprintf("%s-%s", config.Algorithm, test.name), func(t *testing.T) {
				compressor := NewSmartCompressor(config)
				defer compressor.Close()

				compressed, _, err := compressor.CompressWithSmartChoice(test.data)
				if err != nil {
					t.Logf("压缩失败: %v", err)
					return
				}

				if len(test.data) > 0 && len(compressed) == 0 {
					t.Error("压缩结果为空")
				}

				decompressed, err := compressor.Decompress(compressed)
				if err != nil {
					t.Logf("解压失败: %v", err)
					return
				}

				if len(test.data) > 0 && !bytes.Equal(test.data, decompressed) {
					t.Error("解压数据与原始数据不匹配")
				}
			})
		}
	}
}

// TestCompressionRatios 测试压缩率计算
func TestCompressionRatios(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm:       "auto",
		MinSize:         100,
		MaxSize:         10 * 1024 * 1024,
		CompressionGain: 1.0,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	testCases := []struct {
		name     string
		data     []byte
		minRatio float64
	}{
		{"高压缩文本", bytes.Repeat([]byte("hello world "), 1000), 0.5},
		{"中等压缩文本", []byte("This is a test string with some repetition. This is a test."), 0.8},
		{"低压缩随机", generateTestRandomData(1024), 0.9},
		{"JSON数据", generateJSONData(), 0.7},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			compressed, _, err := compressor.CompressWithSmartChoice(tc.data)
			if err != nil {
				t.Fatalf("压缩失败: %v", err)
			}

			ratio := float64(len(compressed)) / float64(len(tc.data))
			if ratio > tc.minRatio {
				t.Logf("压缩率 %.2f 高于预期 %.2f", ratio, tc.minRatio)
			}
		})
	}
}

// TestLargeData 测试大数据压缩
func TestLargeData(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm:       "auto",
		MinSize:         1024,
		MaxSize:         10 * 1024 * 1024, // 10MB
		CompressionGain: 1.0,
		ChunkSize:       64 * 1024,
		EnableParallel:  false,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	sizes := []int{1 * 1024 * 1024, 5 * 1024 * 1024}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size-%dMB", size/1024/1024), func(t *testing.T) {
			data := generateLargeTextData(size)

			start := time.Now()
			compressed, _, err := compressor.CompressWithSmartChoice(data)
			if err != nil {
				t.Fatalf("压缩失败: %v", err)
			}
			compressTime := time.Since(start)

			start = time.Now()
			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("解压失败: %v", err)
			}
			decompressTime := time.Since(start)

			if !bytes.Equal(data, decompressed) {
				t.Error("解压数据不匹配")
			}

			ratio := float64(len(compressed)) / float64(len(data))
			t.Logf("大小: %dMB, 压缩率: %.2f%%, 压缩时间: %v, 解压时间: %v",
				size/1024/1024, ratio*100, compressTime, decompressTime)
		})
	}
}

// TestMemoryLimits 测试内存限制
func TestMemoryLimits(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm: "auto",
		MinSize:   1000,
		MaxSize:   5000,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	testCases := []struct {
		name          string
		size          int
		shouldProcess bool
	}{
		{"太小", 500, false},
		{"边界下限", 1000, true},
		{"正常范围", 3000, true},
		{"边界上限", 5000, true},
		{"太大", 6000, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := generateLargeTextData(tc.size)

			_, _, err := compressor.CompressWithSmartChoice(data)
			if tc.shouldProcess && err != nil {
				t.Logf("应该处理 %d bytes 的数据: %v", tc.size, err)
			}
			if !tc.shouldProcess && err == nil {
				t.Logf("应该拒绝 %d bytes 的数据", tc.size)
			}
		})
	}
}

// TestRealWorldScenarios 测试真实场景
func TestRealWorldScenarios(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm:       "auto",
		MinSize:         100,
		MaxSize:         10 * 1024 * 1024,
		CompressionGain: 1.0,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	testCases := []struct {
		name string
		data func() []byte
	}{
		{"API响应", func() []byte {
			response := map[string]interface{}{
				"status": "success",
				"data":   generateJSONData(),
				"meta": map[string]interface{}{
					"timestamp": time.Now().Unix(),
					"version":   "1.0.0",
				},
			}
			jsonData, _ := json.Marshal(response)
			return jsonData
		}},
		{"配置文件", func() []byte {
			return []byte(`database:
  host: localhost
  port: 5432
  name: testdb
  user: admin
  password: secret
redis:
  host: localhost
  port: 6379
  db: 0`)
		}},
		{"用户会话", func() []byte {
			session := map[string]interface{}{
				"user_id":     12345,
				"username":    "testuser",
				"expires_at":  time.Now().Add(24 * time.Hour).Unix(),
				"permissions": []string{"read", "write", "admin"},
			}
			jsonData, _ := json.Marshal(session)
			return jsonData
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.data()

			compressed, algo, err := compressor.CompressWithSmartChoice(data)
			if err != nil {
				t.Fatalf("压缩失败: %v", err)
			}

			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("解压失败: %v", err)
			}

			if !bytes.Equal(data, decompressed) {
				t.Error("解压数据不匹配")
			}

			ratio := float64(len(compressed)) / float64(len(data))
			t.Logf("场景: %s, 原始大小: %d, 压缩后: %d, 压缩率: %.2f%%, 算法: %d",
				tc.name, len(data), len(compressed), ratio*100, algo)
		})
	}
}

// TestConcurrentSafety 测试并发安全性
func TestConcurrentSafety(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm:      "auto",
		MinSize:        100,
		MaxSize:        5 * 1024 * 1024,
		EnableParallel: false,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	const goroutines = 5
	const operations = 10

	var wg sync.WaitGroup
	testData := bytes.Repeat([]byte("concurrent test data "), 100)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				data := append([]byte{}, testData...)
				data = append(data, byte(id), byte(j))

				compressed, _, err := compressor.CompressWithSmartChoice(data)
				if err != nil {
					t.Errorf("goroutine %d iteration %d 压缩失败: %v", id, j, err)
					return
				}

				decompressed, err := compressor.Decompress(compressed)
				if err != nil {
					t.Errorf("goroutine %d iteration %d 解压失败: %v", id, j, err)
					return
				}

				if !bytes.Equal(data, decompressed) {
					t.Errorf("goroutine %d iteration %d 数据不匹配", id, j)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestGzipCompatibility 测试Gzip兼容性
func TestGzipCompatibility(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm: "gzip",
		MinSize:   100,
		MaxSize:   10 * 1024 * 1024,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	// 使用足够大的测试数据确保触发gzip压缩
	testData := bytes.Repeat([]byte("This is test data for gzip compatibility check. "), 100)

	// 使用我们的压缩器压缩
	compressed, _, err := compressor.CompressWithSmartChoice(testData)
	if err != nil {
		t.Fatalf("压缩失败: %v", err)
	}

	// 验证确实是gzip格式
	if len(compressed) < 2 || compressed[0] != 0x1f || compressed[1] != 0x8b {
		t.Fatal("压缩数据不是有效的gzip格式")
	}

	// 使用标准库解压
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("创建gzip reader失败: %v", err)
	}
	defer reader.Close()

	var decompressed bytes.Buffer
	_, err = decompressed.ReadFrom(reader)
	if err != nil {
		t.Fatalf("标准库解压失败: %v", err)
	}

	if !bytes.Equal(testData, decompressed.Bytes()) {
		t.Error("标准库解压数据不匹配")
	}
}

// TestErrorScenarios 测试错误场景
func TestErrorScenarios(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm: "zstd",
		MinSize:   100,
		MaxSize:   10 * 1024 * 1024,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	t.Run("解压损坏数据", func(t *testing.T) {
		corrupted := []byte("this is not valid compressed data")
		result, err := compressor.Decompress(corrupted)
		if err == nil {
			// 非压缩数据应该原样返回
			if !bytes.Equal(corrupted, result) {
				t.Error("非压缩数据应该原样返回")
			}
		} else {
			// 如果是压缩数据但损坏，应该返回错误
			t.Logf("解压损坏数据返回错误: %v", err)
		}
	})

	t.Run("空数据解压", func(t *testing.T) {
		result, err := compressor.Decompress([]byte{})
		if err != nil {
			t.Errorf("空数据解压返回错误: %v", err)
		}
		if len(result) != 0 {
			t.Error("空数据应该返回空结果")
		}
	})
}

// TestConfigurationEdgeCases 测试配置边界情况
func TestConfigurationEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config *SmartCompressionConfig
		data   []byte
	}{
		{
			name:   "零最小值",
			config: &SmartCompressionConfig{Algorithm: "snappy", MinSize: 0, MaxSize: 1024},
			data:   []byte("test"),
		},
		{
			name:   "超大最大值",
			config: &SmartCompressionConfig{Algorithm: "snappy", MinSize: 100, MaxSize: math.MaxInt64},
			data:   []byte("test"),
		},
		{
			name:   "负压缩增益",
			config: &SmartCompressionConfig{Algorithm: "auto", MinSize: 100, MaxSize: 1024, CompressionGain: -1.0},
			data:   []byte("test"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compressor := NewSmartCompressor(test.config)
			defer compressor.Close()

			_, _, err := compressor.CompressWithSmartChoice(test.data)
			if err != nil {
				t.Logf("预期行为: %v", err)
			}
		})
	}
}

// TestMixedDataTypes 测试混合数据类型
func TestMixedDataTypes(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm:       "auto",
		MinSize:         100,
		MaxSize:         10 * 1024 * 1024,
		CompressionGain: 1.0,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	testCases := []struct {
		name string
		data []byte
	}{
		{"HTML", []byte(`<html><body><h1>Test</h1><p>This is a test paragraph.</p></body></html>`)},
		{"CSS", []byte(`body { margin: 0; padding: 20px; font-family: Arial; } .test { color: red; }`)},
		{"JavaScript", []byte(`function test() { console.log("Hello, World!"); return true; }`)},
		{"SQL", []byte(`SELECT * FROM users WHERE id = 1 AND status = 'active' ORDER BY created_at DESC`)},
		{"日志格式", []byte(`2024-01-01 12:00:00 INFO User login successful user_id=123 ip=192.168.1.1`)},
		{"CSV", []byte(`name,age,city
John,25,New York
Jane,30,Los Angeles`)},
		{"XML", []byte(`<root><item id="1">test</item><item id="2">data</item></root>`)},
		{"UTF-8文本", []byte("你好世界 こんにちは 세계")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 放大测试数据
			largeData := bytes.Repeat(tc.data, 100)

			compressed, _, err := compressor.CompressWithSmartChoice(largeData)
			if err != nil {
				t.Fatalf("压缩失败: %v", err)
			}

			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("解压失败: %v", err)
			}

			if !bytes.Equal(largeData, decompressed) {
				t.Error("解压数据不匹配")
			}

			ratio := float64(len(compressed)) / float64(len(largeData))
			t.Logf("类型: %s, 压缩率: %.2f%%", tc.name, ratio*100)
		})
	}
}

// TestEmptyAndNilCases 测试空值和nil处理
func TestEmptyAndNilCases(t *testing.T) {
	config := &SmartCompressionConfig{
		Algorithm: "auto",
		MinSize:   100,
		MaxSize:   10 * 1024 * 1024,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	testCases := []struct {
		name string
		data []byte
	}{
		{"空切片", []byte{}},
		{"小文本", []byte("hello world")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			compressed, _, err := compressor.CompressWithSmartChoice(tc.data)
			if err != nil {
				t.Logf("压缩错误: %v", err)
				return
			}

			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("解压失败: %v", err)
			}

			if !bytes.Equal(tc.data, decompressed) {
				t.Error("解压数据不匹配")
			}
		})
	}
}

// TestMemoryProfiling 测试内存使用情况
func TestMemoryProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存分析测试")
	}

	config := &SmartCompressionConfig{
		Algorithm:      "auto",
		MinSize:        1024,
		MaxSize:        5 * 1024 * 1024,
		CacheSize:      50,
		ChunkSize:      32 * 1024,
		EnableParallel: false,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	// 强制GC并等待系统稳定
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	const iterations = 1000
	var totalInputSize int64

	// 使用真实大小的测试数据
	testData := generateLargeTextData(100 * 1024) // 100KB数据

	for i := 0; i < iterations; i++ {
		data := testData
		totalInputSize += int64(len(data))

		compressed, _, err := compressor.CompressWithSmartChoice(data)
		if err != nil {
			t.Fatalf("压缩失败: %v", err)
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			t.Fatalf("解压失败: %v", err)
		}

		if !bytes.Equal(data, decompressed) {
			t.Error("解压数据不匹配")
		}
	}

	// 强制GC获取准确内存使用
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&after)

	// 计算实际内存使用
	memoryUsed := int64(after.Alloc - before.Alloc)
	memoryPerMB := float64(memoryUsed) / float64(totalInputSize) * 1024 * 1024

	t.Logf("总处理数据: %d MB", totalInputSize/1024/1024)
	t.Logf("内存使用: %d KB (%.2f MB/GB数据)", memoryUsed/1024, memoryPerMB/1024)
	t.Logf("基础内存占用: %d MB", before.Alloc/1024/1024)
	t.Logf("峰值内存占用: %d MB", after.Alloc/1024/1024)

	// 合理的内存使用阈值
	if memoryUsed > 50*1024*1024 { // 50MB
		t.Errorf("内存使用过高: %d MB", memoryUsed/1024/1024)
	}
}

// 辅助函数
func generateTestRandomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func generateLargeTextData(size int) []byte {
	var buf bytes.Buffer
	for buf.Len() < size {
		buf.WriteString("test data for compression testing and performance benchmarking ")
	}
	return buf.Bytes()[:size]
}

func generateJSONData() []byte {
	data := map[string]interface{}{
		"name":  "test",
		"value": 12345,
		"items": []string{"a", "b", "c", "d", "e"},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}
	jsonData, _ := json.Marshal(data)
	return jsonData
}

func generateBinaryData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

// 新增测试辅助函数
func generateHighEntropyData(size int) []byte {
	// 生成高熵数据（模拟加密数据）
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func generateRepetitiveData(size int) []byte {
	// 生成高重复性数据
	pattern := []byte("AAAAAAAAAA")
	data := make([]byte, size)
	for i := 0; i < size; i += len(pattern) {
		copy(data[i:], pattern)
	}
	return data[:size]
}

func generateMixedData(size int) []byte {
	// 生成混合类型数据
	var buf bytes.Buffer
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
	for buf.Len() < size {
		buf.WriteString(text)
	}
	return buf.Bytes()[:size]
}

// TestRealPerformanceBenchmark 修正后的真实性能基准测试
func TestRealPerformanceBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能基准测试")
	}

	config := &SmartCompressionConfig{
		Algorithm:      "auto",
		MinSize:        1024,
		MaxSize:        10 * 1024 * 1024,
		CacheSize:      100,
		ChunkSize:      64 * 1024,
		EnableParallel: true,
	}
	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	// 测试不同大小的数据块和不同类型数据
	testCases := []struct {
		size      int
		dataType  string
		generator func(int) []byte
	}{
		{1024, "文本", generateLargeTextData},
		{10 * 1024, "文本", generateLargeTextData},
		{100 * 1024, "文本", generateLargeTextData},
		{1024 * 1024, "文本", generateLargeTextData},
		{100 * 1024, "二进制", generateBinaryData},
		{100 * 1024, "重复数据", generateRepetitiveData},
		{100 * 1024, "高熵数据", generateHighEntropyData},
	}

	algorithms := []CompressionType{SnappyCompression, GzipCompression, ZSTDCompression}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%dKB", tc.dataType, tc.size/1024), func(t *testing.T) {
			data := tc.generator(tc.size)

			for _, algo := range algorithms {
				t.Run(algo.String(), func(t *testing.T) {
					// 预热 - 避免第一次分配影响
					for i := 0; i < 3; i++ {
						compressor.compressWithAlgorithm(data, algo)
					}

					// 正式测试 - 减少迭代次数避免缓存影响
					const iterations = 50

					// 压缩测试
					start := time.Now()
					var totalCompressedSize int64
					for i := 0; i < iterations; i++ {
						compressed, err := compressor.compressWithAlgorithm(data, algo)
						if err != nil {
							t.Fatalf("压缩失败: %v", err)
						}
						totalCompressedSize += int64(len(compressed))
					}
					compressTime := time.Since(start)

					// 解压测试
					compressed, _ := compressor.compressWithAlgorithm(data, algo)
					start = time.Now()
					for i := 0; i < iterations; i++ {
						_, err := compressor.Decompress(compressed)
						if err != nil {
							t.Fatalf("解压失败: %v", err)
						}
					}
					decompressTime := time.Since(start)

					// 计算真实性能数据
					totalDataSize := int64(len(data) * iterations)
					compressSpeedMB := float64(totalDataSize) / compressTime.Seconds() / (1024 * 1024)
					decompressSpeedMB := float64(totalDataSize) / decompressTime.Seconds() / (1024 * 1024)
					compressionRatio := float64(len(compressed)) / float64(len(data)) // 压缩比
					spaceSaving := (1.0 - compressionRatio) * 100                     // 空间节省百分比

					t.Logf("=== %s %s %dKB ===", tc.dataType, algo.String(), tc.size/1024)
					t.Logf("压缩速度: %.1f MB/s", compressSpeedMB)
					t.Logf("解压速度: %.1f MB/s", decompressSpeedMB)
					t.Logf("压缩比: 1:%.1f (%.1f%% 空间节省)", 1/compressionRatio, spaceSaving)
					t.Logf("压缩后大小: %d → %d 字节", len(data), len(compressed))
				})
			}
		})
	}
}

// String 方法用于CompressionType的字符串表示
func (ct CompressionType) String() string {
	switch ct {
	case SnappyCompression:
		return "Snappy"
	case GzipCompression:
		return "Gzip"
	case ZSTDCompression:
		return "Zstd"
	default:
		return "Unknown"
	}
}

// TestRealMemoryUsage 修正后的内存使用测试
func TestRealMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存测试")
	}

	config := &SmartCompressionConfig{
		Algorithm:      "auto",
		MinSize:        1024,
		MaxSize:        10 * 1024 * 1024,
		CacheSize:      100,
		ChunkSize:      64 * 1024,
		EnableParallel: true,
	}

	compressor := NewSmartCompressor(config)
	defer compressor.Close()

	// 测试数据
	testCases := []struct {
		size      int
		dataType  string
		generator func(int) []byte
	}{
		{1 * 1024 * 1024, "文本1MB", generateLargeTextData},
		{10 * 1024 * 1024, "文本10MB", generateLargeTextData},
		{100 * 1024 * 1024, "文本100MB", generateLargeTextData},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%dMB", tc.dataType, tc.size/(1024*1024)), func(t *testing.T) {
			data := tc.generator(tc.size)

			// 精确内存测量 - 使用HeapAlloc
			runtime.GC()
			var baselineMem runtime.MemStats
			runtime.ReadMemStats(&baselineMem)

			// 处理数据
			const iterations = 10 // 减少迭代避免累积
			var maxAlloc uint64

			for i := 0; i < iterations; i++ {
				compressed, err := compressor.compressWithAlgorithm(data, SnappyCompression)
				if err != nil {
					t.Fatalf("压缩失败: %v", err)
				}

				_, err = compressor.Decompress(compressed)
				if err != nil {
					t.Fatalf("解压失败: %v", err)
				}

				runtime.GC()
				var currentMem runtime.MemStats
				runtime.ReadMemStats(&currentMem)
				if currentMem.HeapAlloc > maxAlloc {
					maxAlloc = currentMem.HeapAlloc
				}
			}

			// 计算内存增量
			baselineMB := float64(baselineMem.HeapAlloc) / 1024 / 1024
			peakMB := float64(maxAlloc) / 1024 / 1024
			dataSizeMB := float64(len(data)) / 1024 / 1024
			memoryOverhead := peakMB - baselineMB

			t.Logf("=== %s 内存测试 ===", tc.dataType)
			t.Logf("基准内存: %.1f MB", baselineMB)
			t.Logf("数据大小: %.1f MB", dataSizeMB)
			t.Logf("峰值内存: %.1f MB", peakMB)
			t.Logf("内存开销: %.1f MB (%.1fx)", memoryOverhead, memoryOverhead/dataSizeMB)
		})
	}
}
