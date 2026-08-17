package cache

import (
	"container/list"
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type Cache[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, bool)
	Set(ctx context.Context, key K, value V, opts ...Option) error
	Delete(ctx context.Context, key K) error
	Exists(ctx context.Context, key K) bool
	Clear(ctx context.Context)
	Size() int
	Keys() []K
}

type Option func(*options)

type options struct {
	TTL         time.Duration
	Priority    int
	Tags        []string
	Cost        int64
	OnEvicted   func(key interface{}, value interface{})
}

func WithTTL(ttl time.Duration) Option {
	return func(o *options) { o.TTL = ttl }
}

func WithPriority(priority int) Option {
	return func(o *options) { o.Priority = priority }
}

func WithTags(tags ...string) Option {
	return func(o *options) { o.Tags = tags }
}

func WithCost(cost int64) Option {
	return func(o *options) { o.Cost = cost }
}

func WithOnEvicted(fn func(key interface{}, value interface{})) Option {
	return func(o *options) { o.OnEvicted = fn }
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	tags      []string
	priority  int
	cost      int64
	expiresAt time.Time
	createdAt time.Time
	accessAt  time.Time
	freq      int64
}

func (e *entry[K, V]) isExpired() bool {
	return !e.expiresAt.IsZero() && time.Now().After(e.expiresAt)
}

type LRUCache[K comparable, V any] struct {
	capacity  int
	items     map[K]*list.Element
	order     *list.List
	mu        sync.RWMutex
	onEvicted func(K, V)
	stats     *CacheStats
}

type CacheStats struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	Sets        int64
	Deletes     int64
	HitRate     float64
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 1000
	}
	return &LRUCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element),
		order:    list.New(),
		stats:    &CacheStats{},
	}
}

func (c *LRUCache[K, V]) WithOnEvicted(fn func(K, V)) *LRUCache[K, V] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvicted = fn
	return c
}

type cacheEntry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

func (c *LRUCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.stats.Misses++
		c.updateHitRate()
		var zero V
		return zero, false
	}

	e := elem.Value.(*cacheEntry[K, V])
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.removeElement(elem)
		c.stats.Misses++
		c.updateHitRate()
		var zero V
		return zero, false
	}

	c.order.MoveToFront(elem)
	c.stats.Hits++
	c.updateHitRate()
	return e.value, true
}

func (c *LRUCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		e := elem.Value.(*cacheEntry[K, V])
		e.value = value
		if o.TTL > 0 {
			e.expiresAt = time.Now().Add(o.TTL)
		}
		c.stats.Sets++
		return nil
	}

	if c.order.Len() >= c.capacity {
		c.evict()
	}

	e := &cacheEntry[K, V]{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(o.TTL),
	}
	elem := c.order.PushFront(e)
	c.items[key] = elem
	c.stats.Sets++
	return nil
}

func (c *LRUCache[K, V]) Delete(ctx context.Context, key K) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
		c.stats.Deletes++
	}
	return nil
}

func (c *LRUCache[K, V]) Exists(ctx context.Context, key K) bool {
	c.mu.RLock()
	elem, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return false
	}
	e := elem.Value.(*cacheEntry[K, V])
	return e.expiresAt.IsZero() || time.Now().Before(e.expiresAt)
}

func (c *LRUCache[K, V]) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*list.Element)
	c.order.Init()
}

func (c *LRUCache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

func (c *LRUCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

func (c *LRUCache[K, V]) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.stats
}

func (c *LRUCache[K, V]) evict() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *LRUCache[K, V]) removeElement(elem *list.Element) {
	c.order.Remove(elem)
	e := elem.Value.(*cacheEntry[K, V])
	delete(c.items, e.key)
	c.stats.Evictions++
	if c.onEvicted != nil {
		c.onEvicted(e.key, e.value)
	}
}

func (c *LRUCache[K, V]) updateHitRate() {
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total)
	}
}

type LFUCache[K comparable, V any] struct {
	capacity  int
	items     map[K]*list.Element
	freqMap   map[int]*list.List
	minFreq   int
	mu        sync.RWMutex
	stats     *CacheStats
	onEvicted func(K, V)
}

type lfuEntry[K comparable, V any] struct {
	key       K
	value     V
	freq      int
	expiresAt time.Time
}

func NewLFUCache[K comparable, V any](capacity int) *LFUCache[K, V] {
	if capacity <= 0 {
		capacity = 1000
	}
	return &LFUCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element),
		freqMap:  make(map[int]*list.List),
		minFreq:  1,
		stats:    &CacheStats{},
	}
}

