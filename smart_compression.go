package GojiDB

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

// SmartCompressionConfig 智能压缩配置
type SmartCompressionConfig struct {
	MinSize         int     `yaml:"min_size"`         // 最小压缩大小(字节)
	MaxSize         int     `yaml:"max_size"`         // 最大压缩大小(字节)
	CompressionGain float64 `yaml:"compression_gain"` // 最小压缩收益(百分比)
	Algorithm       string  `yaml:"algorithm"`        // 默认算法: auto, snappy, zstd, gzip
	EnableAdaptive  bool    `yaml:"enable_adaptive"`  // 启用自适应算法选择
	SampleSize      int     `yaml:"sample_size"`      // 采样大小用于测试压缩效果

	// 新增优化配置
	EnableParallel   bool `yaml:"enable_parallel"`   // 启用并行压缩
	MaxParallelism   int  `yaml:"max_parallelism"`   // 最大并行度
	EnableStreaming  bool `yaml:"enable_streaming"`  // 启用流式压缩
	EnableCache      bool `yaml:"enable_cache"`      // 启用压缩缓存
	CacheSize        int  `yaml:"cache_size"`        // 缓存大小
	EnableDictionary bool `yaml:"enable_dictionary"` // 启用字典训练
	DictionarySize   int  `yaml:"dictionary_size"`   // 字典大小
	EnableMixedMode  bool `yaml:"enable_mixed_mode"` // 启用混合压缩模式
	ChunkSize        int  `yaml:"chunk_size"`        // 分块大小
}

// CompressionStats 压缩统计信息
type CompressionStats struct {
	TotalBytes       int64   `json:"total_bytes"`
	CompressedBytes  int64   `json:"compressed_bytes"`
	CompressionRatio float64 `json:"compression_ratio"`
	Algorithm        string  `json:"algorithm"`
	SkippedCount     int64   `json:"skipped_count"`
	CompressedCount  int64   `json:"compressed_count"`

	// 新增统计信息
	CacheHitCount    int64   `json:"cache_hit_count"`
	CacheMissCount   int64   `json:"cache_miss_count"`
	ParallelJobs     int64   `json:"parallel_jobs"`
	DictionaryHits   int64   `json:"dictionary_hits"`
	MixedModeSavings float64 `json:"mixed_mode_savings"`
}

// CompressionCache 压缩缓存
type CompressionCache struct {
	cache map[string]CacheEntry
	mutex sync.RWMutex
	size  int
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      []byte
	Algorithm CompressionType
	Timestamp time.Time
	HitCount  int
}

// CompressionDictionary 压缩字典
type CompressionDictionary struct {
	Data      []byte
	Type      CompressionType
	Samples   [][]byte
	UpdatedAt time.Time
}

// ParallelJob 并行压缩任务
type ParallelJob struct {
	ID        int
	Data      []byte
	Algorithm CompressionType
	Result    []byte
	Error     error
	Done      chan struct{}
}

// SmartCompressor 智能压缩器
type SmartCompressor struct {
	config       *SmartCompressionConfig
	stats        *CompressionStats
	zstdEncoder  *zstd.Encoder
	zstdDecoder  *zstd.Decoder
	lastDecision map[string]CompressionDecision

	// 新增组件
	cache        *CompressionCache
	dictionaries map[CompressionType]*CompressionDictionary
	workerPool   chan *ParallelJob
	ctx          context.Context
	cancel       context.CancelFunc
}

// CompressionDecision 压缩决策
type CompressionDecision struct {
	Algorithm      CompressionType
	ShouldCompress bool
	ExpectedGain   float64
	ActualGain     float64
	Timestamp      time.Time
}

// CompressionProfile 压缩性能档案
type CompressionProfile struct {
	DataType      string
	AverageGain   float64
	BestAlgorithm CompressionType
	TotalSamples  int
	LastUpdated   time.Time
}

