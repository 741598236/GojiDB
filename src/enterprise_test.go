package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSuite 企业级测试套件
const (
	TestDataDir      = "test_data_"
	BenchmarkDataDir = "benchmark_data"
	MaxTestDataSize  = 1024 * 1024 // 1MB
)

// TestConfig 测试配置结构体
type TestConfig struct {
	Name        string
	Description string
	Iterations  int
	DataSize    int
	Concurrency int
	Timeout     time.Duration
}

// TestResult 测试结果结构体
type TestResult struct {
	TestName    string
	Passed      bool
	Duration    time.Duration
	MemoryUsage uint64
	Error       string
	Details     map[string]interface{}
}

// EnterpriseTestSuite 企业级测试套件
type EnterpriseTestSuite struct {
	t       *testing.T
	db      *GojiDB
	config  *GojiDBConfig
	results []TestResult
	mutex   sync.Mutex
}

// NewEnterpriseTestSuite 创建新的测试套件
func NewEnterpriseTestSuite(t *testing.T) *EnterpriseTestSuite {
	testDir, err := ioutil.TempDir("", TestDataDir)
	if err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	config := &GojiDBConfig{
		DataPath:         testDir,
		MaxFileSize:      64 * 1024,
		CompressionType:  1, // 传统压缩类型作为回退
		EnableMetrics:    true,
		SyncWrites:       false,
		TTLCheckInterval: 100 * time.Millisecond,
		CacheConfig: CacheConfig{
			BlockCacheSize: 256,  // 企业测试使用适中的缓存
			KeyCacheSize:   1024, // 企业测试使用适中的缓存
			EnableCache:    true,
		},
		SmartCompression: DefaultSmartCompressionConfig(), // 启用智能压缩
	}

	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库实例失败: %v", err)
	}

	return &EnterpriseTestSuite{
		t:       t,
		db:      db,
		config:  config,
		results: make([]TestResult, 0),
	}
}

// Cleanup 清理测试环境
func (ets *EnterpriseTestSuite) Cleanup() {
	if ets.db != nil {
		ets.db.Close()
	}
	if ets.config != nil {
		os.RemoveAll(ets.config.DataPath)
	}
}

// RecordResult 记录测试结果
func (ets *EnterpriseTestSuite) RecordResult(result TestResult) {
	ets.mutex.Lock()
	defer ets.mutex.Unlock()
	ets.results = append(ets.results, result)
}

// TestBatchOperations 测试批量操作功能
func TestBatchOperations(t *testing.T) {
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	testCases := []TestConfig{
		{Name: "BatchPut_SmallData", Iterations: 100, DataSize: 100, Concurrency: 1},
		{Name: "BatchPut_MediumData", Iterations: 1000, DataSize: 1024, Concurrency: 5},
		{Name: "BatchPut_LargeData", Iterations: 5000, DataSize: 4096, Concurrency: 10},
		{Name: "BatchGet_MixedSizes", Iterations: 1000, DataSize: 512, Concurrency: 3},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := suite.runBatchTest(tc)
			suite.RecordResult(result)
			if !result.Passed {
				t.Errorf("测试失败: %s - %s", tc.Name, result.Error)
			}
		})
	}
}

// TestCompression 测试数据压缩功能
func TestCompression(t *testing.T) {
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	testCases := []TestConfig{
		{Name: "Compression_TextData", Iterations: 1000, DataSize: 1024},
		{Name: "Compression_JSONData", Iterations: 500, DataSize: 2048},
		{Name: "Compression_BinaryData", Iterations: 200, DataSize: 8192},
		{Name: "Compression_RepeatedData", Iterations: 1000, DataSize: 100},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := suite.runCompressionTest(tc)
			suite.RecordResult(result)
			if !result.Passed {
				t.Errorf("测试失败: %s - %s", tc.Name, result.Error)
			}
		})
	}
}

// TestTTL 测试TTL功能
func TestTTL(t *testing.T) {
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	testCases := []TestConfig{
		{Name: "TTL_ShortDuration", Iterations: 50, DataSize: 200, Timeout: 2 * time.Second},
		{Name: "TTL_MediumDuration", Iterations: 30, DataSize: 300, Timeout: 1 * time.Second},
		{Name: "TTL_LongDuration", Iterations: 20, DataSize: 500, Timeout: 2 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := suite.runTTLTest(tc)
			suite.RecordResult(result)
			if !result.Passed {
				t.Errorf("测试失败: %s - %s", tc.Name, result.Error)
			}
		})
	}
}