func (c *LFUCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.stats.Misses++
		var zero V
		return zero, false
	}

	e := elem.Value.(*lfuEntry[K, V])
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.removeElement(e)
		c.stats.Misses++
		var zero V
		return zero, false
	}

	c.increaseFreq(e)
	c.stats.Hits++
	return e.value, true
}

func (c *LFUCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*lfuEntry[K, V])
		e.value = value
		if o.TTL > 0 {
			e.expiresAt = time.Now().Add(o.TTL)
		}
		c.increaseFreq(e)
		c.stats.Sets++
		return nil
	}

	if len(c.items) >= c.capacity {
		c.evict()
	}

	e := &lfuEntry[K, V]{key: key, value: value, freq: 1, expiresAt: time.Now().Add(o.TTL)}
	freqList := c.getFreqList(1)
	elem := freqList.PushFront(e)
	c.items[key] = elem
	c.minFreq = 1
	c.stats.Sets++
	return nil
}

func (c *LFUCache[K, V]) Delete(ctx context.Context, key K) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*lfuEntry[K, V])
		c.removeElement(e)
	}
	return nil
}

func (c *LFUCache[K, V]) Exists(ctx context.Context, key K) bool {
	c.mu.RLock()
	elem, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return false
	}
	e := elem.Value.(*lfuEntry[K, V])
	return e.expiresAt.IsZero() || time.Now().Before(e.expiresAt)
}

func (c *LFUCache[K, V]) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*list.Element)
	c.freqMap = make(map[int]*list.List)
	c.minFreq = 1
}

func (c *LFUCache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *LFUCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

func (c *LFUCache[K, V]) getFreqList(freq int) *list.List {
	if l, ok := c.freqMap[freq]; ok {
		return l
	}
	l := list.New()
	c.freqMap[freq] = l
	return l
}

type lfulistElement[K comparable, V any] struct {
	entry *lfuEntry[K, V]
	elem  *list.Element
}

func (c *LFUCache[K, V]) increaseFreq(e *lfuEntry[K, V]) {
	oldFreq := e.freq
	freqList := c.freqMap[oldFreq]
	if freqList != nil {
		freqList.Init()
	}
	e.freq++
	c.getFreqList(e.freq)
	if freqList != nil && freqList.Len() == 0 {
		delete(c.freqMap, oldFreq)
		if c.minFreq == oldFreq {
			c.minFreq++
		}
	}
}

func (c *LFUCache[K, V]) removeElement(e *lfuEntry[K, V]) {
	freqList := c.freqMap[e.freq]
	if freqList != nil {
		freqList.Init()
	}
	delete(c.items, e.key)
	c.stats.Evictions++
	if c.onEvicted != nil {
		c.onEvicted(e.key, e.value)
	}
	if freqList != nil && freqList.Len() == 0 {
		delete(c.freqMap, e.freq)
		if c.minFreq == e.freq {
			c.minFreq++
		}
	}
}

func (c *LFUCache[K, V]) evict() {
	freqList := c.freqMap[c.minFreq]
	if freqList == nil || freqList.Len() == 0 {
		return
	}
	elem := freqList.Back()
	if elem != nil {
		e := elem.Value.(*lfuEntry[K, V])
		c.removeElement(e)
	}
}

func (e *lfuEntry[K, V]) listElement() *list.Element {
	return nil
}

type ARCCache[K comparable, V any] struct {
	capacity int
	t1       *list.List
	t2       *list.List
	b1       *list.List
	b2       *list.List
	items    map[K]*list.Element
	p        int
	mu       sync.RWMutex
	stats    *CacheStats
}

type arcEntry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

func NewARCCache[K comparable, V any](capacity int) *ARCCache[K, V] {
	if capacity <= 0 {
		capacity = 1000
	}
	return &ARCCache[K, V]{
		capacity: capacity,
		t1:       list.New(),
		t2:       list.New(),
		b1:       list.New(),
		b2:       list.New(),
		items:    make(map[K]*list.Element),
		stats:    &CacheStats{},
	}
}

func (c *ARCCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*arcEntry[K, V])
		c.t2.MoveToFront(elem)
		c.stats.Hits++
		return e.value, true
	}

	if elem := c.b1Lookup(key); elem != nil {
		delta := 1
		if c.b2.Len() > c.b1.Len() {
			delta = c.b2.Len() / c.b1.Len()
		}
		c.p = min(c.p+delta, c.capacity)
		c.b1.Remove(elem)
		e := &arcEntry[K, V]{key: key}
		elem = c.t2.PushFront(e)
		c.items[key] = elem
		c.stats.Misses++
		var zero V
		return zero, false
	}

	if elem := c.b2Lookup(key); elem != nil {
		delta := 1
		if c.b1.Len() > c.b2.Len() {
			delta = c.b1.Len() / c.b2.Len()
		}
		c.p = max(c.p-delta, 0)
		c.b2.Remove(elem)
		e := &arcEntry[K, V]{key: key}
		elem = c.t2.PushFront(e)
		c.items[key] = elem
		c.stats.Misses++
		var zero V
		return zero, false
	}

	c.stats.Misses++
	var zero V
	return zero, false
}

