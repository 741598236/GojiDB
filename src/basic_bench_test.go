package main

import (
	"testing"
	"time"
)

func BenchmarkPut(b *testing.B) {
	tempDir := b.TempDir()
	db, err := NewGojiDB(&GojiDBConfig{
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
		key := string(rune(i))
		value := []byte("test_value_" + string(rune(i)))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGet(b *testing.B) {
	tempDir := b.TempDir()
	db, err := NewGojiDB(&GojiDBConfig{
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
		key := string(rune(i))
		value := []byte("test_value_" + string(rune(i)))
		if err := db.Put(key, value); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i % 1000))
		_, err := db.Get(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchPut(b *testing.B) {
	tempDir := b.TempDir()
	db, err := NewGojiDB(&GojiDBConfig{
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
			key := string(rune(i*100 + j))
			value := []byte("test_value_" + string(rune(i*100+j)))
			items[key] = value
		}
		if err := db.BatchPut(items); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPutWithTTL(b *testing.B) {
	tempDir := b.TempDir()
	db, err := NewGojiDB(&GojiDBConfig{
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
		key := string(rune(i))
		value := []byte("test_value_" + string(rune(i)))
		if err := db.PutWithTTL(key, value, time.Minute); err != nil {
			b.Fatal(err)
		}
	}
}