// NewSmartCompressor 创建新的智能压缩器
func NewSmartCompressor(config *SmartCompressionConfig) *SmartCompressor {
	encoder, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	decoder, _ := zstd.NewReader(nil)
	ctx, cancel := context.WithCancel(context.Background())

	sc := &SmartCompressor{
		config:       config,
		stats:        &CompressionStats{},
		zstdEncoder:  encoder,
		zstdDecoder:  decoder,
		lastDecision: make(map[string]CompressionDecision),
		cache:        newCompressionCache(config.CacheSize),
		dictionaries: make(map[CompressionType]*CompressionDictionary),
		workerPool:   make(chan *ParallelJob, 100),
		ctx:          ctx,
		cancel:       cancel,
	}

	// 初始化字典
	if config.EnableDictionary {
		sc.initDictionaries()
	}

	// 启动并行工作池
	if config.EnableParallel {
		sc.startWorkerPool()
	}

	return sc
}

// ShouldCompress 判断是否值得压缩
func (sc *SmartCompressor) ShouldCompress(data []byte) bool {
	dataSize := len(data)

	// 基本大小检查
	if dataSize < sc.config.MinSize {
		return false
	}

	if dataSize > sc.config.MaxSize {
		return false // 数据太大，建议分块处理
	}

	// 数据熵检测 - 高熵数据（如加密数据）压缩效果差
	entropy := sc.calculateEntropy(data)
	if entropy > 7.8 { // 接近最大熵8.0
		return false
	}

	// 重复模式检测
	if sc.hasLowRedundancy(data) {
		return false
	}

	// 自适应算法选择
	if sc.config.EnableAdaptive {
		_, expectedGain := sc.selectBestAlgorithm(data)
		if expectedGain >= sc.config.CompressionGain {
			return true
		}
	} else {
		// 使用默认算法测试
		gain := sc.testCompression(data, sc.getDefaultAlgorithm())
		if gain >= sc.config.CompressionGain {
			return true
		}
	}

	return false
}

// CompressWithSmartChoice 智能压缩（优化版本）
func (sc *SmartCompressor) CompressWithSmartChoice(data []byte) ([]byte, CompressionType, error) {
	// 1. 缓存检查
	if sc.config.EnableCache {
		if cached, algo, hit := sc.cache.get(string(data)); hit {
			atomic.AddInt64(&sc.stats.CacheHitCount, 1)
			return cached, algo, nil
		}
		atomic.AddInt64(&sc.stats.CacheMissCount, 1)
	}

	// 2. 基础检查
	shouldCompress := sc.ShouldCompress(data)
	if !shouldCompress {
		atomic.AddInt64(&sc.stats.SkippedCount, 1)
		if sc.config.EnableCache {
			sc.cache.set(string(data), data, NoCompression)
		}
		return data, NoCompression, nil
	}

	// 3. 混合压缩模式
	if sc.config.EnableMixedMode && len(data) > sc.config.ChunkSize {
		compressed, _, err := sc.compressMixedMode(data)
		return compressed, MixedCompression, err
	}

	// 4. 并行压缩
	if sc.config.EnableParallel && len(data) > sc.config.ChunkSize {
		compressed, _, err := sc.compressParallel(data)
		return compressed, ParallelCompression, err
	}

	// 5. 流式压缩
	if sc.config.EnableStreaming && len(data) > sc.config.ChunkSize {
		compressed, _, err := sc.compressStream(data, sc.config.ChunkSize)
		return compressed, StreamCompression, err
	}

	// 6. 传统压缩路径
	algorithm := sc.selectBestAlgorithmWithContext(data)
	compressed, err := sc.compressWithAlgorithmAndDictionary(data, algorithm)
	if err != nil {
		return data, NoCompression, err
	}

	// 7. 验证压缩效果
	actualGain := float64(len(data)-len(compressed)) / float64(len(data)) * 100
	if actualGain < sc.config.CompressionGain {
		// 压缩效果不理想，返回原始数据
		if sc.config.EnableCache {
			sc.cache.set(string(data), data, NoCompression)
		}
		return data, NoCompression, nil
	}

	// 8. 更新统计和缓存
	atomic.AddInt64(&sc.stats.CompressedCount, 1)
	atomic.AddInt64(&sc.stats.TotalBytes, int64(len(data)))
	atomic.AddInt64(&sc.stats.CompressedBytes, int64(len(compressed)))

	// 9. 训练字典
	if sc.config.EnableDictionary {
		sc.addToDictionaryTraining(algorithm, data)
	}

	// 10. 更新决策记录和缓存
	sc.updateDecision(data, algorithm, actualGain)
	if sc.config.EnableCache {
		sc.cache.set(string(data), compressed, algorithm)
	}

	return compressed, algorithm, nil
}