func (c *ARCCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*arcEntry[K, V])
		e.value = value
		c.t2.MoveToFront(elem)
		c.stats.Sets++
		return nil
	}

	l1 := c.t1.Len() + c.b1.Len()
	l2 := c.t2.Len() + c.b2.Len()
	if l1 >= c.capacity && c.t1.Len() > 0 {
		c.replace(true)
	} else if l1+l2 >= 2*c.capacity {
		if l1+l2 >= 2*c.capacity {
			if l2 > 0 {
				c.replace(false)
			}
		}
	}

	e := &arcEntry[K, V]{key: key, value: value}
	elem := c.t2.PushFront(e)
	c.items[key] = elem
	c.stats.Sets++
	return nil
}

func (c *ARCCache[K, V]) Delete(ctx context.Context, key K) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.t1.Remove(elem)
		c.t2.Remove(elem)
		delete(c.items, key)
	}
	return nil
}

func (c *ARCCache[K, V]) Exists(ctx context.Context, key K) bool {
	c.mu.RLock()
	_, ok := c.items[key]
	c.mu.RUnlock()
	return ok
}

func (c *ARCCache[K, V]) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t1.Init()
	c.t2.Init()
	c.b1.Init()
	c.b2.Init()
	c.items = make(map[K]*list.Element)
	c.p = 0
}

func (c *ARCCache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.t1.Len() + c.t2.Len()
}

func (c *ARCCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

func (c *ARCCache[K, V]) b1Lookup(key K) *list.Element {
	for e := c.b1.Front(); e != nil; e = e.Next() {
		if e.Value.(*arcEntry[K, V]).key == key {
			return e
		}
	}
	return nil
}

func (c *ARCCache[K, V]) b2Lookup(key K) *list.Element {
	for e := c.b2.Front(); e != nil; e = e.Next() {
		if e.Value.(*arcEntry[K, V]).key == key {
			return e
		}
	}
	return nil
}

func (c *ARCCache[K, V]) replace(inB1 bool) {
	if c.t1.Len() > 0 && (inB1 || c.t1.Len() > c.p) {
		elem := c.t1.Back()
		if elem != nil {
			c.t1.Remove(elem)
			e := elem.Value.(*arcEntry[K, V])
			c.b1.PushFront(&arcEntry[K, V]{key: e.key})
			delete(c.items, e.key)
			c.stats.Evictions++
		}
	} else if c.t2.Len() > 0 {
		elem := c.t2.Back()
		if elem != nil {
			c.t2.Remove(elem)
			e := elem.Value.(*arcEntry[K, V])
			c.b2.PushFront(&arcEntry[K, V]{key: e.key})
			delete(c.items, e.key)
			c.stats.Evictions++
		}
	}
}

type WriteThroughCache[K comparable, V any] struct {
	inner Cache[K, V]
	store Store[K, V]
}

type Store[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, error)
	Set(ctx context.Context, key K, value V) error
	Delete(ctx context.Context, key K) error
}

func NewWriteThroughCache[K comparable, V any](inner Cache[K, V], store Store[K, V]) *WriteThroughCache[K, V] {
	return &WriteThroughCache[K, V]{inner: inner, store: store}
}

func (c *WriteThroughCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	if val, ok := c.inner.Get(ctx, key); ok {
		return val, true
	}
	val, err := c.store.Get(ctx, key)
	if err != nil {
		var zero V
		return zero, false
	}
	_ = c.inner.Set(ctx, key, val)
	return val, true
}

