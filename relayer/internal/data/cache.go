package data

import (
	"sync"
	"time"
)

type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

type Cache struct {
	items map[string]*CacheEntry
	mu    sync.RWMutex
}

func NewCache(defaultTTL time.Duration) *Cache {
	c := &Cache{items: make(map[string]*CacheEntry)}
	go c.cleanup(defaultTTL)
	return c
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

func (c *Cache) cleanup(ttl time.Duration) {
	ticker := time.NewTicker(ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.ExpiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

type TTLCache struct {
	entries map[string]*ttlEntry
	mu      sync.RWMutex
}

type ttlEntry struct {
	value     interface{}
	expiresAt time.Time
}

func NewTTLCache() *TTLCache {
	return &TTLCache{entries: make(map[string]*ttlEntry)}
}

func (tc *TTLCache) Set(key string, value interface{}, ttl time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.entries[key] = &ttlEntry{value: value, expiresAt: time.Now().Add(ttl)}
}

func (tc *TTLCache) Get(key string) (interface{}, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	entry, ok := tc.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (tc *TTLCache) SetDefault(key string, value interface{}) {
	tc.Set(key, value, 5*time.Minute)
}

func (tc *TTLCache) Clean() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	now := time.Now()
	count := 0
	for k, v := range tc.entries {
		if now.After(v.expiresAt) {
			delete(tc.entries, k)
			count++
		}
	}
	return count
}

func (tc *TTLCache) Size() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.entries)
}

type WriteBuffer[T any] struct {
	batch    []T
	batchSize int
	flushFn  func([]T)
	mu       sync.Mutex
}

func NewWriteBuffer[T any](batchSize int, flushFn func([]T)) *WriteBuffer[T] {
	return &WriteBuffer[T]{
		batch:     make([]T, 0, batchSize),
		batchSize: batchSize,
		flushFn:   flushFn,
	}
}

func (wb *WriteBuffer[T]) Add(item T) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.batch = append(wb.batch, item)
	if len(wb.batch) >= wb.batchSize {
		wb.flush()
	}
}

func (wb *WriteBuffer[T]) Flush() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	if len(wb.batch) > 0 {
		wb.flush()
	}
}

func (wb *WriteBuffer[T]) flush() {
	batch := wb.batch
	wb.batch = make([]T, 0, wb.batchSize)
	go wb.flushFn(batch)
}

func (wb *WriteBuffer[T]) Size() int {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return len(wb.batch)
}

type BloomCache struct {
	cache *Cache
	bloom *BloomFilter
	mu    sync.RWMutex
}

func NewBloomCache(cacheSize, bloomSize, bloomHash int) *BloomCache {
	return &BloomCache{
		cache: NewCache(time.Hour),
		bloom: NewBloomFilter(bloomSize, bloomHash),
	}
}

func (bc *BloomCache) Set(key string, value interface{}, ttl time.Duration) {
	bc.cache.Set(key, value, ttl)
	bc.bloom.Add(key)
}

func (bc *BloomCache) Get(key string) (interface{}, bool) {
	if !bc.bloom.MayContain(key) {
		return nil, false
	}
	return bc.cache.Get(key)
}