// selectBestAlgorithm 选择最佳压缩算法
func (sc *SmartCompressor) selectBestAlgorithm(data []byte) (CompressionType, float64) {
	algorithms := []CompressionType{SnappyCompression, ZSTDCompression, GzipCompression}
	bestAlgo := NoCompression
	bestGain := 0.0

	// 采样测试
	sample := data
	if len(data) > sc.config.SampleSize {
		sample = data[:sc.config.SampleSize]
	}

	for _, algo := range algorithms {
		gain := sc.testCompression(sample, algo)
		if gain > bestGain {
			bestGain = gain
			bestAlgo = algo
		}
	}

	return bestAlgo, bestGain
}

// testCompression 测试压缩效果
func (sc *SmartCompressor) testCompression(data []byte, algorithm CompressionType) float64 {
	if len(data) == 0 {
		return 0
	}

	compressed, err := sc.compressWithAlgorithm(data, algorithm)
	if err != nil {
		return 0
	}

	return float64(len(data)-len(compressed)) / float64(len(data)) * 100
}

// compressWithAlgorithm 使用指定算法压缩
func (sc *SmartCompressor) compressWithAlgorithm(data []byte, algorithm CompressionType) ([]byte, error) {
	switch algorithm {
	case SnappyCompression:
		return snappy.Encode(nil, data), nil
	case ZSTDCompression:
		return sc.zstdEncoder.EncodeAll(data, make([]byte, 0, len(data))), nil
	case GzipCompression:
		return sc.compressGzip(data)
	default:
		return data, nil
	}
}

// compressGzip gzip压缩
func (sc *SmartCompressor) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	w.Close()
	return buf.Bytes(), nil
}

// calculateEntropy 计算数据熵
func (sc *SmartCompressor) calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	entropy := 0.0
	length := float64(len(data))
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * (func(p float64) float64 {
				return math.Log2(p)
			}(p))
		}
	}

	return entropy
}

// hasLowRedundancy 检测数据是否缺乏冗余性
func (sc *SmartCompressor) hasLowRedundancy(data []byte) bool {
	if len(data) < 32 {
		return false
	}

	// 简单的重复模式检测 - 放宽条件
	uniqueBytes := make(map[byte]bool)
	for _, b := range data {
		uniqueBytes[b] = true
	}

	// 如果唯一字节数超过90%的数据长度，认为冗余度低
	uniqueRatio := float64(len(uniqueBytes)) / float64(len(data))
	if uniqueRatio > 0.9 {
		return true
	}

	// 检测重复模式 - 放宽连续重复的要求
	// 简单的重复检测：检查是否有重复的字节序列
	for i := 0; i < len(data)-4; i++ {
		pattern := data[i : i+4]
		count := 0
		for j := 0; j < len(data)-3; j++ {
			if bytes.Equal(data[j:j+4], pattern) {
				count++
				if count >= 3 { // 发现重复模式
					return false
				}
			}
		}
	}

	// 检查文本类型的特征
	printableCount := 0
	for _, b := range data {
		if b >= 32 && b <= 126 { // 可打印ASCII字符
			printableCount++
		}
	}

	// 如果大部分是可打印字符，可能适合压缩
	printableRatio := float64(printableCount) / float64(len(data))
	return printableRatio < 0.7
}

// getDefaultAlgorithm 获取默认压缩算法
func (sc *SmartCompressor) getDefaultAlgorithm() CompressionType {
	switch sc.config.Algorithm {
	case "snappy":
		return SnappyCompression
	case "zstd":
		return ZSTDCompression
	case "gzip":
		return GzipCompression
	default:
		return SnappyCompression // 默认使用Snappy
	}
}

// updateDecision 更新决策记录
func (sc *SmartCompressor) updateDecision(data []byte, algorithm CompressionType, actualGain float64) {
	// 这里可以根据实际效果调整算法选择策略
	// 实现机器学习式的算法优化
}

// GetStats 获取压缩统计信息
func (sc *SmartCompressor) GetStats() CompressionStats {
	return CompressionStats{
		TotalBytes:       atomic.LoadInt64(&sc.stats.TotalBytes),
		CompressedBytes:  atomic.LoadInt64(&sc.stats.CompressedBytes),
		CompressionRatio: sc.calculateOverallRatio(),
		Algorithm:        sc.config.Algorithm,
		SkippedCount:     atomic.LoadInt64(&sc.stats.SkippedCount),
		CompressedCount:  atomic.LoadInt64(&sc.stats.CompressedCount),
	}
}