func (c *WriteThroughCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	if err := c.store.Set(ctx, key, value); err != nil {
		return err
	}
	return c.inner.Set(ctx, key, value, opts...)
}

func (c *WriteThroughCache[K, V]) Delete(ctx context.Context, key K) error {
	if err := c.store.Delete(ctx, key); err != nil {
		return err
	}
	return c.inner.Delete(ctx, key)
}

func (c *WriteThroughCache[K, V]) Exists(ctx context.Context, key K) bool {
	return c.inner.Exists(ctx, key)
}

func (c *WriteThroughCache[K, V]) Clear(ctx context.Context) {
	c.inner.Clear(ctx)
}

func (c *WriteThroughCache[K, V]) Size() int {
	return c.inner.Size()
}

func (c *WriteThroughCache[K, V]) Keys() []K {
	return c.inner.Keys()
}

type WriteBehindCache[K comparable, V any] struct {
	inner     Cache[K, V]
	store     Store[K, V]
	writeCh   chan writeOp[K, V]
	flushInterval time.Duration
	mu        sync.Mutex
	closed    bool
}

type writeOp[K comparable, V any] struct {
	key   K
	value V
	op    string
}

func NewWriteBehindCache[K comparable, V any](inner Cache[K, V], store Store[K, V], flushInterval time.Duration, bufSize int) *WriteBehindCache[K, V] {
	if bufSize <= 0 {
		bufSize = 1000
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	wb := &WriteBehindCache[K, V]{
		inner:         inner,
		store:         store,
		writeCh:       make(chan writeOp[K, V], bufSize),
		flushInterval: flushInterval,
	}
	go wb.flushLoop()
	return wb
}

func (c *WriteBehindCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	return c.inner.Get(ctx, key)
}

func (c *WriteBehindCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	if err := c.inner.Set(ctx, key, value, opts...); err != nil {
		return err
	}
	select {
	case c.writeCh <- writeOp[K, V]{key: key, value: value, op: "set"}:
	default:
	}
	return nil
}

func (c *WriteBehindCache[K, V]) Delete(ctx context.Context, key K) error {
	if err := c.inner.Delete(ctx, key); err != nil {
		return err
	}
	select {
	case c.writeCh <- writeOp[K, V]{key: key, op: "delete"}:
	default:
	}
	return nil
}

func (c *WriteBehindCache[K, V]) Exists(ctx context.Context, key K) bool {
	return c.inner.Exists(ctx, key)
}

func (c *WriteBehindCache[K, V]) Clear(ctx context.Context) {
	c.inner.Clear(ctx)
}

func (c *WriteBehindCache[K, V]) Size() int {
	return c.inner.Size()
}

func (c *WriteBehindCache[K, V]) Keys() []K {
	return c.inner.Keys()
}

func (c *WriteBehindCache[K, V]) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	close(c.writeCh)
}

