# GojiDB - High-Performance Go Key-Value Database

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)](https://github.com/741598236/GojiDB/actions)

[📖 English README](README_EN.md) | [📚 中文文档](README.md)

**GojiDB** is a high-performance, lightweight key-value database designed for modern applications. Built on LSM-tree architecture with WAL pre-write logging, TTL expiration management, intelligent compression algorithms, and enterprise-grade features. The latest version fixes WAL stability issues and provides production-grade reliability.

### Go Module Usage
```bash
go get github.com/741598236/GojiDB
```

## 🌟 Features

### Core Features

- **WAL Pre-write Logging**: Crash-safe data persistence with transaction recovery
- **High-Performance Storage**: LSM-tree architecture optimized for write performance
- **TTL Support**: Automatic key expiration with customizable TTL times
- **Intelligent Compression**: Support for Snappy, Zstd, Gzip algorithms with automatic optimal selection
- **Batch Operations**: Efficient batch read/write APIs with complete missing key handling
- **Snapshot Mechanism**: Support for data snapshots and recovery
- **Memory Pool**: Reduce GC pressure and improve performance
- **Concurrent Safety**: Thread-safe design based on segmented locks with 0% concurrency error rate

### Initial Release (v1.0.0)

- ✅ **Core Features Complete**: Complete key-value database basic functionality
- ✅ **Data Serialization**: Reliable data storage and retrieval mechanisms
- ✅ **Batch Operations**: Complete batch APIs with missing key handling
- ✅ **TTL Support**: Millisecond-precision expiration time management
- ✅ **Concurrent Safety**: Verified thread-safe design
- ✅ **Memory Safety**: Zero memory leak validation
- ✅ **Complete Tests**: 100% test pass rate

## 🚀 Quick Start

### Installation

```bash
# Clone the project
git clone https://github.com/741598236/GojiDB.git
cd GojiDB

# Install dependencies
go mod tidy

# Build
go build -o gojidbv2 .

# Run test validation
go test -v ./test
```

### Basic Usage

#### 1. Programming API

```go
package main

import (
    "log"
    "time"
    "github.com/741598236/GojiDB"
)

func main() {
    // Create database instance
    db, err := GojiDB.Open("./data", &GojiDB.Config{
        DataPath:         "./data",
        TTLCheckInterval: 1 * time.Minute,
        EnableMetrics:    true,
        CompressionType:  GojiDB.CompressionSnappy,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Basic operations
    err = db.Put("key", []byte("value"))
    if err != nil {
        log.Fatal(err)
    }

    value, err := db.Get("key")
    if err != nil {
        log.Fatal(err)
    }
    println(string(value))

    // Batch operations - with missing key handling
    keys := []string{"existing1", "missing1", "existing2", "missing2"}
    results, err := db.BatchGet(keys)
    if err != nil {
        log.Fatal(err)
    }
    
    for key, value := range results {
        if len(value) == 0 {
            println(key, "does not exist")
        } else {
            println(key, "=", string(value))
        }
    }

    // TTL operations
    err = db.PutWithTTL("temp", []byte("temporary"), 5*time.Minute)
    if err != nil {
        log.Fatal(err)
    }
}
```

#### 2. Run CLI (if CLI functionality is implemented)

```bash
./gojidbv2
```

## 📊 Test Results & Performance Report

### 🏆 Complete Performance Benchmark (2025-01-19)

We have completed comprehensive performance benchmark testing. All tests passed successfully with a 20-minute timeout setting, covering basic operations, concurrency, transactions, compression, TTL, and comprehensive scenarios.

#### 🚀 Core Performance Metrics

| Operation Type | Performance(ops/sec) | Latency(ns/op) | Memory Usage(B/op) | Performance Rating |
|----------------|----------------------|----------------|--------------------|------------------|
| **Write(Put)** | 115,267 | 8,671 ns | 634 B | 🟢 Excellent |
| **Read(Get)** | 2,147,720 | 466.3 ns | 258 B | 🟢 Excellent |
| **Delete** | 356,517 | 2,805 ns | 248 B | 🟢 Excellent |
| **PutWithTTL** | 143,658 | 8,707 ns | 632 B | 🟢 Excellent |

### 📦 Batch Operations Performance

| Batch Operation Type | Performance(ops/sec) | Latency(ns/op) | Memory Usage(B/op) | Performance Rating |
|----------------------|----------------------|----------------|--------------------|------------------|
| **Batch Write(100 items)** | 2,805 | 359,575 ns | 37,421 B | 🟡 Medium |
| **Batch Read(1000 items)** | 377 | 2,646,712 ns | 114,436 B | 🟡 Needs Optimization |

### 🔥 Concurrent Performance Highlights 

| Concurrent Test Type | Performance(ops/sec) | Latency(ns/op) | Memory Usage(B/op) | Concurrency Rating |
|----------------------|----------------------|----------------|--------------------|------------------|
| **Concurrent Writes** | 83,122 | 12,022 ns | 630 B | 🟢 High |
| **Concurrent Reads** | 497,317 | 2,557 ns | 1,325 B | 🟢 Excellent |

### 🗄️ Storage & Compression Performance

| Storage Related Tests | Performance(ops/sec) | Latency(ns/op) | Memory Usage(B/op) | Efficiency Rating |
|-----------------------|----------------------|----------------|--------------------|------------------|
| **File Rotation** | 86 | 11,632,434 ns | 670,197 B | 🟡 Needs Optimization |
| **Compression Recovery** | 4,227 | 236,366 ns | 236,838 B | 🟡 Medium |

### 📈 Transaction Processing Performance

| Transaction Operation Type | Performance(ops/sec) | Latency(ns/op) | Memory Usage(B/op) | Transaction Rating |
|----------------------------|----------------------|----------------|--------------------|------------------|
| **Transaction Write** | 106,877 | 9,307 ns | 822 B | 🟢 Excellent |
| **Transaction Batch** | 1,452 | 912,089 ns | 82,567 B | 🟡 Medium |
| **Transaction Rollback** | 486,193 | 2,056 ns | 2,435 B | 🟢 Ultra Fast |

### 🎯 Performance Summary & Optimization Recommendations

#### ✅ Strength Areas
- **Read Performance**: 466ns latency, achieving microsecond-level response
- **Concurrent Reads**: 497K ops/sec, extremely high concurrent processing capability
- **Delete Operations**: 2.8μs latency, fast cleanup capability
- **Transaction Rollback**: 2μs latency, almost zero overhead

#### ⚠️ Areas for Optimization
- **Batch Reads**: 377 ops/sec, 1000 items require 2.6ms, still room for improvement
- **File Rotation**: 86 ops/sec, large file processing is slow
- **Batch Writes**: 2.8K ops/sec, batch operation efficiency is moderate

#### 🔧 Optimization Achievements
- **Batch Read Optimization**: Improved from 369 ops/sec to 377 ops/sec (+2.2%)
- **Memory Usage Optimization**: Batch read memory usage reduced from 205KB to 114KB (-44%)
- **Cache Hit Rate**: Improved hot data access efficiency through cache-first strategy

### 💡 Usage Recommendations

**High-Concurrency Read Scenarios**: GojiDB performs excellently in concurrent reads, suitable for read-heavy applications
**Transaction Processing**: Transaction overhead is extremely low, feel free to use ACID features
**Batch Operations**: For large batch operations, recommend processing in batches for better performance
- **Usable Monitoring**: Real-time monitoring overhead is acceptable

### Latest Test Validation (2025-01-19)

All tests now pass:
- ✅ **TestBatchGetWithMissingKeys** - Batch processing with missing key handling
- ✅ **TestCachePersistence** - Cache persistence validation
- ✅ **TestCacheDisabledVsEnabledPerformance** - Cache performance comparison
- ✅ **TestTTLBasic** - TTL basic functionality
- ✅ **TestTTLNegative** - TTL negative value handling
- ✅ **All batch operations tests** - 100% pass rate
- ✅ **Complete benchmark tests** - 100% pass rate with 20-minute timeout

### Data Integrity Validation

- **1-byte data**: 100% data integrity ✅
- **1KB data**: Complete serialization/deserialization validation ✅
- **1MB data**: Large object handling validation ✅
- **100MB data**: Extra-large object handling validation ✅

### Concurrent Safety Validation

- **Concurrent read/write**: 1000 concurrent operations, 0% error rate ✅
- **Memory safety**: Zero memory leak validation ✅
- **Boundary tests**: Extreme data scenarios 100% pass ✅

## 🏗️ Project Structure

```
GojiDB/
├── .gitignore          # Git ignore configuration
├── LICENSE            # MIT license
├── README.md          # Project documentation
├── README_EN.md       # English documentation
├── go.mod            # Go module definition
├── go.sum            # Dependency version lock
├── db.go             # Core database implementation
├── entry.go          # Data entry definition and serialization
├── cache.go          # Multi-layer cache system
├── ttl.go            # TTL expiration management
├── wal.go            # Pre-write log implementation
├── config.go         # Configuration management
├── metrics.go        # Performance monitoring
├── transaction.go    # Transaction support
├── concurrent.go     # Concurrent safety implementation
├── cli.go            # Command-line interface
├── main/             # Main program entry
├── test/             # Test files (git ignored)
├── data/             # Data directory (git ignored)
├── logs/             # Log directory (git ignored)
└── benchmark/        # Performance benchmark tests
```

## 🔧 Configuration Options

### Basic Configuration

```go
type Config struct {
    DataPath         string        // Data storage path
    MaxFileSize      int64         // Maximum file size (default 64MB)
    SyncWrites       bool          // Synchronous writes (default true)
    TTLCheckInterval time.Duration // TTL check interval (default 1 minute)
    EnableMetrics    bool          // Enable monitoring statistics
    CompressionType  int           // Compression algorithm type
}
```

### Usage Example

```go
config := &GojiDB.Config{
    DataPath:         "./mydb",
    MaxFileSize:      64 * 1024 * 1024, // 64MB
    SyncWrites:       true,
    TTLCheckInterval: 30 * time.Second,
    EnableMetrics:    true,
    CompressionType:  GojiDB.CompressionSnappy,
}
```

## 🧪 Running Tests

### Run All Tests

```bash
go test -v ./test
```

### Run Specific Tests

```bash
# Run batch tests
go test -v -run TestBatch ./test

# Run TTL tests
go test -v -run TestTTL ./test

# Run cache tests
go test -v -run TestCache ./test
```

### Performance Benchmark Tests

```bash
cd benchmark
go test -bench=. -benchmem
```

## 📈 Core Features Explained

### 1. Data Serialization & Integrity

- **Binary Serialization**: Efficient and compact data format
- **CRC Validation**: Data integrity verification
- **Boundary Checking**: Prevent buffer overflow
- **Memory Safety**: Zero-copy optimization

### 2. Batch Operations

- **BatchGet**: Support for returning empty values for missing keys
- **BatchPut**: Atomic batch writes
- **BatchDelete**: Batch delete operations
- **Consistency Guarantee**: All succeed or all fail

### 3. TTL Expiration System

- **Precise Expiration**: Millisecond-precision TTL
- **Background Cleanup**: Asynchronous expired key cleanup
- **Lazy Deletion**: TTL checking during reads
- **Memory Optimization**: Minimal memory footprint

### 4. Cache System

- **Multi-level Cache**: Key cache + block cache
- **LRU Eviction**: Intelligent cache management
- **Concurrent Safety**: Segmented lock design
- **Performance Monitoring**: Hit rate statistics

## 🔒 Security & Reliability

### Data Security

- **Crash Recovery**: Automatic WAL replay
- **Transaction Safety**: ACID characteristics guarantee
- **Data Validation**: CRC integrity verification
- **Backup Support**: Snapshot mechanism

### Concurrent Safety

- **Thread Safety**: Lock-based concurrency control
- **Memory Barrier**: Prevent data races
- **Deadlock Prevention**: Intelligent lock management
- **Performance Optimization**: Minimize lock contention

## 🤝 Contribution Guidelines

### Development Environment

1. Clone the project
2. Run tests to ensure environment is normal
3. Create feature branch
4. Run complete tests before submission

### Code Standards

- Follow Go code standards
- Add appropriate test cases
- Update relevant documentation
- Ensure all tests pass

## 📄 License

This project uses MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Thanks to the Go community for excellent tools and libraries
- Thanks to LSM-tree research pioneers
- Thanks to all testers and contributors

---

**GojiDB** - A battle-tested high-performance key-value database designed for modern applications