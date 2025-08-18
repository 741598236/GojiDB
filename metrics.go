package GojiDB

import (
	"time"
)

// Metrics 数据库性能指标
type Metrics struct {
	TotalWrites      int64     // 总写入次数
	TotalReads       int64     // 总读取次数
	TotalDeletes     int64     // 总删除次数
	KeyCount         int64     // 当前键数量
	DataSize         int64     // 数据总大小
	CompressionRatio int64     // 压缩率
	CacheHits        int64     // 缓存命中次数
	CacheMisses      int64     // 缓存未命中次数
	ExpiredKeyCount  int64     // 过期键数量
	CompactionCount  int64     // 合并次数
	StartTime        time.Time // 启动时间
	SystemInfo       map[string]string
}