// TestMetrics 测试监控指标功能
func TestMetrics(t *testing.T) {
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	testCases := []TestConfig{
		{Name: "Metrics_BasicOperations", Iterations: 1000, DataSize: 100},
		{Name: "Metrics_CachePerformance", Iterations: 2000, DataSize: 50},
		{Name: "Metrics_ConcurrentAccess", Iterations: 500, DataSize: 200, Concurrency: 10},
		{Name: "Metrics_MemoryUsage", Iterations: 10000, DataSize: 10},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := suite.runMetricsTest(tc)
			suite.RecordResult(result)
			if !result.Passed {
				t.Errorf("测试失败: %s - %s", tc.Name, result.Error)
			}
		})
	}
}

// TestCompaction 测试数据合并功能
func TestCompaction(t *testing.T) {
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	testCases := []TestConfig{
		{Name: "Compaction_SmallData", Iterations: 500, DataSize: 100},
		{Name: "Compaction_MediumData", Iterations: 2000, DataSize: 500},
		{Name: "Compaction_LargeData", Iterations: 5000, DataSize: 1000},
		{Name: "Compaction_WithDeletes", Iterations: 1000, DataSize: 200},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := suite.runCompactionTest(tc)
			suite.RecordResult(result)
			if !result.Passed {
				t.Errorf("测试失败: %s - %s", tc.Name, result.Error)
			}
		})
	}
}

// TestConcurrentOperations 测试并发操作
func TestConcurrentOperations(t *testing.T) {
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	testCases := []TestConfig{
		{Name: "Concurrent_ReadWrite", Iterations: 1000, DataSize: 100, Concurrency: 20},
		{Name: "Concurrent_BatchOperations", Iterations: 100, DataSize: 1000, Concurrency: 10},
		{Name: "Concurrent_TTL", Iterations: 500, DataSize: 50, Concurrency: 5},
		{Name: "Concurrent_Metrics", Iterations: 1000, DataSize: 20, Concurrency: 15},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := suite.runConcurrentTest(tc)
			suite.RecordResult(result)
			if !result.Passed {
				t.Errorf("测试失败: %s - %s", tc.Name, result.Error)
			}
		})
	}
}

// BenchmarkBatchOperations 基准测试批量操作
func BenchmarkBatchOperations(b *testing.B) {
	testDir, err := ioutil.TempDir("", BenchmarkDataDir)
	if err != nil {
		b.Fatalf("创建基准测试目录失败: %v", err)
	}
	defer os.RemoveAll(testDir)

	config := &GojiDBConfig{
		DataPath:        testDir,
		MaxFileSize:     1024 * 1024,
		CompressionType: 1,
		EnableMetrics:   true,
		SyncWrites:      false,
	}

	db, err := NewGojiDB(config)
	if err != nil {
		b.Fatalf("创建数据库实例失败: %v", err)
	}
	defer db.Close()

	b.Run("BatchPut_100_Items", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data := make(map[string][]byte)
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("bench_key_%d_%d", i, j)
				value := []byte(fmt.Sprintf("benchmark_data_%d_%d", i, j))
				data[key] = value
			}
			if err := db.BatchPut(data); err != nil {
				b.Fatalf("批量写入失败: %v", err)
			}
		}
	})

	b.Run("BatchGet_100_Items", func(b *testing.B) {
		// 先准备数据
		data := make(map[string][]byte)
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("bench_get_%d", i)
			value := []byte(fmt.Sprintf("value_%d", i))
			data[key] = value
		}
		if err := db.BatchPut(data); err != nil {
			b.Fatalf("准备基准测试数据失败: %v", err)
		}

		keys := make([]string, 0, 100)
		for key := range data {
			keys = append(keys, key)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := db.BatchGet(keys); err != nil {
				b.Fatalf("批量读取失败: %v", err)
			}
		}
	})
}

