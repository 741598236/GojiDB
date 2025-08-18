package gojidb

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构体
type Config struct {
	App         AppConfig         `yaml:"app"`
	Database    DatabaseConfig    `yaml:"database"`
	CLI         CLIConfig         `yaml:"cli"`
	Performance PerformanceConfig `yaml:"performance"`
	Logging     LoggingConfig     `yaml:"logging"`
	Features    FeaturesConfig    `yaml:"features"`
	Cache       CacheConfig       `yaml:"cache"`
	GojiDB      GojiDBConfig      `yaml:"gojidb"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

type DatabaseConfig struct {
	DataDir                string `yaml:"data_dir"`
	MaxMemoryMB            int    `yaml:"max_memory_mb"`
	AutoCompact            bool   `yaml:"auto_compact"`
	Compression            bool   `yaml:"compression"`
	CompressionLevel       int    `yaml:"compression_level"`
	CompressionType        string `yaml:"compression_type"`
	EnableSmartCompression bool   `yaml:"enable_smart_compression"`
}

type CLIConfig struct {
	Prompt     string `yaml:"prompt"`
	Theme      string `yaml:"theme"`
	ShowBanner bool   `yaml:"show_banner"`
	ShowStats  bool   `yaml:"show_stats"`
}

type PerformanceConfig struct {
	BatchSize     int `yaml:"batch_size"`
	FlushInterval int `yaml:"flush_interval"`
	MaxOpenFiles  int `yaml:"max_open_files"`
}

type CacheConfig struct {
	BlockCacheSize int  `yaml:"block_cache_size"`
	KeyCacheSize   int  `yaml:"key_cache_size"`
	EnableCache    bool `yaml:"enable_cache"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

type FeaturesConfig struct {
	TTLSupport         bool `yaml:"ttl_support"`
	TransactionSupport bool `yaml:"transaction_support"`
	BackupSupport      bool `yaml:"backup_support"`
	MetricsSupport     bool `yaml:"metrics_support"`
}

type GojiDBConfig struct {
	DataPath    string `yaml:"data_path"`
	MaxFileSize int64  `yaml:"max_file_size"`
	// 0: 无压缩, 1: Snappy, 2: LZ4, 3: ZSTD
	CompressionType     CompressionType         `yaml:"compression_type"`
	SyncWrites          bool                    `yaml:"sync_writes"`
	CompactionThreshold float64                 `yaml:"compaction_threshold"`
	TTLCheckInterval    time.Duration           `yaml:"ttl_check_interval"`
	EnableMetrics       bool                    `yaml:"enable_metrics"`
	EnableTTL           bool                    `yaml:"enable_ttl"`
	CacheConfig         CacheConfig             `yaml:"cache"`
	SmartCompression    *SmartCompressionConfig `yaml:"smart_compression"`
	
	// WAL配置
	EnableWAL        bool          `yaml:"enable_wal"`
	WALFlushInterval time.Duration `yaml:"wal_flush_interval"`
	WALMaxSize      int64         `yaml:"wal_max_size"`
	WALSyncWrites   bool          `yaml:"wal_sync_writes"`
}

// LoadConfig 加载配置文件
func LoadConfig(filename string) (*Config, error) {
	config := &Config{}

	// 读取配置文件
	data, err := os.ReadFile(filename)
	if err != nil {
		// 如果文件不存在，创建默认配置
		if os.IsNotExist(err) {
			config = createDefaultConfig()
			if err := SaveConfig(filename, config); err != nil {
				return nil, fmt.Errorf("创建默认配置文件失败: %v", err)
			}
			return config, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return config, nil
}

// SaveConfig 保存配置文件
func SaveConfig(filename string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// createDefaultConfig 创建默认配置
func createDefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:        "GojiDB",
			Version:     "2.1.0",
			Description: "高性能键值存储数据库",
		},
		Database: DatabaseConfig{
			DataDir:                "../data",
			MaxMemoryMB:            512,
			AutoCompact:            true,
			Compression:            true,
			EnableSmartCompression: true,
		},
		CLI: CLIConfig{
			Prompt:     "GojiDB> ",
			Theme:      "default",
			ShowBanner: true,
			ShowStats:  true,
		},
		Performance: PerformanceConfig{
			BatchSize:     1000,
			FlushInterval: 5,
			MaxOpenFiles:  1000,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "../data/gojidb.log",
			MaxSizeMB:  100,
			MaxBackups: 3,
		},
		Features: FeaturesConfig{
			TTLSupport:         true,
			TransactionSupport: true,
			BackupSupport:      true,
			MetricsSupport:     true,
		},
		Cache: CacheConfig{
			BlockCacheSize: 1024,
			KeyCacheSize:   8192,
			EnableCache:    true,
		},
		GojiDB: GojiDBConfig{
			DataPath:            "../data",
			MaxFileSize:         64 * 1024 * 1024, // 64MB
			CompressionType:     SnappyCompression,
			SyncWrites:          true,
			CompactionThreshold: 0.5,
			TTLCheckInterval:    5 * time.Minute,
			EnableMetrics:       true,
			EnableTTL:           true,
			SmartCompression:    DefaultSmartCompressionConfig(),
			EnableWAL:           true,
			WALFlushInterval:    1 * time.Second,
			WALMaxSize:          32 * 1024 * 1024, // 32MB
			WALSyncWrites:       true,
		},
	}
}

// GetPrompt 获取CLI提示符
func (c *Config) GetPrompt() string {
	return ColorBold + ColorGreen + c.CLI.Prompt + ColorReset
}

// IsFeatureEnabled 检查特性是否启用
func (c *Config) IsFeatureEnabled(feature string) bool {
	switch feature {
	case "ttl":
		return c.Features.TTLSupport
	case "transaction":
		return c.Features.TransactionSupport
	case "backup":
		return c.Features.BackupSupport
	case "metrics":
		return c.Features.MetricsSupport
	default:
		return false
	}
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() map[string]string {
	hostname, _ := os.Hostname()
	user, _ := user.Current()
	return map[string]string{
		"hostname":   hostname,
		"username":   user.Username,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
		"cpu_count":  fmt.Sprintf("%d", runtime.NumCPU()),
	}
}