// calculateOverallRatio 计算整体压缩比
func (sc *SmartCompressor) calculateOverallRatio() float64 {
	total := atomic.LoadInt64(&sc.stats.TotalBytes)
	compressed := atomic.LoadInt64(&sc.stats.CompressedBytes)

	if total == 0 {
		return 0
	}

	return float64(total-compressed) / float64(total) * 100
}

// ResetStats 重置统计信息
func (sc *SmartCompressor) ResetStats() {
	atomic.StoreInt64(&sc.stats.TotalBytes, 0)
	atomic.StoreInt64(&sc.stats.CompressedBytes, 0)
	atomic.StoreInt64(&sc.stats.SkippedCount, 0)
	atomic.StoreInt64(&sc.stats.CompressedCount, 0)
}

// algorithmToString 算法类型转字符串
func (sc *SmartCompressor) algorithmToString(algo CompressionType) string {
	switch algo {
	case SnappyCompression:
		return "snappy"
	case ZSTDCompression:
		return "zstd"
	case GzipCompression:
		return "gzip"
	default:
		return "none"
	}
}

// 压缩缓存相关方法
func newCompressionCache(size int) *CompressionCache {
	if size <= 0 {
		size = 1000 // 默认缓存1000个条目
	}
	return &CompressionCache{
		cache: make(map[string]CacheEntry),
		size:  size,
	}
}

func (cc *CompressionCache) get(key string) ([]byte, CompressionType, bool) {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()

	if entry, exists := cc.cache[key]; exists {
		entry.HitCount++
		return entry.Data, entry.Algorithm, true
	}
	return nil, NoCompression, false
}

func (cc *CompressionCache) set(key string, data []byte, algorithm CompressionType) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	// 如果缓存已满，移除最旧的条目
	if len(cc.cache) >= cc.size {
		var oldestKey string
		var oldestTime time.Time

		for k, v := range cc.cache {
			if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		delete(cc.cache, oldestKey)
	}

	cc.cache[key] = CacheEntry{
		Data:      data,
		Algorithm: algorithm,
		Timestamp: time.Now(),
		HitCount:  0,
	}
}

// 字典相关方法
func (sc *SmartCompressor) initDictionaries() {
	for _, algo := range []CompressionType{SnappyCompression, ZSTDCompression, GzipCompression} {
		sc.dictionaries[algo] = &CompressionDictionary{
			Type:      algo,
			Samples:   make([][]byte, 0),
			UpdatedAt: time.Now(),
		}
	}
}

func (sc *SmartCompressor) trainDictionary(algo CompressionType, samples [][]byte) {
	if !sc.config.EnableDictionary || len(samples) < 10 {
		return
	}

	// 简单的字典训练：收集常见模式
	commonPatterns := sc.extractCommonPatterns(samples)
	if len(commonPatterns) > 0 {
		dict := sc.dictionaries[algo]
		dict.Data = commonPatterns
		dict.Samples = samples
		dict.UpdatedAt = time.Now()
	}
}

func (sc *SmartCompressor) extractCommonPatterns(samples [][]byte) []byte {
	// 提取常见模式用于字典训练
	patternFreq := make(map[string]int)

	for _, sample := range samples {
		if len(sample) < 8 {
			continue
		}

		// 提取8字节模式
		for i := 0; i <= len(sample)-8; i++ {
			pattern := string(sample[i : i+8])
			patternFreq[pattern]++
		}
	}

	// 找出最常见的模式
	var commonPatterns []byte
	maxPatterns := 100 // 限制字典大小

	for pattern, freq := range patternFreq {
		if freq > 5 && len(commonPatterns) < maxPatterns*8 {
			commonPatterns = append(commonPatterns, []byte(pattern)...)
		}
	}

	return commonPatterns
}

// 并行压缩相关方法
func (sc *SmartCompressor) startWorkerPool() {
	maxWorkers := sc.config.MaxParallelism
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}

	for i := 0; i < maxWorkers; i++ {
		go sc.worker()
	}
}