func (c *WriteBehindCache[K, V]) flushLoop() {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()
	batch := make([]writeOp[K, V], 0)
	for {
		select {
		case op, ok := <-c.writeCh:
			if !ok {
				c.flushBatch(batch)
				return
			}
			batch = append(batch, op)
			if len(batch) >= 100 {
				c.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			c.flushBatch(batch)
			batch = batch[:0]
		}
	}
}

func (c *WriteBehindCache[K, V]) flushBatch(batch []writeOp[K, V]) {
	ctx := context.Background()
	for _, op := range batch {
		switch op.op {
		case "set":
			_ = c.store.Set(ctx, op.key, op.value)
		case "delete":
			_ = c.store.Delete(ctx, op.key)
		}
	}
}

type ReadThroughCache[K comparable, V any] struct {
	inner  Cache[K, V]
	loader Loader[K, V]
}

type Loader[K comparable, V any] interface {
	Load(ctx context.Context, key K) (V, error)
}

func NewReadThroughCache[K comparable, V any](inner Cache[K, V], loader Loader[K, V]) *ReadThroughCache[K, V] {
	return &ReadThroughCache[K, V]{inner: inner, loader: loader}
}

func (c *ReadThroughCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	if val, ok := c.inner.Get(ctx, key); ok {
		return val, true
	}
	val, err := c.loader.Load(ctx, key)
	if err != nil {
		var zero V
		return zero, false
	}
	_ = c.inner.Set(ctx, key, val)
	return val, true
}

func (c *ReadThroughCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	return c.inner.Set(ctx, key, value, opts...)
}

func (c *ReadThroughCache[K, V]) Delete(ctx context.Context, key K) error {
	return c.inner.Delete(ctx, key)
}

func (c *ReadThroughCache[K, V]) Exists(ctx context.Context, key K) bool {
	return c.inner.Exists(ctx, key)
}

func (c *ReadThroughCache[K, V]) Clear(ctx context.Context) {
	c.inner.Clear(ctx)
}

func (c *ReadThroughCache[K, V]) Size() int {
	return c.inner.Size()
}

func (c *ReadThroughCache[K, V]) Keys() []K {
	return c.inner.Keys()
}

type MultiTierCache[K comparable, V any] struct {
	tiers   []Cache[K, V]
	loader  Loader[K, V]
	mu      sync.RWMutex
	stats   *CacheStats
}

func NewMultiTierCache[K comparable, V any](tiers []Cache[K, V], loader Loader[K, V]) *MultiTierCache[K, V] {
	return &MultiTierCache[K, V]{
		tiers:  tiers,
		loader: loader,
		stats:  &CacheStats{},
	}
}

func (c *MultiTierCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	for i, tier := range c.tiers {
		if val, ok := tier.Get(ctx, key); ok {
			for j := 0; j < i; j++ {
				_ = c.tiers[j].Set(ctx, key, val)
			}
			c.stats.Hits++
			return val, true
		}
	}
	if c.loader != nil {
		val, err := c.loader.Load(ctx, key)
		if err == nil {
			for _, tier := range c.tiers {
				_ = tier.Set(ctx, key, val)
			}
			c.stats.Misses++
			return val, true
		}
	}
	c.stats.Misses++
	var zero V
	return zero, false
}

func (c *MultiTierCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	for _, tier := range c.tiers {
		if err := tier.Set(ctx, key, value, opts...); err != nil {
			return err
		}
	}
	c.stats.Sets++
	return nil
}

func (c *MultiTierCache[K, V]) Delete(ctx context.Context, key K) error {
	for _, tier := range c.tiers {
		if err := tier.Delete(ctx, key); err != nil {
			return err
		}
	}
	c.stats.Deletes++
	return nil
}

func (c *MultiTierCache[K, V]) Exists(ctx context.Context, key K) bool {
	for _, tier := range c.tiers {
		if tier.Exists(ctx, key) {
			return true
		}
	}
	return false
}

func (c *MultiTierCache[K, V]) Clear(ctx context.Context) {
	for _, tier := range c.tiers {
		tier.Clear(ctx)
	}
}

func (c *MultiTierCache[K, V]) Size() int {
	return c.tiers[0].Size()
}

func (c *MultiTierCache[K, V]) Keys() []K {
	return c.tiers[0].Keys()
}

type ShardedCache[K comparable, V any] struct {
	shards   []*LRUCache[string, V]
	numShards int
}

func NewShardedCache[K comparable, V any](numShards, capacityPerShard int) *ShardedCache[K, V] {
	if numShards <= 0 {
		numShards = 16
	}
	shards := make([]*LRUCache[string, V], numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = NewLRUCache[string, V](capacityPerShard)
	}
	return &ShardedCache[K, V]{
		shards:    shards,
		numShards: numShards,
	}
}

func (c *ShardedCache[K, V]) getShard(key K) *LRUCache[string, V] {
	h := fnv.New32a()
	fmt.Fprintf(h, "%v", key)
	return c.shards[h.Sum32()%uint32(c.numShards)]
}

func (c *ShardedCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	shard := c.getShard(key)
	keyStr := fmt.Sprintf("%v", key)
	return shard.Get(ctx, keyStr)
}

func (c *ShardedCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	shard := c.getShard(key)
	keyStr := fmt.Sprintf("%v", key)
	return shard.Set(ctx, keyStr, value, opts...)
}

func (c *ShardedCache[K, V]) Delete(ctx context.Context, key K) error {
	shard := c.getShard(key)
	keyStr := fmt.Sprintf("%v", key)
	return shard.Delete(ctx, keyStr)
}

func (c *ShardedCache[K, V]) Exists(ctx context.Context, key K) bool {
	shard := c.getShard(key)
	keyStr := fmt.Sprintf("%v", key)
	return shard.Exists(ctx, keyStr)
}

func (c *ShardedCache[K, V]) Clear(ctx context.Context) {
	for _, shard := range c.shards {
		shard.Clear(ctx)
	}
}

func (c *ShardedCache[K, V]) Size() int {
	total := 0
	for _, shard := range c.shards {
		total += shard.Size()
	}
	return total
}

func (c *ShardedCache[K, V]) Keys() []K {
	var allKeys []K
	for _, shard := range c.shards {
		for range shard.Keys() {
		}
	}
	return allKeys
}

type CacheManager[K comparable, V any] struct {
	caches map[string]Cache[K, V]
	mu     sync.RWMutex
}

func NewCacheManager[K comparable, V any]() *CacheManager[K, V] {
	return &CacheManager[K, V]{caches: make(map[string]Cache[K, V])}
}

func (cm *CacheManager[K, V]) Register(name string, cache Cache[K, V]) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.caches[name] = cache
}