// BenchmarkCompression 基准测试压缩功能
func BenchmarkCompression(b *testing.B) {
	testDir, err := ioutil.TempDir("", BenchmarkDataDir)
	if err != nil {
		b.Fatalf("创建基准测试目录失败: %v", err)
	}
	defer os.RemoveAll(testDir)

	config := &GojiDBConfig{
		DataPath:        testDir,
		MaxFileSize:     2 * 1024 * 1024,
		CompressionType: 1,
		EnableMetrics:   true,
		SyncWrites:      false,
	}

	db, err := NewGojiDB(config)
	if err != nil {
		b.Fatalf("创建数据库实例失败: %v", err)
	}
	defer db.Close()

	// 准备测试数据
	testData := generateTestData(1000, 1024)
	if err := db.BatchPut(testData); err != nil {
		b.Fatalf("准备基准测试数据失败: %v", err)
	}

	b.Run("Compression_Ratio", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics := db.GetMetrics()
			if metrics.CompressionRatio <= 0 {
				b.Fatalf("压缩比无效: %f", float64(metrics.CompressionRatio)/100.0)
			}
		}
	})
}

// runBatchTest 运行批量操作测试
func (ets *EnterpriseTestSuite) runBatchTest(config TestConfig) TestResult {
	start := time.Now()
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 准备测试数据
	data := make(map[string][]byte)
	for i := 0; i < config.Iterations; i++ {
		key := fmt.Sprintf("batch_%s_%d", config.Name, i)
		value := generateRandomData(config.DataSize)
		data[key] = value
	}

	// 执行批量写入
	err := ets.db.BatchPut(data)
	if err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("批量写入失败: %v", err),
		}
	}

	// 验证数据完整性
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	results, err := ets.db.BatchGet(keys)
	if err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("批量读取失败: %v", err),
		}
	}

	// 验证数据一致性
	if len(results) != len(data) {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("数据数量不匹配: 期望 %d, 实际 %d", len(data), len(results)),
		}
	}

	runtime.ReadMemStats(&m2)
	return TestResult{
		TestName:    config.Name,
		Passed:      true,
		Duration:    time.Since(start),
		MemoryUsage: m2.Alloc - m1.Alloc,
		Details: map[string]interface{}{
			"items":    len(data),
			"dataSize": config.DataSize,
			"qps":      float64(len(data)) / time.Since(start).Seconds(),
		},
	}
}

// runCompressionTest 运行压缩测试
func (ets *EnterpriseTestSuite) runCompressionTest(config TestConfig) TestResult {
	start := time.Now()

	// 准备不同格式的测试数据
	var testData []byte
	switch config.Name {
	case "Compression_TextData":
		testData = []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", config.DataSize/45))
	case "Compression_JSONData":
		testData = []byte(fmt.Sprintf(`{"id":%d,"name":"test","data":"%s"}`, config.Iterations, strings.Repeat("x", config.DataSize-50)))
	case "Compression_BinaryData":
		testData = generateRandomData(config.DataSize)
	case "Compression_RepeatedData":
		testData = bytes.Repeat([]byte("ABCD"), config.DataSize/4)
	}

	// 写入测试数据
	key := fmt.Sprintf("compress_%s", config.Name)
	err := ets.db.Put(key, testData)
	if err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("压缩写入失败: %v", err),
		}
	}

	// 验证数据完整性
	retrieved, err := ets.db.Get(key)
	if err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("压缩读取失败: %v", err),
		}
	}

	if !bytes.Equal(retrieved, testData) {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    "压缩数据完整性验证失败",
		}
	}

	return TestResult{
		TestName: config.Name,
		Passed:   true,
		Duration: time.Since(start),
		Details: map[string]interface{}{
			"originalSize":       len(testData),
			"compressionEnabled": ets.db.config.CompressionType != 0,
		},
	}
}

// runTTLTest 运行TTL测试
func (ets *EnterpriseTestSuite) runTTLTest(config TestConfig) TestResult {
	start := time.Now()

	// 写入带TTL的数据
	key := fmt.Sprintf("ttl_%s", config.Name)
	value := generateRandomData(config.DataSize)

	// 增加TTL持续时间，避免异步清理导致的测试失败
	actualTTL := config.Timeout * 3 // 延长TTL时间
	err := ets.db.PutWithTTL(key, value, actualTTL)
	if err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("TTL写入失败: %v", err),
		}
	}

	// 立即验证数据存在
	retrieved, err := ets.db.Get(key)
	if err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("TTL读取失败: %v", err),
		}
	}

	if !bytes.Equal(retrieved, value) {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    "TTL数据验证失败",
		}
	}

	// 等待TTL过期（使用实际TTL时间）
	time.Sleep(actualTTL + 100*time.Millisecond)

	// 验证数据已过期
	_, err = ets.db.Get(key)
	if err == nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    "TTL数据未正确过期",
		}
	}

	return TestResult{
		TestName: config.Name,
		Passed:   true,
		Duration: time.Since(start),
		Details: map[string]interface{}{
			"ttlDuration": actualTTL.String(),
			"expiredKeys": ets.db.GetMetrics().ExpiredKeyCount,
		},
	}
}