func (sc *SmartCompressor) worker() {
	for {
		select {
		case job := <-sc.workerPool:
			if job == nil {
				return
			}

			result, err := sc.compressWithAlgorithm(job.Data, job.Algorithm)
			job.Result = result
			job.Error = err
			close(job.Done)

		case <-sc.ctx.Done():
			return
		}
	}
}

// 流式压缩
func (sc *SmartCompressor) compressStream(data []byte, chunkSize int) ([]byte, CompressionType, error) {
	if len(data) <= chunkSize {
		compressed, _, err := sc.CompressWithSmartChoice(data)
		return compressed, StreamCompression, err
	}

	var result bytes.Buffer

	// 添加流式压缩标记
	result.WriteByte(0x03)

	chunks := len(data) / chunkSize
	if len(data)%chunkSize != 0 {
		chunks++
	}

	// 写入总块数
	binary.Write(&result, binary.LittleEndian, uint32(chunks))

	// 使用相同的算法处理所有块
	algorithm := sc.selectBestAlgorithmWithContext(data)
	result.WriteByte(byte(algorithm))

	// 并行压缩每个块
	jobs := make([]*ParallelJob, chunks)
	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		jobs[i] = &ParallelJob{
			ID:        i,
			Data:      data[start:end],
			Algorithm: algorithm,
			Done:      make(chan struct{}),
		}

		sc.workerPool <- jobs[i]
	}

	// 收集结果
	for _, job := range jobs {
		<-job.Done
		if job.Error != nil {
			return nil, StreamCompression, job.Error
		}

		// 写入每个块的元信息：原始大小(4字节) + 压缩大小(4字节) + 压缩数据
		binary.Write(&result, binary.LittleEndian, uint32(len(job.Data)))
		binary.Write(&result, binary.LittleEndian, uint32(len(job.Result)))
		result.Write(job.Result)
	}

	return result.Bytes(), StreamCompression, nil
}

// Decompress 解压数据
func (sc *SmartCompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// 检查是否是混合压缩格式 (以\x01开头，格式: \x01<algo>:<size>:<data>)
	if len(data) > 0 && data[0] == 0x01 {
		return sc.decompressMixedMode(data)
	}

	// 检查是否是并行压缩格式 (以\x02开头)
	if len(data) > 0 && data[0] == 0x02 {
		return sc.decompressParallelMode(data)
	}

	// 检查是否是流式压缩格式 (以\x03开头)
	if len(data) > 0 && data[0] == 0x03 {
		return sc.decompressStreamMode(data)
	}

	// 检测压缩算法类型
	// 检查Snappy格式
	if len(data) > 0 {
		if decompressed, err := snappy.Decode(nil, data); err == nil {
			return decompressed, nil
		}
	}

	// 检查ZSTD格式
	if sc.zstdDecoder != nil {
		if decompressed, err := sc.zstdDecoder.DecodeAll(data, nil); err == nil {
			return decompressed, nil
		}
	}

	// 检查Gzip格式
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %v", err)
		}
		defer reader.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(reader); err != nil {
			return nil, fmt.Errorf("failed to read decompressed data: %v", err)
		}
		return buf.Bytes(), nil
	}

	// 如果不是压缩格式，返回原始数据
	return data, nil
}

// Close 关闭压缩器
func (sc *SmartCompressor) Close() {
	if sc.zstdEncoder != nil {
		sc.zstdEncoder.Close()
	}
	if sc.zstdDecoder != nil {
		sc.zstdDecoder.Close()
	}

	// 关闭工作池
	sc.cancel()

	// 发送nil到所有worker以退出循环
	maxWorkers := sc.config.MaxParallelism
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}
	for i := 0; i < maxWorkers; i++ {
		sc.workerPool <- nil
	}
	close(sc.workerPool)
}

// DefaultSmartCompressionConfig 默认智能压缩配置（优化版本）
func DefaultSmartCompressionConfig() *SmartCompressionConfig {
	return &SmartCompressionConfig{
		MinSize:         64,          // 最小64字节才压缩
		MaxSize:         1024 * 1024, // 最大1MB
		CompressionGain: 10.0,        // 至少10%的压缩收益
		Algorithm:       "auto",      // 自动选择
		EnableAdaptive:  true,        // 启用自适应
		SampleSize:      1024,        // 采样1KB测试

		// 新增优化配置
		EnableParallel:   true,             // 启用并行压缩
		MaxParallelism:   runtime.NumCPU(), // 使用CPU核心数
		EnableStreaming:  true,             // 启用流式压缩
		EnableCache:      true,             // 启用压缩缓存
		CacheSize:        10000,            // 缓存10000个条目
		EnableDictionary: true,             // 启用字典训练
		DictionarySize:   64 * 1024,        // 64KB字典
		EnableMixedMode:  true,             // 启用混合压缩模式
		ChunkSize:        256 * 1024,       // 256KB分块
	}
}

