package GojiDB

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

// TTLManager TTL管理器
type TTLManager struct {
	expiryHeap    *ExpiryHeap
	mu            sync.RWMutex
	stopChan      chan struct{}
	checkInterval time.Duration
	cleanupFunc   func(key string)
}

// ExpiryItem 过期项
type ExpiryItem struct {
	key       string
	expiresAt time.Time
	index     int
}

// ExpiryHeap 过期时间堆
type ExpiryHeap []*ExpiryItem

func (h ExpiryHeap) Len() int           { return len(h) }
func (h ExpiryHeap) Less(i, j int) bool { return h[i].expiresAt.Before(h[j].expiresAt) }
func (h ExpiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *ExpiryHeap) Push(x interface{}) {
	item := x.(*ExpiryItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *ExpiryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// NewTTLManager 创建新的TTL管理器
func NewTTLManager(cleanupFunc func(key string)) *TTLManager {
	manager := &TTLManager{
		expiryHeap:    &ExpiryHeap{},
		stopChan:      make(chan struct{}),
		checkInterval: 100 * time.Millisecond,
		cleanupFunc:   cleanupFunc,
	}

	heap.Init(manager.expiryHeap)
	go manager.cleanupLoop()

	return manager
}

// Add 添加或更新TTL项
func (tm *TTLManager) Add(key string, ttl time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	expiresAt := time.Now().Add(ttl)

	// 查找是否已存在
	for _, item := range *tm.expiryHeap {
		if item.key == key {
			item.expiresAt = expiresAt
			heap.Fix(tm.expiryHeap, item.index)
			return
		}
	}

	// 添加新项
	item := &ExpiryItem{
		key:       key,
		expiresAt: expiresAt,
	}
	heap.Push(tm.expiryHeap, item)
}

// Remove 移除TTL项
func (tm *TTLManager) Remove(key string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, item := range *tm.expiryHeap {
		if item.key == key {
			heap.Remove(tm.expiryHeap, item.index)
			return
		}
	}
}

// cleanupLoop 清理循环
func (tm *TTLManager) cleanupLoop() {
	ticker := time.NewTicker(tm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.processExpiredKeys()
		case <-tm.stopChan:
			return
		}
	}
}

// processExpiredKeys 处理过期键
func (tm *TTLManager) processExpiredKeys() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()

	for tm.expiryHeap.Len() > 0 {
		item := (*tm.expiryHeap)[0]
		if item.expiresAt.After(now) {
			break // 没有更多过期的
		}

		// 移除并处理过期键
		heap.Pop(tm.expiryHeap)
		if tm.cleanupFunc != nil {
			go tm.cleanupFunc(item.key) // 异步清理
		}
	}
}

// Stop 停止TTL管理器
func (tm *TTLManager) Stop() {
	close(tm.stopChan)
}

// GetStats 获取统计信息
func (tm *TTLManager) GetStats() (int, time.Time) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	size := tm.expiryHeap.Len()
	var nextExpiry time.Time

	if size > 0 {
		nextExpiry = (*tm.expiryHeap)[0].expiresAt
	}

	return size, nextExpiry
}

// TimeWheel 时间轮实现
type TimeWheel struct {
	interval    time.Duration
	ticker      *time.Ticker
	ticks       int64
	buckets     []*bucket
	currentPos  int
	bucketCount int
	mu          sync.RWMutex
	stopChan    chan struct{}
	cleanupFunc func(key string)
}

type bucket struct {
	items map[string]*time.Timer
	mu    sync.RWMutex
}

// NewTimeWheel 创建新的时间轮
func NewTimeWheel(interval time.Duration, bucketCount int, cleanupFunc func(key string)) *TimeWheel {
	if bucketCount <= 0 {
		bucketCount = 3600 // 默认3600个桶
	}

	tw := &TimeWheel{
		interval:    interval,
		buckets:     make([]*bucket, bucketCount),
		bucketCount: bucketCount,
		currentPos:  0,
		cleanupFunc: cleanupFunc,
		stopChan:    make(chan struct{}),
	}

	for i := range tw.buckets {
		tw.buckets[i] = &bucket{
			items: make(map[string]*time.Timer),
		}
	}

	tw.ticker = time.NewTicker(interval)
	go tw.run()

	return tw
}

// Add 添加或更新TTL项
func (tw *TimeWheel) Add(key string, ttl time.Duration) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	// 计算应该放入的桶位置
	ticks := int(ttl / tw.interval)
	if ticks <= 0 {
		ticks = 1
	}

	pos := (tw.currentPos + ticks) % tw.bucketCount
	bucket := tw.buckets[pos]

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 如果已存在，先移除旧的定时器
	if timer, exists := bucket.items[key]; exists {
		timer.Stop()
	}

	// 创建新的定时器
	timer := time.AfterFunc(ttl, func() {
		if tw.cleanupFunc != nil {
			tw.cleanupFunc(key)
		}
	})

	bucket.items[key] = timer
}

// Remove 移除TTL项
func (tw *TimeWheel) Remove(key string) {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	for _, bucket := range tw.buckets {
		bucket.mu.Lock()
		if timer, exists := bucket.items[key]; exists {
			timer.Stop()
			delete(bucket.items, key)
		}
		bucket.mu.Unlock()
	}
}

// run 时间轮运行循环
func (tw *TimeWheel) run() {
	for {
		select {
		case <-tw.ticker.C:
			tw.advance()
		case <-tw.stopChan:
			return
		}
	}
}

// advance 推进时间轮
func (tw *TimeWheel) advance() {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.currentPos = (tw.currentPos + 1) % tw.bucketCount
	bucket := tw.buckets[tw.currentPos]

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 清理当前桶中的所有定时器
	for key, timer := range bucket.items {
		timer.Stop()
		delete(bucket.items, key)
	}
}

// Stop 停止时间轮
func (tw *TimeWheel) Stop() {
	tw.ticker.Stop()
	close(tw.stopChan)
}

// Stats 获取统计信息
func (tw *TimeWheel) Stats() map[string]int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	stats := make(map[string]int)
	total := 0

	for i, bucket := range tw.buckets {
		bucket.mu.RLock()
		count := len(bucket.items)
		stats[fmt.Sprintf("bucket_%d", i)] = count
		total += count
		bucket.mu.RUnlock()
	}

	stats["total"] = total
	stats["current_pos"] = tw.currentPos

	return stats
}