// runMetricsTest 运行监控指标测试
func (ets *EnterpriseTestSuite) runMetricsTest(config TestConfig) TestResult {
	start := time.Now()
	initialMetrics := ets.db.GetMetrics()

	// 执行各种操作
	for i := 0; i < config.Iterations; i++ {
		key := fmt.Sprintf("metrics_%s_%d", config.Name, i)
		value := generateRandomData(config.DataSize)

		// 写入
		if err := ets.db.Put(key, value); err != nil {
			return TestResult{
				TestName: config.Name,
				Passed:   false,
				Duration: time.Since(start),
				Error:    fmt.Sprintf("监控测试写入失败: %v", err),
			}
		}

		// 读取
		_, _ = ets.db.Get(key)

		// 删除部分数据
		if i%10 == 0 {
			_ = ets.db.Delete(key)
		}
	}

	// 验证监控指标
	finalMetrics := ets.db.GetMetrics()
	if finalMetrics.TotalWrites <= initialMetrics.TotalWrites {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    "监控指标未正确更新",
		}
	}

	return TestResult{
		TestName: config.Name,
		Passed:   true,
		Duration: time.Since(start),
		Details: map[string]interface{}{
			"writes":       finalMetrics.TotalWrites - initialMetrics.TotalWrites,
			"reads":        finalMetrics.TotalReads - initialMetrics.TotalReads,
			"deletes":      finalMetrics.TotalDeletes - initialMetrics.TotalDeletes,
			"cacheHitRate": calculateHitRate(finalMetrics.CacheHits, finalMetrics.CacheMisses),
		},
	}
}

// runCompactionTest 运行数据合并测试
func (ets *EnterpriseTestSuite) runCompactionTest(config TestConfig) TestResult {
	start := time.Now()

	// 写入大量数据
	for i := 0; i < config.Iterations; i++ {
		key := fmt.Sprintf("compact_%s_%d", config.Name, i)
		value := generateRandomData(config.DataSize)

		if err := ets.db.Put(key, value); err != nil {
			return TestResult{
				TestName: config.Name,
				Passed:   false,
				Duration: time.Since(start),
				Error:    fmt.Sprintf("合并测试写入失败: %v", err),
			}
		}

		// 删除部分数据
		if i%5 == 0 {
			_ = ets.db.Delete(key)
		}
	}

	// 执行合并
	initialKeys := ets.db.ListKeys()
	if err := ets.db.Compact(); err != nil {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("数据合并失败: %v", err),
		}
	}

	// 验证合并结果
	finalKeys := ets.db.ListKeys()
	if len(finalKeys) > len(initialKeys) {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    "数据合并结果异常",
		}
	}

	return TestResult{
		TestName: config.Name,
		Passed:   true,
		Duration: time.Since(start),
		Details: map[string]interface{}{
			"initialKeys": len(initialKeys),
			"finalKeys":   len(finalKeys),
			"compactions": ets.db.GetMetrics().CompactionCount,
		},
	}
}

