package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestWALAuditChain WAL审计链全面测试
func TestWALAuditChain(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"BasicAudit", testWALBasicAudit},
		{"RecoveryAudit", testWALRecoveryAudit},
		{"TransactionAudit", testWALTransactionAudit},
		{"ConcurrentAudit", testWALConcurrentAudit},
		{"IntegrityAudit", testWALIntegrityAudit},
		{"PerformanceAudit", testWALPerformanceAudit},
		{"EdgeCaseAudit", testWALEdgeCaseAudit},
		{"ConfigurationAudit", testWALConfigurationAudit},
		{"StressAudit", testWALStressAudit},
		{"CleanupAudit", testWALCleanupAudit},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

// 基础审计测试
func testWALBasicAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	// 验证WAL管理器初始化
	if db.walManager == nil {
		t.Fatal("WAL管理器未初始化")
	}

	// 基础写入审计
	auditOperations := []struct {
		key   string
		value string
		op    string
	}{
		{"audit1", "value1", "PUT"},
		{"audit2", "value2", "PUT"},
		{"audit1", "", "DELETE"},
		{"audit3", "value3", "PUT"},
	}

	for _, op := range auditOperations {
		switch op.op {
		case "PUT":
			if err := db.Put(op.key, []byte(op.value)); err != nil {
				t.Errorf("审计写入失败 %s: %v", op.key, err)
			}
		case "DELETE":
			if err := db.Delete(op.key); err != nil {
				t.Errorf("审计删除失败 %s: %v", op.key, err)
			}
		}
	}

	// 验证WAL文件存在
	walPath := filepath.Join(tempDir, "wal.log")
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Error("WAL文件未创建")
	}

	// 验证文件大小
	size, err := db.walManager.GetWALSize()
	if err != nil {
		t.Errorf("获取WAL大小失败: %v", err)
	}
	if size == 0 {
		t.Error("WAL文件为空")
	}
}

// 恢复审计测试
func testWALRecoveryAudit(t *testing.T) {
	tempDir := setupTestDir(t)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}

	// 写入审计数据
	auditData := map[string]string{
		"recover_audit1": "audit_data_1",
		"recover_audit2": "audit_data_2",
		"recover_audit3": "audit_data_3",
	}

	for k, v := range auditData {
		if err := db.Put(k, []byte(v)); err != nil {
			t.Errorf("审计写入失败 %s: %v", k, err)
		}
	}

	// 强制关闭模拟崩溃
	db.Close()

	// 重新打开验证恢复
	db2, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("恢复数据库失败: %v", err)
	}
	defer db2.Close()

	// 验证所有审计数据
	for k, expected := range auditData {
		value, err := db2.Get(k)
		if err != nil {
			t.Errorf("恢复后审计数据丢失 %s: %v", k, err)
			continue
		}
		if string(value) != expected {
			t.Errorf("审计数据不一致 %s: 期望 %s, 实际 %s", k, expected, string(value))
		}
	}

	// 验证WAL文件完整性
	walPath := filepath.Join(tempDir, "wal.log")
	if info, err := os.Stat(walPath); err == nil {
		if info.Size() == 0 {
			t.Error("恢复后WAL文件被清空")
		}
	}
}

// 事务审计测试
func testWALTransactionAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	// 测试事务审计链
	tests := []struct {
		name     string
		ops      []TransactionOp
		expected map[string]string
	}{
		{
			name: "简单事务",
			ops: []TransactionOp{
				{Type: "PUT", Key: "tx_audit1", Value: []byte("tx_value1")},
				{Type: "PUT", Key: "tx_audit2", Value: []byte("tx_value2")},
			},
			expected: map[string]string{
				"tx_audit1": "tx_value1",
				"tx_audit2": "tx_value2",
			},
		},
		{
			name: "混合操作事务",
			ops: []TransactionOp{
				{Type: "PUT", Key: "mixed1", Value: []byte("initial")},
				{Type: "PUT", Key: "mixed2", Value: []byte("update")},
				{Type: "DELETE", Key: "mixed1"},
				{Type: "PUT", Key: "mixed3", Value: []byte("final")},
			},
			expected: map[string]string{
				"mixed2": "update",
				"mixed3": "final",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := db.BeginTransaction()
			if err != nil {
				t.Fatalf("开始事务失败: %v", err)
			}

			for _, op := range tt.ops {
				switch op.Type {
				case "PUT":
					tx.Put(op.Key, op.Value)
				case "DELETE":
					tx.Delete(op.Key)
				}
			}

			if err := tx.Commit(); err != nil {
				t.Fatalf("提交事务失败: %v", err)
			}

			// 验证事务结果
			for k, expected := range tt.expected {
				value, err := db.Get(k)
				if err != nil {
					t.Errorf("事务审计验证失败 %s: %v", k, err)
				}
				if string(value) != expected {
					t.Errorf("事务审计结果错误 %s: 期望 %s, 实际 %s", k, expected, string(value))
				}
			}
		})
	}
}

