package gojidb

import (
	"time"
)

type Metrics struct {
	TotalWrites      int64
	TotalReads       int64
	TotalDeletes     int64
	KeyCount         int64
	DataSize         int64
	CompressionRatio int64
	CacheHits        int64
	CacheMisses      int64
	ExpiredKeyCount  int64
	CompactionCount  int64
	StartTime        time.Time
	SystemInfo       map[string]string
}