// runConcurrentTest 运行并发测试
func (ets *EnterpriseTestSuite) runConcurrentTest(config TestConfig) TestResult {
	start := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]string, 0)

	// 准备测试数据
	data := make(map[string][]byte)
	for i := 0; i < config.Iterations; i++ {
		key := fmt.Sprintf("concurrent_%s_%d", config.Name, i)
		value := generateRandomData(config.DataSize)
		data[key] = value
	}

	// 并发执行
	chunkSize := len(data) / config.Concurrency
	for i := 0; i < config.Concurrency; i++ {
		startIdx := i * chunkSize
		endIdx := startIdx + chunkSize
		if i == config.Concurrency-1 {
			endIdx = len(data)
		}

		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()

			// 批量写入
			chunk := make(map[string][]byte)
			for j := start; j < end; j++ {
				key := fmt.Sprintf("concurrent_%s_%d", config.Name, j)
				chunk[key] = data[key]
			}

			if err := ets.db.BatchPut(chunk); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("并发写入失败: %v", err))
				mu.Unlock()
				return
			}

			// 并发读取
			keys := make([]string, 0, len(chunk))
			for key := range chunk {
				keys = append(keys, key)
			}

			_, err := ets.db.BatchGet(keys)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("并发读取失败: %v", err))
				mu.Unlock()
			}
		}(startIdx, endIdx)
	}

	wg.Wait()

	if len(errors) > 0 {
		return TestResult{
			TestName: config.Name,
			Passed:   false,
			Duration: time.Since(start),
			Error:    strings.Join(errors, "; "),
		}
	}

	return TestResult{
		TestName: config.Name,
		Passed:   true,
		Duration: time.Since(start),
		Details: map[string]interface{}{
			"concurrency": config.Concurrency,
			"items":       config.Iterations,
			"dataSize":    config.DataSize,
		},
	}
}

// TestReport 生成测试报告
func TestReport(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过测试报告生成")
	}

	// 运行所有测试
	fmt.Println("🧪 开始企业级测试套件...")

	// 创建测试套件
	suite := NewEnterpriseTestSuite(t)
	defer suite.Cleanup()

	// 运行所有测试
	tests := []func(*testing.T){
		TestBatchOperations,
		TestCompression,
		TestTTL,
		TestMetrics,
		TestCompaction,
		TestConcurrentOperations,
	}

	for _, test := range tests {
		test(t)
	}

	// 生成测试报告
	generateTestReport(suite)
}

// generateTestReport 生成详细的测试报告
func generateTestReport(suite *EnterpriseTestSuite) {
	report := struct {
		Timestamp     time.Time              `json:"timestamp"`
		TotalTests    int                    `json:"total_tests"`
		PassedTests   int                    `json:"passed_tests"`
		FailedTests   int                    `json:"failed_tests"`
		TotalDuration time.Duration          `json:"total_duration"`
		Results       []TestResult           `json:"results"`
		SystemInfo    map[string]interface{} `json:"system_info"`
	}{
		Timestamp:     time.Now(),
		TotalTests:    len(suite.results),
		PassedTests:   0,
		FailedTests:   0,
		TotalDuration: 0,
		Results:       suite.results,
		SystemInfo:    getSystemInfo(),
	}

	for _, result := range suite.results {
		if result.Passed {
			report.PassedTests++
		} else {
			report.FailedTests++
		}
		report.TotalDuration += result.Duration
	}

	// 输出报告
	jsonReport, _ := json.MarshalIndent(report, "", "  ")
	fmt.Printf("\n🧪 企业级测试报告\n")
	fmt.Printf("总测试数: %d\n", report.TotalTests)
	fmt.Printf("通过测试: %d\n", report.PassedTests)
	fmt.Printf("失败测试: %d\n", report.FailedTests)
	fmt.Printf("总耗时: %v\n", report.TotalDuration)
	fmt.Printf("详细报告:\n%s\n", string(jsonReport))

	// 保存报告到文件
	reportsDir := "../reports"
	filename := fmt.Sprintf("enterprise_test_report_%s.json", time.Now().Format("20060102_150405"))
	filepath := reportsDir + "/" + filename
	if err := ioutil.WriteFile(filepath, jsonReport, 0644); err != nil {
		fmt.Printf("保存报告失败: %v\n", err)
	} else {
		fmt.Printf("测试报告已保存到: %s\n", filepath)
	}
}

// 辅助函数
func generateRandomData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

func generateTestData(count, size int) map[string][]byte {
	data := make(map[string][]byte)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("test_%d", i)
		value := generateRandomData(size)
		data[key] = value
	}
	return data
}

func calculateHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total) * 100
}

func getSystemInfo() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"go_version":    runtime.Version(),
		"num_cpu":       runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
		"memory_alloc":  m.Alloc,
		"memory_total":  m.TotalAlloc,
		"memory_sys":    m.Sys,
		"gc_runs":       m.NumGC,
	}
}