func (cm *CacheManager[K, V]) Get(name string) (Cache[K, V], bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	c, ok := cm.caches[name]
	return c, ok
}

func (cm *CacheManager[K, V]) InvalidateAll(ctx context.Context) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, cache := range cm.caches {
		cache.Clear(ctx)
	}
}

type CacheWarmer[K comparable, V any] struct {
	cache  Cache[K, V]
	loader Loader[K, V]
	keys   []K
}

func NewCacheWarmer[K comparable, V any](cache Cache[K, V], loader Loader[K, V]) *CacheWarmer[K, V] {
	return &CacheWarmer[K, V]{cache: cache, loader: loader}
}

func (w *CacheWarmer[K, V]) AddKey(key K) {
	w.keys = append(w.keys, key)
}

func (w *CacheWarmer[K, V]) AddKeys(keys []K) {
	w.keys = append(w.keys, keys...)
}

func (w *CacheWarmer[K, V]) Warm(ctx context.Context) error {
	for _, key := range w.keys {
		val, err := w.loader.Load(ctx, key)
		if err != nil {
			continue
		}
		if err := w.cache.Set(ctx, key, val); err != nil {
			return fmt.Errorf("warm key %v: %w", key, err)
		}
	}
	return nil
}

type EvictionPolicy int

const (
	EvictionLRU EvictionPolicy = iota
	EvictionLFU
	EvictionFIFO
	EvictionRandom
	EvictionTTL
	EvictionSizeBased
)

type AdaptiveCache[K comparable, V any] struct {
	items     map[K]*adaptiveEntry[K, V]
	policy    EvictionPolicy
	capacity  int
	hits      int64
	misses    int64
	mu        sync.RWMutex
}

type adaptiveEntry[K comparable, V any] struct {
	key       K
	value     V
	freq      int64
	createdAt time.Time
	accessAt  time.Time
	cost      int64
}

func NewAdaptiveCache[K comparable, V any](capacity int, policy EvictionPolicy) *AdaptiveCache[K, V] {
	if capacity <= 0 {
		capacity = 1000
	}
	return &AdaptiveCache[K, V]{
		items:    make(map[K]*adaptiveEntry[K, V]),
		policy:   policy,
		capacity: capacity,
	}
}

func (c *AdaptiveCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		c.misses++
		var zero V
		return zero, false
	}

	item.freq++
	item.accessAt = time.Now()
	c.hits++
	return item.value, true
}

func (c *AdaptiveCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if _, ok := c.items[key]; ok {
		c.items[key].value = value
		c.items[key].accessAt = time.Now()
		return nil
	}

	if len(c.items) >= c.capacity {
		c.evict()
	}

	c.items[key] = &adaptiveEntry[K, V]{
		key:       key,
		value:     value,
		createdAt: time.Now(),
		accessAt:  time.Now(),
		cost:      o.Cost,
	}
	return nil
}

func (c *AdaptiveCache[K, V]) Delete(ctx context.Context, key K) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

func (c *AdaptiveCache[K, V]) Exists(ctx context.Context, key K) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.items[key]
	return ok
}

func (c *AdaptiveCache[K, V]) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*adaptiveEntry[K, V])
}

