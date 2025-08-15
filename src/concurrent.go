package main

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// ShardedLock 分段锁实现
type ShardedLock struct {
	shards []*sync.RWMutex
	count  uint32
}

// NewShardedLock 创建新的分段锁
func NewShardedLock(shardCount int) *ShardedLock {
	if shardCount <= 0 {
		shardCount = 16 // 默认16个分片
	}
	
	shards := make([]*sync.RWMutex, shardCount)
	for i := range shards {
		shards[i] = &sync.RWMutex{}
	}
	
	return &ShardedLock{
		shards: shards,
		count:  uint32(shardCount),
	}
}

// getShard 根据key获取对应的分片锁
func (sl *ShardedLock) getShard(key string) *sync.RWMutex {
	h := fnv.New32a()
	h.Write([]byte(key))
	index := h.Sum32() % sl.count
	return sl.shards[index]
}

// Lock 对指定key加写锁
func (sl *ShardedLock) Lock(key string) {
	sl.getShard(key).Lock()
}

// Unlock 对指定key解锁
func (sl *ShardedLock) Unlock(key string) {
	sl.getShard(key).Unlock()
}

// RLock 对指定key加读锁
func (sl *ShardedLock) RLock(key string) {
	sl.getShard(key).RLock()
}

// RUnlock 对指定key解读锁
func (sl *ShardedLock) RUnlock(key string) {
	sl.getShard(key).RUnlock()
}

// ConcurrentMap 并发安全的分段Map
type ConcurrentMap struct {
	shards []*shard
	count  uint32
}

type shard struct {
	items map[string]*KeyDir
	lock sync.RWMutex
}

// NewConcurrentMap 创建新的并发Map
func NewConcurrentMap(shardCount int) *ConcurrentMap {
	if shardCount <= 0 {
		shardCount = 16
	}
	
	shards := make([]*shard, shardCount)
	for i := range shards {
		shards[i] = &shard{
			items: make(map[string]*KeyDir),
		}
	}
	
	return &ConcurrentMap{
		shards: shards,
		count:  uint32(shardCount),
	}
}

// getShard 根据key获取对应的分片
func (cm *ConcurrentMap) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	index := h.Sum32() % cm.count
	return cm.shards[index]
}

// Get 获取值
func (cm *ConcurrentMap) Get(key string) (*KeyDir, bool) {
	shard := cm.getShard(key)
	shard.lock.RLock()
	defer shard.lock.RUnlock()
	
	val, ok := shard.items[key]
	return val, ok
}

// Set 设置值
func (cm *ConcurrentMap) Set(key string, value *KeyDir) {
	shard := cm.getShard(key)
	shard.lock.Lock()
	defer shard.lock.Unlock()
	
	shard.items[key] = value
}

// Delete 删除值
func (cm *ConcurrentMap) Delete(key string) {
	shard := cm.getShard(key)
	shard.lock.Lock()
	defer shard.lock.Unlock()
	
	delete(shard.items, key)
}

// Range 遍历所有键值对（读操作）
func (cm *ConcurrentMap) Range(f func(key string, value *KeyDir) bool) {
	for _, shard := range cm.shards {
		shard.lock.RLock()
		for key, value := range shard.items {
			if !f(key, value) {
				shard.lock.RUnlock()
				return
			}
		}
		shard.lock.RUnlock()
	}
}

// RangeWrite 遍历所有键值对（写操作）
func (cm *ConcurrentMap) RangeWrite(f func(key string, value *KeyDir) bool) {
	for _, shard := range cm.shards {
		shard.lock.Lock()
		for key, value := range shard.items {
			if !f(key, value) {
				shard.lock.Unlock()
				return
			}
		}
		shard.lock.Unlock()
	}
}

// Size 获取总大小
func (cm *ConcurrentMap) Size() int {
	size := 0
	for _, shard := range cm.shards {
		shard.lock.RLock()
		size += len(shard.items)
		shard.lock.RUnlock()
	}
	return size
}

// AtomicCounter 原子计数器
type AtomicCounter struct {
	value int64
}

// NewAtomicCounter 创建新的原子计数器
func NewAtomicCounter() *AtomicCounter {
	return &AtomicCounter{}
}

// Increment 原子递增
func (ac *AtomicCounter) Increment() int64 {
	return atomic.AddInt64(&ac.value, 1)
}

// Decrement 原子递减
func (ac *AtomicCounter) Decrement() int64 {
	return atomic.AddInt64(&ac.value, -1)
}

// Get 获取当前值
func (ac *AtomicCounter) Get() int64 {
	return atomic.LoadInt64(&ac.value)
}

// Set 设置值
func (ac *AtomicCounter) Set(value int64) {
	atomic.StoreInt64(&ac.value, value)
}

// WorkQueue 工作队列
type WorkQueue struct {
	workChan chan func()
	workers  int
	wg       sync.WaitGroup
	closed   int32
}

// NewWorkQueue 创建工作队列
func NewWorkQueue(workers int) *WorkQueue {
	if workers <= 0 {
		workers = 4
	}
	
	wq := &WorkQueue{
		workChan: make(chan func(), workers*2),
		workers:  workers,
	}
	
	wq.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go wq.worker()
	}
	
	return wq
}

// Submit 提交任务
func (wq *WorkQueue) Submit(work func()) bool {
	if atomic.LoadInt32(&wq.closed) == 1 {
		return false
	}
	
	select {
	case wq.workChan <- work:
		return true
	default:
		return false
	}
}

// Close 关闭工作队列
func (wq *WorkQueue) Close() {
	if atomic.CompareAndSwapInt32(&wq.closed, 0, 1) {
		close(wq.workChan)
		wq.wg.Wait()
	}
}

// worker 工作协程
func (wq *WorkQueue) worker() {
	defer wq.wg.Done()
	for work := range wq.workChan {
		if work != nil {
			work()
		}
	}
}