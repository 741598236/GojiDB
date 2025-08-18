package benchmark

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	gojidb "GojiDB"
)

// 压缩和存储优化基准测试套件

// ==================== 压缩算法测试 ====================

// BenchmarkCompressionNone 基准测试无压缩性能
func BenchmarkCompressionNone(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:        tempDir,
		MaxFileSize:     64 * 1024 * 1024,
		SyncWrites:      false,
		CompressionType: gojidb.NoCompression,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	value := generateTestData(1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("no_compress_%d", i)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompressionSnappy 基准测试Snappy压缩性能
func BenchmarkCompressionSnappy(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:        tempDir,
		MaxFileSize:     64 * 1024 * 1024,
		SyncWrites:      false,
		CompressionType: gojidb.SnappyCompression,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	value := generateTestData(1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("snappy_%d", i)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompressionMixed 基准测试混合压缩类型性能
func BenchmarkCompressionMixed(b *testing.B) {
	tempDir := b.TempDir()
	
	// 测试不同压缩类型的混合使用
	compressionTypes := []gojidb.CompressionType{
		gojidb.NoCompression,
		gojidb.SnappyCompression,
		gojidb.GzipCompression,
	}

	for _, compressionType := range compressionTypes {
		b.Run(fmt.Sprintf("Type_%d", compressionType), func(b *testing.B) {
			db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
				DataPath:        tempDir,
				MaxFileSize:     64 * 1024 * 1024,
				SyncWrites:      false,
				CompressionType: compressionType,
			})
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()

			value := generateTestData(1024) // 1KB

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("mixed_%d_%d", compressionType, i)
				if err := db.Put(key, value); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCompressionGzip 基准测试Gzip压缩性能
func BenchmarkCompressionGzip(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:        tempDir,
		MaxFileSize:     64 * 1024 * 1024,
		SyncWrites:      false,
		CompressionType: gojidb.GzipCompression,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	value := generateTestData(1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("gzip_%d", i)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompressionZSTD 基准测试ZSTD压缩性能
func BenchmarkCompressionZSTD(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:        tempDir,
		MaxFileSize:     64 * 1024 * 1024,
		SyncWrites:      false,
		CompressionType: gojidb.ZSTDCompression,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	value := generateTestData(1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("zstd_%d", i)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 压缩效率测试 ====================

// BenchmarkCompressionRatio 基准测试不同数据类型的压缩率
func BenchmarkCompressionRatio(b *testing.B) {
	testCases := []struct {
		name     string
		dataSize int
		dataType string
	}{
		{"SmallText", 256, "text"},
		{"MediumText", 1024, "text"},
		{"LargeText", 4096, "text"},
		{"SmallJSON", 512, "json"},
		{"MediumJSON", 2048, "json"},
		{"LargeJSON", 8192, "json"},
		{"Binary", 1024, "binary"},
		{"RepeatedPattern", 2048, "pattern"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			tempDir := b.TempDir()
			db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
				DataPath:        tempDir,
				MaxFileSize:     64 * 1024 * 1024,
				SyncWrites:      false,
				CompressionType: gojidb.SnappyCompression,
			})
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()

			data := generateDataByType(tc.dataSize, tc.dataType)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("compress_%s_%d", tc.name, i)
				if err := db.Put(key, data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ==================== 智能压缩测试 ====================

// BenchmarkSmartCompression 基准测试智能压缩决策
func BenchmarkSmartCompression(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:           tempDir,
		MaxFileSize:        64 * 1024 * 1024,
		SyncWrites:         false,
		CompressionType:    gojidb.SnappyCompression,
		SmartCompression:   gojidb.DefaultSmartCompressionConfig(),
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 混合不同类型和大小的数据
	dataTypes := []string{"text", "json", "binary", "pattern"}
	sizes := []int{64, 256, 1024, 4096}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dataType := dataTypes[i%len(dataTypes)]
		size := sizes[i%len(sizes)]
		
		data := generateDataByType(size, dataType)
		key := fmt.Sprintf("smart_%s_%d", dataType, i)
		
		if err := db.Put(key, data); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 存储优化测试 ====================

// BenchmarkFileRotation 基准测试文件轮转性能
func BenchmarkFileRotation(b *testing.B) {
	tempDir := b.TempDir()
	smallFileSize := 1024 * 1024 // 1MB 强制频繁轮转
	
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:    tempDir,
		MaxFileSize: int64(smallFileSize),
		SyncWrites:  false,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	largeValue := generateTestData(512 * 1024) // 512KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("rotation_%d", i)
		if err := db.Put(key, largeValue); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompaction 基准测试压缩清理性能
func BenchmarkCompaction(b *testing.B) {
	tempDir := b.TempDir()
	db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
		DataPath:             tempDir,
		MaxFileSize:          1024 * 1024,
		SyncWrites:           false,
		CompactionThreshold:  0.3, // 低阈值触发压缩
	})
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 先写入大量数据
	b.StopTimer()
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("compact_%d", i)
		value := generateTestData(1024)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	// 更新和删除操作触发压缩
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("compact_%d", i)
		if i%2 == 0 {
			value := generateTestData(1024)
			if err := db.Put(key, value); err != nil {
				b.Fatal(err)
			}
		} else {
			if err := db.Delete(key); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.StartTimer()

	// 继续写入以触发压缩
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("compact_new_%d", i)
		value := generateTestData(512)
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 批量压缩测试 ====================

// BenchmarkBatchCompression 基准测试批量压缩操作
func BenchmarkBatchCompression(b *testing.B) {
	testCases := []struct {
		name     string
		batchSize int
		dataSize  int
	}{
		{"SmallBatch", 10, 256},
		{"MediumBatch", 100, 1024},
		{"LargeBatch", 1000, 4096},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			tempDir := b.TempDir()
			db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
				DataPath:        tempDir,
				MaxFileSize:     128 * 1024 * 1024,
				SyncWrites:      false,
				CompressionType: gojidb.ZSTDCompression,
			})
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				batch := make(map[string][]byte)
				for j := 0; j < tc.batchSize; j++ {
					key := fmt.Sprintf("batch_%s_%d_%d", tc.name, i, j)
					value := generateTestData(tc.dataSize)
					batch[key] = value
				}
				
				if err := db.BatchPut(batch); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ==================== 压缩恢复测试 ====================

// BenchmarkCompressionRecovery 基准测试压缩数据的恢复性能
func BenchmarkCompressionRecovery(b *testing.B) {
	tempDir := b.TempDir()
	
	// 预填充压缩数据
	{
		db, err := gojidb.NewGojiDB(&gojidb.GojiDBConfig{
			DataPath:        tempDir,
			MaxFileSize:     32 * 1024 * 1024,
			SyncWrites:      true,
			CompressionType: gojidb.SnappyCompression,
		})
		if err != nil {
			b.Fatal(err)
		}
		
		for i := 0; i < 5000; i++ {
			key := fmt.Sprintf("recovery_%d", i)
			value := generateTestData(1024 + i%4096)
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
			DataPath:        tempDir,
			MaxFileSize:     32 * 1024 * 1024,
			SyncWrites:      false,
			CompressionType: gojidb.SnappyCompression,
		})
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		
		// 读取压缩数据
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("recovery_%d", j)
			_, _ = db.Get(key)
		}
		
		b.StopTimer()
		db.Close()
	}
}

// ==================== 工具函数 ====================

// generateTestData 生成测试数据
func generateTestData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

// generateDataByType 根据类型生成特定测试数据
func generateDataByType(size int, dataType string) []byte {
	switch dataType {
	case "text":
		return generateEnglishText(size)
	case "json":
		return generateJSONData(size)
	case "binary":
		return generateBinaryData(size)
	case "pattern":
		return generatePatternData(size)
	default:
		return generateTestData(size)
	}
}

// generateEnglishText 生成英文文本
func generateEnglishText(size int) []byte {
	words := []string{"hello", "world", "database", "performance", "compression", "optimization", "benchmark", "testing"}
	result := make([]byte, 0, size)
	
	// 避免无限循环，确保能够生成数据
	if size <= 0 {
		return []byte{}
	}
	
	for len(result) < size {
		word := words[rand.Intn(len(words))]
		// 如果单词太大，截取它
		if len(word) > size-len(result) {
			word = word[:size-len(result)]
		}
		if len(word) > 0 {
			result = append(result, word...)
			// 只在还有空间时添加空格
			if len(result) < size {
				result = append(result, ' ')
			}
		}
		// 防止死循环：如果已经填满或无法添加更多内容
		if len(result) >= size || (len(result) == 0 && len(word) == 0) {
			break
		}
	}
	
	// 确保结果正好是所需大小
	if len(result) > size {
		return result[:size]
	}
	
	// 如果不足，用空格填充
	for len(result) < size {
		result = append(result, ' ')
	}
	
	return result
}

// generateJSONData 生成JSON格式数据
func generateJSONData(size int) []byte {
	if size <= 0 {
		return []byte{}
	}
	
	template := `{"id":%d,"name":"user%d","email":"user%d@example.com","data":"%s","timestamp":%d}`
	
	// 为模板预留足够空间
	minTemplateSize := 50 // 模板最小长度
	if size < minTemplateSize {
		// 如果空间太小，生成简化JSON
		simple := fmt.Sprintf(`{"id":%d}`, rand.Intn(100))
		if len(simple) > size {
			return []byte(`{}`)[:size]
		}
		result := make([]byte, size)
		copy(result, simple)
		for i := len(simple); i < size; i++ {
			result[i] = ' '
		}
		return result
	}
	
	paddingSize := (size - len(template)) / 2
	if paddingSize < 0 {
		paddingSize = 0
	}
	
	padding := make([]byte, paddingSize)
	for i := range padding {
		padding[i] = 'x'
	}
	
	jsonStr := fmt.Sprintf(template, rand.Intn(10000), rand.Intn(1000), rand.Intn(1000), string(padding), time.Now().Unix())
	
	// 确保结果正好是所需大小
	result := make([]byte, size)
	if len(jsonStr) >= size {
		copy(result, jsonStr[:size])
	} else {
		copy(result, jsonStr)
		// 填充剩余空间
		for i := len(jsonStr); i < size; i++ {
			result[i] = ' '
		}
	}
	
	return result
}

// generateBinaryData 生成二进制数据
func generateBinaryData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return data
}

// generatePatternData 生成重复模式数据
func generatePatternData(size int) []byte {
	pattern := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	data := make([]byte, size)
	for i := range data {
		data[i] = pattern[i%len(pattern)]
	}
	return data
}