func (c *AdaptiveCache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *AdaptiveCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

func (c *AdaptiveCache[K, V]) evict() {
	switch c.policy {
	case EvictionLRU:
		c.evictLRU()
	case EvictionLFU:
		c.evictLFU()
	case EvictionFIFO:
		c.evictFIFO()
	case EvictionRandom:
		c.evictRandom()
	case EvictionSizeBased:
		c.evictSizeBased()
	default:
		c.evictLRU()
	}
}

func (c *AdaptiveCache[K, V]) evictLRU() {
	var oldest *adaptiveEntry[K, V]
	var oldestKey K
	for k, v := range c.items {
		if oldest == nil || v.accessAt.Before(oldest.accessAt) {
			oldest = v
			oldestKey = k
		}
	}
	if oldest != nil {
		delete(c.items, oldestKey)
	}
}

func (c *AdaptiveCache[K, V]) evictLFU() {
	var least *adaptiveEntry[K, V]
	var leastKey K
	for k, v := range c.items {
		if least == nil || v.freq < least.freq {
			least = v
			leastKey = k
		}
	}
	if least != nil {
		delete(c.items, leastKey)
	}
}

func (c *AdaptiveCache[K, V]) evictFIFO() {
	var oldest *adaptiveEntry[K, V]
	var oldestKey K
	for k, v := range c.items {
		if oldest == nil || v.createdAt.Before(oldest.createdAt) {
			oldest = v
			oldestKey = k
		}
	}
	if oldest != nil {
		delete(c.items, oldestKey)
	}
}

func (c *AdaptiveCache[K, V]) evictRandom() {
	for k := range c.items {
		delete(c.items, k)
		return
	}
}

func (c *AdaptiveCache[K, V]) evictSizeBased() {
	var largest *adaptiveEntry[K, V]
	var largestKey K
	for k, v := range c.items {
		if largest == nil || v.cost > largest.cost {
			largest = v
			largestKey = k
		}
	}
	if largest != nil {
		delete(c.items, largestKey)
	}
}

func (c *AdaptiveCache[K, V]) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

type CacheEvent int

const (
	CacheEventHit CacheEvent = iota
	CacheEventMiss
	CacheEventSet
	CacheEventDelete
	CacheEventEvict
)

type CacheEventListener[K comparable, V any] interface {
	OnEvent(event CacheEvent, key K, value interface{})
}

type ObservableCache[K comparable, V any] struct {
	inner     Cache[K, V]
	listeners []CacheEventListener[K, V]
	mu        sync.RWMutex
}

func NewObservableCache[K comparable, V any](inner Cache[K, V]) *ObservableCache[K, V] {
	return &ObservableCache[K, V]{inner: inner}
}

func (c *ObservableCache[K, V]) AddListener(listener CacheEventListener[K, V]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listeners = append(c.listeners, listener)
}

func (c *ObservableCache[K, V]) notify(event CacheEvent, key K, value interface{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, l := range c.listeners {
		l.OnEvent(event, key, value)
	}
}

func (c *ObservableCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	val, ok := c.inner.Get(ctx, key)
	if ok {
		c.notify(CacheEventHit, key, val)
	} else {
		c.notify(CacheEventMiss, key, nil)
	}
	return val, ok
}

func (c *ObservableCache[K, V]) Set(ctx context.Context, key K, value V, opts ...Option) error {
	err := c.inner.Set(ctx, key, value, opts...)
	if err == nil {
		c.notify(CacheEventSet, key, value)
	}
	return err
}

func (c *ObservableCache[K, V]) Delete(ctx context.Context, key K) error {
	err := c.inner.Delete(ctx, key)
	if err == nil {
		c.notify(CacheEventDelete, key, nil)
	}
	return err
}

func (c *ObservableCache[K, V]) Exists(ctx context.Context, key K) bool {
	return c.inner.Exists(ctx, key)
}

func (c *ObservableCache[K, V]) Clear(ctx context.Context) {
	c.inner.Clear(ctx)
}

func (c *ObservableCache[K, V]) Size() int {
	return c.inner.Size()
}

func (c *ObservableCache[K, V]) Keys() []K {
	return c.inner.Keys()
}

type BloomFilter struct {
	bits    []uint64
	size    uint64
	hashes  int
	count   uint64
	mu      sync.RWMutex
}

func NewBloomFilter(expectedItems int, falsePositiveRate float64) *BloomFilter {
	size := optimalSize(expectedItems, falsePositiveRate)
	hashes := optimalHashes(size, expectedItems)
	return &BloomFilter{
		bits:   make([]uint64, (size+63)/64),
		size:   size,
		hashes: hashes,
	}
}

func optimalSize(n int, p float64) uint64 {
	m := -float64(n) * math.Log(p) / (math.Log(2) * math.Log(2))
	return uint64(m)
}

func optimalHashes(m uint64, n int) int {
	return int(float64(m) / float64(n) * math.Log(2))
}

func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	for i := 0; i < bf.hashes; i++ {
		idx := bf.hash(item, i)
		word := idx / 64
		bit := idx % 64
		bf.bits[word] |= (1 << bit)
	}
	bf.count++
}

func (bf *BloomFilter) Contains(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	for i := 0; i < bf.hashes; i++ {
		idx := bf.hash(item, i)
		word := idx / 64
		bit := idx % 64
		if bf.bits[word]&(1<<bit) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) hash(item string, i int) uint64 {
	h := fnv.New64a()
	h.Write([]byte(item))
	h64 := h.Sum64()
	return (h64 + uint64(i)*(h64>>32)) % bf.size
}

func (bf *BloomFilter) Count() uint64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.count
}

func (bf *BloomFilter) FalsePositiveRate() float64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return math.Pow(1-math.Exp(-float64(bf.hashes)*float64(bf.count)/float64(bf.size)), float64(bf.hashes))
}

