package benchmark

import (
	"fmt"
	"testing"

	gojidb "GojiDB"
)

// BenchmarkSmallValue 基准测试小值存储
func BenchmarkSmallValue(b *testing.B) {
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
		key := fmt.Sprintf("small_%d", i)
		value := []byte("small")
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMediumValue 基准测试中值存储
func BenchmarkMediumValue(b *testing.B) {
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

	mediumValue := make([]byte, 1024) // 1KB
	for i := range mediumValue {
		mediumValue[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("medium_%d", i)
		if err := db.Put(key, mediumValue); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLargeValue 基准测试大值存储
func BenchmarkLargeValue(b *testing.B) {
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

	largeValue := make([]byte, 1024*10) // 10KB
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("large_%d", i)
		if err := db.Put(key, largeValue); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVariableValue 基准测试可变大小值存储
func BenchmarkVariableValue(b *testing.B) {
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
		key := fmt.Sprintf("var_%d", i)

		// 创建不同大小的值
		var value []byte
		switch i % 4 {
		case 0:
			value = []byte("small")
		case 1:
			value = make([]byte, 256)
		case 2:
			value = make([]byte, 1024)
		case 3:
			value = make([]byte, 4096)
		}

		for j := range value {
			value[j] = byte(j % 256)
		}

		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}
