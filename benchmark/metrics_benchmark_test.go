package benchmark

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	gojidb "GojiDB"
)

// 监控和指标基准测试套件

// ==================== 基础指标测试 ====================

// BenchmarkMetricsEnabled 基准测试启用指标收集的性能开销
func BenchmarkMetricsEnabled(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("metrics_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMetricsDisabled 基准测试禁用指标收集的性能
func BenchmarkMetricsDisabled(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("no_metrics_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 内存监控测试 ====================

// BenchmarkMemoryTracking 基准测试内存使用跟踪
func BenchmarkMemoryTracking(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   64 * 1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	var m runtime.MemStats

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		runtime.ReadMemStats(&m)
		before := m.Alloc
		b.StartTimer()

		key := fmt.Sprintf("mem_track_%d", i)
		value := make([]byte, 1024)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		runtime.ReadMemStats(&m)
		after := m.Alloc
		_ = after - before // 使用内存差值
		b.StartTimer()
	}
}

// ==================== 性能计数器测试 ====================

// BenchmarkPerformanceCounters 基准测试性能计数器
func BenchmarkPerformanceCounters(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 测试各种操作的计数器
	b.Run("WriteCounter", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("write_count_%d", i)
			value := []byte("test")
			_ = db.Put(key, value)
		}
	})

	b.Run("ReadCounter", func(b *testing.B) {
		// 预填充数据
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("read_count_%d", i)
			_ = db.Put(key, []byte("test"))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("read_count_%d", i%1000)
			_, _ = db.Get(key)
		}
	})

	b.Run("DeleteCounter", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("delete_count_%d", i)
			_ = db.Put(key, []byte("test"))
			_ = db.Delete(key)
		}
	})
}

// ==================== 实时监控测试 ====================

// BenchmarkRealTimeMonitoring 基准测试实时监控性能
func BenchmarkRealTimeMonitoring(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 预填充一些数据
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("pre_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	// 测试基本的Put/Get操作性能
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("monitor_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		
		// 主要测试Put操作性能
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
		
		// 测试Get操作
		if i%2 == 0 {
			_, err := db.Get(key)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// ==================== 资源使用测试 ====================

// BenchmarkResourceUsage 基准测试资源使用统计
func BenchmarkResourceUsage(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   64 * 1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 执行操作
		key := fmt.Sprintf("resource_%d", i)
		value := make([]byte, 1024)
		_ = db.Put(key, value)

		// 获取资源使用情况
			if i%100 == 0 {
				_ = db.GetMetrics()
			}
	}
}

// ==================== 延迟监控测试 ====================

// BenchmarkLatencyTracking 基准测试延迟跟踪
func BenchmarkLatencyTracking(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()

		key := fmt.Sprintf("latency_%d", i)
		value := []byte("test")
		_ = db.Put(key, value)

		latency := time.Since(start)
		_ = latency // 使用延迟数据
	}
}

// ==================== 并发监控测试 ====================

// BenchmarkConcurrentMetrics 基准测试并发指标收集
func BenchmarkConcurrentMetrics(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			// 混合操作同时收集指标
			switch id % 4 {
			case 0:
				key := fmt.Sprintf("concurrent_write_%d", id)
				value := []byte("test")
				_ = db.Put(key, value)
			case 1:
				key := fmt.Sprintf("concurrent_read_%d", id%1000)
				_, _ = db.Get(key)
			case 2:
				key := fmt.Sprintf("concurrent_delete_%d", id)
				_ = db.Delete(key)
			case 3:
				// 查询指标
				_ = db.GetMetrics()
			}
			id++
		}
	})
}

// ==================== 内存泄漏检测测试 ====================

// BenchmarkMemoryLeakDetection 基准测试内存泄漏检测
func BenchmarkMemoryLeakDetection(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   64 * 1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	initialAlloc := m.Alloc

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("leak_test_%d", i)
		value := make([]byte, 1024)

		_ = db.Put(key, value)
		retrieved, _ := db.Get(key)
		_ = db.Delete(key)

		// 强制GC以检测泄漏
		if i%1000 == 0 {
			runtime.GC()
		}

		_ = retrieved // 使用返回值
	}

	b.StopTimer()
	runtime.GC()
	runtime.ReadMemStats(&m)
	finalAlloc := m.Alloc

	// 检查内存增长
	if finalAlloc > initialAlloc+uint64(b.N*1024*2) {
		b.Logf("Potential memory leak detected: %d bytes", finalAlloc-initialAlloc)
	}
}

// ==================== 吞吐量测试 ====================

// BenchmarkThroughputMetrics 基准测试吞吐量指标
func BenchmarkThroughputMetrics(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	start := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("throughput_%d", i)
		value := []byte(fmt.Sprintf("value_%d", i))
		_ = db.Put(key, value)

		// 每1000次操作计算吞吐量
		if i > 0 && i%1000 == 0 {
			elapsed := time.Since(start)
			opsPerSec := float64(i) / elapsed.Seconds()
			_ = opsPerSec // 使用吞吐量数据
		}
	}
}

// ==================== 错误监控测试 ====================

// BenchmarkErrorTracking 基准测试错误跟踪性能
func BenchmarkErrorTracking(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 故意触发错误
		_, _ = db.Get("non_existent_key")

		// 记录错误指标
		if i%100 == 0 {
			_ = db.GetMetrics()
		}
	}
}

// ==================== 系统资源监控 ====================

// BenchmarkSystemResourceMonitoring 基准测试系统资源监控
func BenchmarkSystemResourceMonitoring(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:      tempDir,
		MaxFileSize:   1024 * 1024,
		SyncWrites:    false,
		EnableMetrics: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 执行操作
		key := fmt.Sprintf("sys_resource_%d", i)
		value := []byte("test")
		_ = db.Put(key, value)

		// 监控系统资源
		if i%500 == 0 {
			_ = db.GetMetrics()
		}
	}
}