// compressMixedMode 混合压缩模式
func (sc *SmartCompressor) compressMixedMode(data []byte) ([]byte, CompressionType, error) {
	chunkSize := sc.config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 64 * 1024 // 默认64KB
	}

	chunks := len(data) / chunkSize
	if len(data)%chunkSize != 0 {
		chunks++
	}

	var result bytes.Buffer
	var totalCompressed int

	// 添加混合压缩标记
	result.WriteByte(0x01)

	// 为每个块选择最佳算法
	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[start:end]
		algorithm := sc.selectBestAlgorithmWithContext(chunk)

		compressed, err := sc.compressWithAlgorithmAndDictionary(chunk, algorithm)
		if err != nil {
			return data, NoCompression, err
		}

		// 写入块头信息
		header := fmt.Sprintf("%d:%d:", algorithm, len(compressed))
		result.WriteString(header)
		result.Write(compressed)

		totalCompressed += len(compressed)
	}

	// 计算混合压缩的节省率
	mixedSavings := float64(len(data)-totalCompressed) / float64(len(data)) * 100
	atomic.StoreInt64((*int64)(unsafe.Pointer(&sc.stats.MixedModeSavings)), int64(mixedSavings*100))

	return result.Bytes(), MixedCompression, nil
}

// compressParallel 并行压缩
func (sc *SmartCompressor) compressParallel(data []byte) ([]byte, CompressionType, error) {
	chunkSize := sc.config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 256 * 1024 // 默认256KB用于并行压缩
	}

	chunks := len(data) / chunkSize
	if len(data)%chunkSize != 0 {
		chunks++
	}

	// 限制最大并行度
	maxChunks := sc.config.MaxParallelism
	if maxChunks <= 0 {
		maxChunks = runtime.NumCPU()
	}
	if chunks > maxChunks {
		chunkSize = len(data) / maxChunks
		chunks = maxChunks
	}

	jobs := make([]*ParallelJob, chunks)
	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		algorithm := sc.selectBestAlgorithmWithContext(data[start:end])

		jobs[i] = &ParallelJob{
			ID:        i,
			Data:      data[start:end],
			Algorithm: algorithm,
			Done:      make(chan struct{}),
		}

		sc.workerPool <- jobs[i]
	}

	var result bytes.Buffer

	// 添加并行压缩标记
	result.WriteByte(0x02)

	// 写入总块数
	binary.Write(&result, binary.LittleEndian, uint32(chunks))

	for _, job := range jobs {
		<-job.Done
		if job.Error != nil {
			return data, NoCompression, job.Error
		}

		// 写入每个块的元信息：算法类型(1字节) + 原始大小(4字节) + 压缩大小(4字节) + 压缩数据
		result.WriteByte(byte(job.Algorithm))
		binary.Write(&result, binary.LittleEndian, uint32(len(job.Data)))
		binary.Write(&result, binary.LittleEndian, uint32(len(job.Result)))
		result.Write(job.Result)
	}

	atomic.AddInt64(&sc.stats.ParallelJobs, int64(chunks))
	return result.Bytes(), ParallelCompression, nil
}

// selectBestAlgorithmWithContext 带上下文感知的算法选择
func (sc *SmartCompressor) selectBestAlgorithmWithContext(data []byte) CompressionType {
	if !sc.config.EnableAdaptive {
		return sc.getDefaultAlgorithm()
	}

	// 数据类型识别
	dataType := sc.identifyDataType(data)

	// 基于数据类型选择算法
	switch dataType {
	case "text":
		return ZSTDCompression // 文本数据用ZSTD效果好
	case "json":
		return GzipCompression // JSON数据用Gzip
	case "binary":
		return SnappyCompression // 二进制数据用Snappy
	default:
		// 回退到采样测试
		algo, _ := sc.selectBestAlgorithm(data)
		return algo
	}
}