// 并发审计测试
func testWALConcurrentAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	const (
		workers    = 20
		operations = 50
	)

	var wg sync.WaitGroup
	errChan := make(chan error, workers)
	resultChan := make(chan map[string]string, workers)

	// 并发审计写入
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			results := make(map[string]string)
			
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("concurrent_audit_%d_%d", workerID, j)
				value := fmt.Sprintf("audit_value_%d_%d", workerID, j)
				
				if err := db.Put(key, []byte(value)); err != nil {
					errChan <- fmt.Errorf("并发审计写入失败 %s: %v", key, err)
					return
				}
				results[key] = value
			}
			
			resultChan <- results
		}(i)
	}

	wg.Wait()
	close(errChan)
	close(resultChan)

	// 检查错误
	for err := range errChan {
		t.Errorf("并发审计错误: %v", err)
	}

	// 验证所有并发结果
	allResults := make(map[string]string)
	for results := range resultChan {
		for k, v := range results {
			allResults[k] = v
		}
	}

	// 验证数据完整性
	for key, expected := range allResults {
		value, err := db.Get(key)
		if err != nil {
			t.Errorf("并发审计数据丢失 %s: %v", key, err)
			continue
		}
		if string(value) != expected {
			t.Errorf("并发审计数据错误 %s: 期望 %s, 实际 %s", key, expected, string(value))
		}
	}

	// 验证WAL文件大小
	size, err := db.walManager.GetWALSize()
	if err != nil {
		t.Errorf("获取并发审计WAL大小失败: %v", err)
	}
	if size == 0 {
		t.Error("并发审计后WAL文件为空")
	}
}

// 完整性审计测试
func testWALIntegrityAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	// 测试各种数据类型的完整性
	integrityTests := []struct {
		name  string
		key   string
		value []byte
		desc  string
	}{
		{"空值", "empty_audit", []byte{}, "空字节数组"},
		{"小数据", "small_audit", []byte("small audit data"), "小数据"},
		{"大数据", "large_audit", make([]byte, 5000), "大数据块"},
		{"特殊字符", "unicode_audit", []byte("🎯🔥⚡审计测试🚀"), "Unicode字符"},
		{"二进制", "binary_audit", []byte{0, 1, 255, 254, 128, 127}, "二进制数据"},
		{"JSON数据", "json_audit", []byte(`{"audit": "test", "timestamp": "2024-01-01"}`), "JSON字符串"},
	}

	// 填充大数据
	for i := range integrityTests[2].value {
		integrityTests[2].value[i] = byte(i % 256)
	}

	for _, test := range integrityTests {
		t.Run(test.name, func(t *testing.T) {
			// 写入审计数据
			if err := db.Put(test.key, test.value); err != nil {
				t.Errorf("完整性审计写入失败 %s: %v", test.desc, err)
				return
			}

			// 立即读取验证
			retrieved, err := db.Get(test.key)
			if err != nil {
				t.Errorf("完整性审计读取失败 %s: %v", test.desc, err)
				return
			}

			// 验证长度
			if len(retrieved) != len(test.value) {
				t.Errorf("完整性审计长度不匹配 %s: 期望 %d, 实际 %d", 
					test.desc, len(test.value), len(retrieved))
				return
			}

			// 验证内容
			for i := range retrieved {
				if retrieved[i] != test.value[i] {
					t.Errorf("完整性审计内容不匹配 %s: 位置 %d", test.desc, i)
					break
				}
			}
		})
	}

	// 模拟崩溃后验证
	db.Close()
	db2, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("重新打开数据库失败: %v", err)
	}
	defer db2.Close()

	// 验证所有完整性测试数据
	for _, test := range integrityTests {
		retrieved, err := db2.Get(test.key)
		if err != nil {
			t.Errorf("恢复后完整性审计数据丢失 %s: %v", test.desc, err)
			continue
		}
		if len(retrieved) != len(test.value) {
			t.Errorf("恢复后完整性审计长度错误 %s", test.desc)
		}
	}
}