type CacheMetrics struct {
	Hits          int64
	Misses        int64
	Sets          int64
	Deletes       int64
	Evictions     int64
	TotalItems    int64
	MemoryUsage   int64
	AvgLatency    time.Duration
	P99Latency    time.Duration
	HitRate       float64
	Timestamp     time.Time
	mu            sync.RWMutex
}

func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{Timestamp: time.Now()}
}

func (cm *CacheMetrics) RecordHit() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Hits++
	cm.recalculate()
}

func (cm *CacheMetrics) RecordMiss() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Misses++
	cm.recalculate()
}

func (cm *CacheMetrics) RecordSet() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Sets++
}

func (cm *CacheMetrics) RecordDelete() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Deletes++
}

func (cm *CacheMetrics) RecordEviction() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Evictions++
}

func (cm *CacheMetrics) recalculate() {
	total := cm.Hits + cm.Misses
	if total > 0 {
		cm.HitRate = float64(cm.Hits) / float64(total)
	}
	cm.Timestamp = time.Now()
}

func (cm *CacheMetrics) Snapshot() CacheMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return CacheMetrics{
		Hits:        cm.Hits,
		Misses:      cm.Misses,
		Sets:        cm.Sets,
		Deletes:     cm.Deletes,
		Evictions:   cm.Evictions,
		TotalItems:  cm.TotalItems,
		MemoryUsage: cm.MemoryUsage,
		AvgLatency:  cm.AvgLatency,
		P99Latency:  cm.P99Latency,
		HitRate:     cm.HitRate,
		Timestamp:   cm.Timestamp,
	}
}

type CacheConfig struct {
	MaxSize          int
	DefaultTTL       time.Duration
	EvictionPolicy   string
	CleanupInterval  time.Duration
	EnableMetrics    bool
	MetricsInterval  time.Duration
	OnEvicted        func(key, value interface{})
	OnExpired        func(key interface{})
	EnableCompression bool
	CompressionLevel  int
	ShardCount       int
	MaxMemoryBytes   int64
	EagerExpiration  bool
}

func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize:         10000,
		DefaultTTL:      5 * time.Minute,
		EvictionPolicy:  "lru",
		CleanupInterval: 1 * time.Minute,
		EnableMetrics:   true,
		MetricsInterval: 30 * time.Second,
		ShardCount:      16,
	}
}

type CacheBuilder[K comparable, V any] struct {
	config CacheConfig
}

func NewCacheBuilder[K comparable, V any]() *CacheBuilder[K, V] {
	return &CacheBuilder[K, V]{config: DefaultCacheConfig()}
}

func (b *CacheBuilder[K, V]) MaxSize(size int) *CacheBuilder[K, V] {
	b.config.MaxSize = size
	return b
}

func (b *CacheBuilder[K, V]) TTL(ttl time.Duration) *CacheBuilder[K, V] {
	b.config.DefaultTTL = ttl
	return b
}

func (b *CacheBuilder[K, V]) EvictionPolicy(policy string) *CacheBuilder[K, V] {
	b.config.EvictionPolicy = policy
	return b
}

func (b *CacheBuilder[K, V]) WithMetrics() *CacheBuilder[K, V] {
	b.config.EnableMetrics = true
	return b
}

func (b *CacheBuilder[K, V]) WithShards(count int) *CacheBuilder[K, V] {
	b.config.ShardCount = count
	return b
}

func (b *CacheBuilder[K, V]) Build() Cache[K, V] {
	return NewLRUCache[K, V](b.config.MaxSize)
}

func init() {
	_ = sort.Strings
	_ = strings.TrimSpace
}