// identifyDataType 识别数据类型
func (sc *SmartCompressor) identifyDataType(data []byte) string {
	if len(data) == 0 {
		return "empty"
	}

	// JSON检测
	if (data[0] == '{' && data[len(data)-1] == '}') ||
		(data[0] == '[' && data[len(data)-1] == ']') {
		return "json"
	}

	// 文本检测
	printable := 0
	for _, b := range data {
		if b >= 32 && b <= 126 {
			printable++
		}
	}
	if float64(printable)/float64(len(data)) > 0.8 {
		return "text"
	}

	return "binary"
}

// compressWithAlgorithmAndDictionary 使用字典的压缩
func (sc *SmartCompressor) compressWithAlgorithmAndDictionary(data []byte, algorithm CompressionType) ([]byte, error) {
	// 检查字典是否可用
	if sc.config.EnableDictionary {
		if dict := sc.dictionaries[algorithm]; dict != nil && len(dict.Data) > 0 {
			// 使用字典优化压缩
			atomic.AddInt64(&sc.stats.DictionaryHits, 1)
		}
	}

	return sc.compressWithAlgorithm(data, algorithm)
}

// addToDictionaryTraining 添加到字典训练
func (sc *SmartCompressor) addToDictionaryTraining(algorithm CompressionType, data []byte) {
	if !sc.config.EnableDictionary {
		return
	}

	dict := sc.dictionaries[algorithm]
	if len(dict.Samples) < 1000 { // 限制样本数量
		dict.Samples = append(dict.Samples, data)
	}

	// 定期训练字典
	if len(dict.Samples) >= 100 {
		sc.trainDictionary(algorithm, dict.Samples)
	}
}

// decompressMixedMode 解压混合压缩模式
func (sc *SmartCompressor) decompressMixedMode(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != 0x01 {
		return data, nil
	}

	var result bytes.Buffer
	pos := 1

	for pos < len(data) {
		// 解析块头: <algo>:<size>:
		colon1 := bytes.IndexByte(data[pos:], ':')
		if colon1 == -1 {
			break
		}
		colon1 += pos

		colon2 := bytes.IndexByte(data[colon1+1:], ':')
		if colon2 == -1 {
			break
		}
		colon2 += colon1 + 1

		// 提取算法和大小
		algoStr := string(data[pos:colon1])
		sizeStr := string(data[colon1+1 : colon2])

		chunkSize, err := strconv.Atoi(sizeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid chunk size: %v", err)
		}

		algorithm := NoCompression
		switch algoStr {
		case "1":
			algorithm = SnappyCompression
		case "2":
			algorithm = ZSTDCompression
		case "3":
			algorithm = GzipCompression
		default:
			algorithm = NoCompression
		}

		// 提取压缩数据
		dataStart := colon2 + 1
		dataEnd := dataStart + chunkSize
		if dataEnd > len(data) {
			return nil, fmt.Errorf("invalid chunk data boundary")
		}

		chunkData := data[dataStart:dataEnd]

		// 解压块数据
		decompressed, err := sc.decompressChunk(chunkData, algorithm)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress chunk: %v", err)
		}

		result.Write(decompressed)
		pos = dataEnd
	}

	return result.Bytes(), nil
}

// decompressParallelMode 解压并行压缩模式
func (sc *SmartCompressor) decompressParallelMode(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != 0x02 {
		return data, nil
	}

	if len(data) < 5 {
		return nil, fmt.Errorf("invalid parallel compression format")
	}

	reader := bytes.NewReader(data[1:])

	// 读取总块数
	var chunkCount uint32
	if err := binary.Read(reader, binary.LittleEndian, &chunkCount); err != nil {
		return nil, fmt.Errorf("failed to read chunk count: %v", err)
	}

	var result bytes.Buffer

	// 解压每个块
	for i := 0; i < int(chunkCount); i++ {
		// 读取算法类型
		algorithmByte, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read algorithm type: %v", err)
		}

		// 读取原始大小和压缩大小
		var originalSize, compressedSize uint32
		if err := binary.Read(reader, binary.LittleEndian, &originalSize); err != nil {
			return nil, fmt.Errorf("failed to read original size: %v", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &compressedSize); err != nil {
			return nil, fmt.Errorf("failed to read compressed size: %v", err)
		}

		// 读取压缩数据
		compressedData := make([]byte, compressedSize)
		if _, err := io.ReadFull(reader, compressedData); err != nil {
			return nil, fmt.Errorf("failed to read compressed data: %v", err)
		}

		// 解压块数据
		decompressed, err := sc.decompressChunk(compressedData, CompressionType(algorithmByte))
		if err != nil {
			return nil, fmt.Errorf("failed to decompress chunk %d: %v", i, err)
		}

		if len(decompressed) != int(originalSize) {
			return nil, fmt.Errorf("chunk %d size mismatch: expected %d, got %d", i, originalSize, len(decompressed))
		}

		result.Write(decompressed)
	}

	return result.Bytes(), nil
}