// 性能审计测试
func testWALPerformanceAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	const operations = 10000

	// 写入性能审计
	start := time.Now()
	for i := 0; i < operations; i++ {
		key := fmt.Sprintf("perf_audit_%d", i)
		value := fmt.Sprintf("performance_audit_value_%d", i)
		if err := db.Put(key, []byte(value)); err != nil {
			t.Errorf("性能审计写入失败: %v", err)
		}
	}
	writeDuration := time.Since(start)

	// 读取性能审计
	start = time.Now()
	for i := 0; i < operations; i++ {
		key := fmt.Sprintf("perf_audit_%d", i)
		_, _ = db.Get(key)
	}
	readDuration := time.Since(start)

	// 计算性能指标
	writeTPS := float64(operations) / writeDuration.Seconds()
	readTPS := float64(operations) / readDuration.Seconds()

	t.Logf("性能审计结果:")
	t.Logf("写入操作: %d 次, 耗时: %v, TPS: %.2f", operations, writeDuration, writeTPS)
	t.Logf("读取操作: %d 次, 耗时: %v, TPS: %.2f", operations, readDuration, readTPS)

	// 验证WAL文件大小
	size, err := db.walManager.GetWALSize()
	if err != nil {
		t.Errorf("获取性能审计WAL大小失败: %v", err)
	}
	t.Logf("WAL文件大小: %d 字节", size)

	// 性能阈值检查
	if writeTPS < 100 {
		t.Errorf("写入性能过低: %.2f TPS < 100 TPS", writeTPS)
	}
	if readTPS < 1000 {
		t.Errorf("读取性能过低: %.2f TPS < 1000 TPS", readTPS)
	}
}

// 边界情况审计测试
func testWALEdgeCaseAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	// 边界情况测试
	edgeTests := []struct {
		name  string
		key   string
		value []byte
		valid bool
	}{
		{"空键", "", []byte("value"), false},
		{"特殊字符键", "key:with:colons", []byte("value"), true},
		{"空格键", "key with spaces", []byte("value"), true},
		{"点号键", "key.with.dots", []byte("value"), true},
		{"斜杠键", "key/with/slashes", []byte("value"), true},
		{"反斜杠键", "key\\with\\backslashes", []byte("value"), true},
		{"超长键", string(make([]byte, 1000)), []byte("value"), true},
		{"超长值", "normal_key", make([]byte, 10000), true},
	}

	for _, test := range edgeTests {
		t.Run(test.name, func(t *testing.T) {
			err := db.Put(test.key, test.value)
			if test.valid && err != nil {
				t.Errorf("边界审计期望成功但失败: %v", err)
			} else if !test.valid && err == nil {
				t.Errorf("边界审计期望失败但成功")
			}

			if test.valid && err == nil {
				// 验证边界数据
				retrieved, err := db.Get(test.key)
				if err != nil {
					t.Errorf("边界审计读取失败: %v", err)
				} else if len(retrieved) != len(test.value) {
					t.Errorf("边界审计长度不匹配")
				}
			}
		})
	}
}