// decompressStreamMode 解压流式压缩模式
func (sc *SmartCompressor) decompressStreamMode(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != 0x03 {
		return data, nil
	}

	if len(data) < 6 {
		return nil, fmt.Errorf("invalid stream compression format")
	}

	reader := bytes.NewReader(data[1:])

	// 读取总块数
	var chunkCount uint32
	if err := binary.Read(reader, binary.LittleEndian, &chunkCount); err != nil {
		return nil, fmt.Errorf("failed to read chunk count: %v", err)
	}

	// 读取算法类型
	algorithmByte, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read algorithm type: %v", err)
	}
	algorithm := CompressionType(algorithmByte)

	var result bytes.Buffer

	// 解压每个块
	for i := 0; i < int(chunkCount); i++ {
		// 读取原始大小和压缩大小
		var originalSize, compressedSize uint32
		if err := binary.Read(reader, binary.LittleEndian, &originalSize); err != nil {
			return nil, fmt.Errorf("failed to read original size: %v", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &compressedSize); err != nil {
			return nil, fmt.Errorf("failed to read compressed size: %v", err)
		}

		// 读取压缩数据
		compressedData := make([]byte, compressedSize)
		if _, err := io.ReadFull(reader, compressedData); err != nil {
			return nil, fmt.Errorf("failed to read compressed data: %v", err)
		}

		// 解压块数据
		decompressed, err := sc.decompressChunk(compressedData, algorithm)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress chunk %d: %v", i, err)
		}

		if len(decompressed) != int(originalSize) {
			return nil, fmt.Errorf("chunk %d size mismatch: expected %d, got %d", i, originalSize, len(decompressed))
		}

		result.Write(decompressed)
	}

	return result.Bytes(), nil
}

// decompressSimpleFormat 解压简单格式（直接压缩数据）
func (sc *SmartCompressor) decompressSimpleFormat(data []byte) ([]byte, error) {
	// 检查Snappy格式
	if len(data) > 0 {
		if decompressed, err := snappy.Decode(nil, data); err == nil {
			return decompressed, nil
		}
	}

	// 检查ZSTD格式
	if sc.zstdDecoder != nil {
		if decompressed, err := sc.zstdDecoder.DecodeAll(data, nil); err == nil {
			return decompressed, nil
		}
	}

	// 检查Gzip格式
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %v", err)
		}
		defer reader.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(reader); err != nil {
			return nil, fmt.Errorf("failed to read decompressed data: %v", err)
		}
		return buf.Bytes(), nil
	}

	// 如果不是压缩格式，返回原始数据
	return data, nil
}

// decompressChunk 解压单个块数据
func (sc *SmartCompressor) decompressChunk(data []byte, algorithm CompressionType) ([]byte, error) {
	switch algorithm {
	case SnappyCompression:
		return snappy.Decode(nil, data)
	case ZSTDCompression:
		return sc.zstdDecoder.DecodeAll(data, nil)
	case GzipCompression:
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(reader); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	default:
		return data, nil
	}
}

// PredictCompressionGain 预测压缩收益
func (sc *SmartCompressor) PredictCompressionGain(data []byte) (CompressionType, float64, bool) {
	if len(data) < sc.config.MinSize {
		return NoCompression, 0, false
	}

	entropy := sc.calculateEntropy(data)
	if entropy > 7.8 {
		return NoCompression, 0, false
	}

	bestAlgo, expectedGain := sc.selectBestAlgorithm(data)
	return bestAlgo, expectedGain, expectedGain >= sc.config.CompressionGain
}