// 配置审计测试
func testWALConfigurationAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	// 测试不同配置组合
	configs := []struct {
		name         string
		enableWAL    bool
		syncWrites   bool
		flushInterval time.Duration
		maxSize      int64
	}{
		{"默认配置", true, true, 100 * time.Millisecond, 32 * 1024 * 1024},
		{"禁用WAL", false, true, 100 * time.Millisecond, 32 * 1024 * 1024},
		{"异步写入", true, false, 1 * time.Second, 32 * 1024 * 1024},
		{"小文件", true, true, 50 * time.Millisecond, 1024 * 1024},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			config := &GojiDBConfig{
				DataPath:         tempDir,
				EnableWAL:        cfg.enableWAL,
				WALSyncWrites:    cfg.syncWrites,
				WALFlushInterval: cfg.flushInterval,
				WALMaxSize:       cfg.maxSize,
			}

			db, err := NewGojiDB(config)
			if err != nil {
				t.Fatalf("创建配置审计数据库失败: %v", err)
			}
			defer db.Close()

			// 验证配置应用
			if cfg.enableWAL && db.walManager == nil {
				t.Error("WAL应该被启用")
			}
			if !cfg.enableWAL && db.walManager != nil {
				t.Error("WAL应该被禁用")
			}

			// 测试基本操作
			if cfg.enableWAL {
				if err := db.Put("config_test", []byte("value")); err != nil {
					t.Errorf("配置审计写入失败: %v", err)
				}
			}
		})
	}
}

// 压力审计测试
func testWALStressAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	const (
		workers    = 30
		operations = 100
	)

	var wg sync.WaitGroup
	start := time.Now()

	// 混合操作压力测试
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("stress_audit_%d_%d", workerID, j)
				
				switch j % 4 {
				case 0:
					// 写入
					value := fmt.Sprintf("stress_value_%d_%d", workerID, j)
					db.Put(key, []byte(value))
				case 1:
					// 读取
					db.Get(key)
				case 2:
					// 删除
					db.Delete(key)
				case 3:
					// 事务
					tx, _ := db.BeginTransaction()
					if tx != nil {
						tx.Put(key, []byte("tx_stress"))
						tx.Commit()
					}
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// 验证系统稳定性
	size, err := db.walManager.GetWALSize()
	if err != nil {
		t.Errorf("获取压力审计WAL大小失败: %v", err)
	}

	t.Logf("压力审计完成: %d 操作, 耗时: %v, WAL大小: %d 字节", 
		workers*operations, duration, size)
}

// 清理审计测试
func testWALCleanupAudit(t *testing.T) {
	tempDir := setupTestDir(t)
	defer cleanupTestDir(t, tempDir)

	config := createTestConfig(tempDir)
	db, err := NewGojiDB(config)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}

	// 写入测试数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("cleanup_audit_%d", i)
		value := fmt.Sprintf("cleanup_value_%d", i)
		if err := db.Put(key, []byte(value)); err != nil {
			t.Errorf("清理审计写入失败: %v", err)
		}
	}

	// 创建检查点
	if err := db.walManager.Checkpoint(); err != nil {
		t.Errorf("清理审计检查点失败: %v", err)
	}

	// 验证文件状态
	walPath := filepath.Join(tempDir, "wal.log")
	if info, err := os.Stat(walPath); err == nil {
		t.Logf("清理审计前WAL大小: %d 字节", info.Size())
	}

	db.Close()

	// 验证文件完整性
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Error("清理审计后WAL文件丢失")
	}
}

// 辅助函数
func setupTestDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "gojidb_audit_test")
	if err != nil {
		t.Fatalf("创建审计测试临时目录失败: %v", err)
	}
	return tempDir
}

func cleanupTestDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("清理审计测试临时目录失败: %v", err)
	}
}

func createTestConfig(tempDir string) *GojiDBConfig {
	return &GojiDBConfig{
		DataPath:         tempDir,
		EnableWAL:        true,
		WALSyncWrites:    true,
		WALFlushInterval: 100 * time.Millisecond,
		WALMaxSize:       32 * 1024 * 1024, // 32MB
	}
}

// BenchmarkWALAudit 基准测试
func BenchmarkWALAudit(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "gojidb_audit_bench")
	defer os.RemoveAll(tempDir)

	config := createTestConfig(tempDir)
	db, _ := NewGojiDB(config)
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("audit_bench_%d", i)
		value := []byte("audit benchmark value")
		_ = db.Put(key, value)
	}
}

// BenchmarkWALTransactionAudit 事务基准测试
func BenchmarkWALTransactionAudit(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "gojidb_audit_tx_bench")
	defer os.RemoveAll(tempDir)

	config := createTestConfig(tempDir)
	db, _ := NewGojiDB(config)
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := db.BeginTransaction()
		key := fmt.Sprintf("audit_tx_bench_%d", i)
		value := []byte("audit transaction value")
		tx.Put(key, value)
		tx.Commit()
	}
}