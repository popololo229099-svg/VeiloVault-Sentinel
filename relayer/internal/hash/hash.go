package hash

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"sync"
)

type ConsistentHash struct {
	ring       []uint32
	ringMap    map[uint32]string
	vnodes     int
	mu         sync.RWMutex
}

func NewConsistentHash(vnodes int) *ConsistentHash {
	if vnodes <= 0 {
		vnodes = 150
	}
	return &ConsistentHash{
		ring:    make([]uint32, 0),
		ringMap: make(map[uint32]string),
		vnodes:  vnodes,
	}
}

func (ch *ConsistentHash) AddNode(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.vnodes; i++ {
		h := fnv.New32a()
		key := fmt.Sprintf("%s#%d", node, i)
		h.Write([]byte(key))
		hash := h.Sum32()
		ch.ring = append(ch.ring, hash)
		ch.ringMap[hash] = node
	}
	sort.Slice(ch.ring, func(i, j int) bool {
		return ch.ring[i] < ch.ring[j]
	})
}

func (ch *ConsistentHash) RemoveNode(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	newRing := make([]uint32, 0)
	newRingMap := make(map[uint32]string)
	for _, hash := range ch.ring {
		if ch.ringMap[hash] != node {
			newRing = append(newRing, hash)
			newRingMap[hash] = ch.ringMap[hash]
		}
	}
	ch.ring = newRing
	ch.ringMap = newRingMap
}

func (ch *ConsistentHash) GetNode(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return ""
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	idx := sort.Search(len(ch.ring), func(i int) bool {
		return ch.ring[i] >= hash
	})

	if idx >= len(ch.ring) {
		idx = 0
	}

	return ch.ringMap[ch.ring[idx]]
}

func (ch *ConsistentHash) GetNodes(key string, count int) []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return nil
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	idx := sort.Search(len(ch.ring), func(i int) bool {
		return ch.ring[i] >= hash
	})

	seen := make(map[string]bool)
	result := make([]string, 0, count)

	for i := 0; i < len(ch.ring) && len(result) < count; i++ {
		pos := (idx + i) % len(ch.ring)
		node := ch.ringMap[ch.ring[pos]]
		if !seen[node] {
			seen[node] = true
			result = append(result, node)
		}
	}

	return result
}

func (ch *ConsistentHash) NodeCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	seen := make(map[string]bool)
	for _, node := range ch.ringMap {
		seen[node] = true
	}
	return len(seen)
}

func (ch *ConsistentHash) Clear() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.ring = ch.ring[:0]
	ch.ringMap = make(map[uint32]string)
}

type HashRing struct {
	nodes     map[string]float64
	ring      []ringEntry
	vnodes    int
	mu        sync.RWMutex
}

type ringEntry struct {
	hash  uint32
	node  string
	weight float64
}

func NewHashRing(vnodes int) *HashRing {
	if vnodes <= 0 {
		vnodes = 150
	}
	return &HashRing{
		nodes:  make(map[string]float64),
		ring:   make([]ringEntry, 0),
		vnodes: vnodes,
	}
}

func (hr *HashRing) AddNode(node string, weight float64) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.nodes[node] = weight

	vnodeCount := int(float64(hr.vnodes) * weight)
	for i := 0; i < vnodeCount; i++ {
		h := fnv.New32a()
		key := fmt.Sprintf("%s#%d", node, i)
		h.Write([]byte(key))
		hr.ring = append(hr.ring, ringEntry{
			hash:   h.Sum32(),
			node:   node,
			weight: weight,
		})
	}

	sort.Slice(hr.ring, func(i, j int) bool {
		return hr.ring[i].hash < hr.ring[j].hash
	})
}

func (hr *HashRing) RemoveNode(node string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	delete(hr.nodes, node)
	newRing := make([]ringEntry, 0)
	for _, entry := range hr.ring {
		if entry.node != node {
			newRing = append(newRing, entry)
		}
	}
	hr.ring = newRing
}

func (hr *HashRing) GetNode(key string) string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return ""
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	idx := sort.Search(len(hr.ring), func(i int) bool {
		return hr.ring[i].hash >= hash
	})

	if idx >= len(hr.ring) {
		idx = 0
	}

	return hr.ring[idx].node
}

func (hr *HashRing) NodeCount() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.nodes)
}

func (hr *HashRing) GetWeight(node string) float64 {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return hr.nodes[node]
}

type BloomFilter struct {
	bits    []bool
	size    uint
	hashes  int
	count   uint64
	mu      sync.RWMutex
}

func NewBloomFilter(size uint, hashes int) *BloomFilter {
	if size == 0 {
		size = 1024
	}
	if hashes <= 0 {
		hashes = 7
	}
	return &BloomFilter{
		bits:   make([]bool, size),
		size:   size,
		hashes: hashes,
	}
}

func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := 0; i < bf.hashes; i++ {
		idx := bf.hash(item, i)
		bf.bits[idx] = true
	}
	bf.count++
}

func (bf *BloomFilter) Contains(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for i := 0; i < bf.hashes; i++ {
		idx := bf.hash(item, i)
		if !bf.bits[idx] {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) hash(item string, seed int) uint {
	h := fnv.New32a()
	h.Write([]byte(item))
	h.Write([]byte{byte(seed)})
	return uint(h.Sum32()) % bf.size
}

func (bf *BloomFilter) Count() uint64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.count
}

func (bf *BloomFilter) FalsePositiveRate() float64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	setBits := 0
	for _, b := range bf.bits {
		if b {
			setBits++
		}
	}
	return math.Pow(float64(setBits)/float64(bf.size), float64(bf.hashes))
}

func (bf *BloomFilter) Reset() {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.bits = make([]bool, bf.size)
	bf.count = 0
}

type CountingBloomFilter struct {
	counts []uint8
	size   uint
	hashes int
	count  uint64
	mu     sync.RWMutex
}

func NewCountingBloomFilter(size uint, hashes int) *CountingBloomFilter {
	if size == 0 {
		size = 1024
	}
	if hashes <= 0 {
		hashes = 7
	}
	return &CountingBloomFilter{
		counts: make([]uint8, size),
		size:   size,
		hashes: hashes,
	}
}

func (cbf *CountingBloomFilter) Add(item string) {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	for i := 0; i < cbf.hashes; i++ {
		idx := cbf.hash(item, i)
		if cbf.counts[idx] < 255 {
			cbf.counts[idx]++
		}
	}
	cbf.count++
}

func (cbf *CountingBloomFilter) Remove(item string) bool {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	if !cbf.containsLocked(item) {
		return false
	}

	for i := 0; i < cbf.hashes; i++ {
		idx := cbf.hash(item, i)
		if cbf.counts[idx] > 0 {
			cbf.counts[idx]--
		}
	}
	cbf.count--
	return true
}

func (cbf *CountingBloomFilter) Contains(item string) bool {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()
	return cbf.containsLocked(item)
}

func (cbf *CountingBloomFilter) containsLocked(item string) bool {
	for i := 0; i < cbf.hashes; i++ {
		idx := cbf.hash(item, i)
		if cbf.counts[idx] == 0 {
			return false
		}
	}
	return true
}

func (cbf *CountingBloomFilter) hash(item string, seed int) uint {
	h := fnv.New32a()
	h.Write([]byte(item))
	h.Write([]byte{byte(seed)})
	return uint(h.Sum32()) % cbf.size
}

func (cbf *CountingBloomFilter) Count() uint64 {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()
	return cbf.count
}

func (cbf *CountingBloomFilter) Reset() {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()
	cbf.counts = make([]uint8, cbf.size)
	cbf.count = 0
}

type ScalableBloomFilter struct {
	filters   []*BloomFilter
	size      uint
	hashes    int
	threshold float64
	mu        sync.RWMutex
}

func NewScalableBloomFilter(initialSize uint, hashes int, threshold float64) *ScalableBloomFilter {
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.9
	}
	return &ScalableBloomFilter{
		filters:   []*BloomFilter{NewBloomFilter(initialSize, hashes)},
		size:      initialSize,
		hashes:    hashes,
		threshold: threshold,
	}
}

func (sbf *ScalableBloomFilter) Add(item string) {
	sbf.mu.Lock()
	defer sbf.mu.Unlock()

	last := sbf.filters[len(sbf.filters)-1]
	if last.FalsePositiveRate() > sbf.threshold {
		newSize := sbf.size * uint(len(sbf.filters)+1)
		sbf.filters = append(sbf.filters, NewBloomFilter(newSize, sbf.hashes))
	}

	sbf.filters[len(sbf.filters)-1].Add(item)
}

func (sbf *ScalableBloomFilter) Contains(item string) bool {
	sbf.mu.RLock()
	defer sbf.mu.RUnlock()

	for _, filter := range sbf.filters {
		if filter.Contains(item) {
			return true
		}
	}
	return false
}

func (sbf *ScalableBloomFilter) FilterCount() int {
	sbf.mu.RLock()
	defer sbf.mu.RUnlock()
	return len(sbf.filters)
}

func (sbf *ScalableBloomFilter) Count() uint64 {
	sbf.mu.RLock()
	defer sbf.mu.RUnlock()
	var total uint64
	for _, f := range sbf.filters {
		total += f.Count()
	}
	return total
}

type HyperLogLog struct {
	registers []uint8
	m         uint
	alpha     float64
	count     uint64
	mu        sync.RWMutex
}

func NewHyperLogLog(precision uint) *HyperLogLog {
	m := uint(1 << precision)
	alpha := 0.7213 / (1.0 + 1.079/float64(m))
	return &HyperLogLog{
		registers: make([]uint8, m),
		m:         m,
		alpha:     alpha,
	}
}

func (hll *HyperLogLog) Add(item string) {
	hll.mu.Lock()
	defer hll.mu.Unlock()

	h := fnv.New32a()
	h.Write([]byte(item))
	hash := h.Sum32()

	idx := hash >> (32 - hll.leadingBits())
	remaining := hash << hll.leadingBits()

	leadingZeros := uint8(0)
	for i := uint32(0x80000000); i > 0; i >>= 1 {
		if remaining&i == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	leadingZeros++
	if leadingZeros > hll.registers[idx] {
		hll.registers[idx] = leadingZeros
	}
	hll.count++
}

func (hll *HyperLogLog) Estimate() float64 {
	hll.mu.RLock()
	defer hll.mu.RUnlock()

	var sum float64
	zeroCount := 0

	for _, val := range hll.registers {
		if val == 0 {
			zeroCount++
		}
		sum += math.Pow(2, -float64(val))
	}

	rawEstimate := hll.alpha * float64(hll.m) * float64(hll.m) / sum

	if rawEstimate <= 5.0*float64(hll.m)/2.0 && zeroCount > 0 {
		return float64(hll.m) * math.Log(float64(hll.m)/float64(zeroCount))
	}

	if rawEstimate > (1.0/30.0)*math.Pow(2.0, 32.0) {
		return -math.Pow(2.0, 32.0) * math.Log(1.0-rawEstimate/math.Pow(2.0, 32.0))
	}

	return rawEstimate
}

func (hll *HyperLogLog) leadingBits() uint {
	return uint(math.Log2(float64(hll.m)))
}

func (hll *HyperLogLog) Count() uint64 {
	hll.mu.RLock()
	defer hll.mu.RUnlock()
	return hll.count
}

func (hll *HyperLogLog) Reset() {
	hll.mu.Lock()
	defer hll.mu.Unlock()
	hll.registers = make([]uint8, hll.m)
	hll.count = 0
}

type CountMinSketch struct {
	width   uint
	depth   uint
	table   [][]uint64
	counts  []uint64
	mu      sync.RWMutex
}

func NewCountMinSketch(width, depth uint) *CountMinSketch {
	if width == 0 {
		width = 1024
	}
	if depth == 0 {
		depth = 7
	}

	table := make([][]uint64, depth)
	for i := range table {
		table[i] = make([]uint64, width)
	}

	return &CountMinSketch{
		width:  width,
		depth:  depth,
		table:  table,
		counts: make([]uint64, depth),
	}
}

func (cms *CountMinSketch) Add(item string, count uint64) {
	cms.mu.Lock()
	defer cms.mu.Unlock()

	for i := uint(0); i < cms.depth; i++ {
		idx := cms.hash(item, i)
		cms.table[i][idx] += count
		cms.counts[i] += count
	}
}

func (cms *CountMinSketch) Estimate(item string) uint64 {
	cms.mu.RLock()
	defer cms.mu.RUnlock()

	minCount := uint64(math.MaxUint64)
	for i := uint(0); i < cms.depth; i++ {
		idx := cms.hash(item, i)
		if cms.table[i][idx] < minCount {
			minCount = cms.table[i][idx]
		}
	}
	return minCount
}

func (cms *CountMinSketch) hash(item string, seed uint) uint {
	h := fnv.New32a()
	h.Write([]byte(item))
	h.Write([]byte{byte(seed)})
	return uint(h.Sum32()) % cms.width
}

func (cms *CountMinSketch) Reset() {
	cms.mu.Lock()
	defer cms.mu.Unlock()
	for i := range cms.table {
		for j := range cms.table[i] {
			cms.table[i][j] = 0
		}
		cms.counts[i] = 0
	}
}

type CuckooFilter struct {
	buckets    [][]entry
	numBuckets uint
	fpSize     int
	count      uint64
	mu         sync.RWMutex
}

type entry struct {
	hash    uint32
	finger  uint8
	occupied bool
}

func NewCuckooFilter(capacity uint) *CuckooFilter {
	if capacity == 0 {
		capacity = 1024
	}
	numBuckets := capacity / 4
	return &CuckooFilter{
		buckets:    make([][]entry, numBuckets),
		numBuckets: numBuckets,
		fpSize:     3,
	}
}

func (cf *CuckooFilter) Add(item string) bool {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	h := cf.hashItem(item)
	fp := cf.fingerprint(h)
	b1 := cf.bucket(h)

	if cf.insert(b1, fp) {
		cf.count++
		return true
	}

	b2 := cf.altBucket(b1, fp)
	if cf.insert(b2, fp) {
		cf.count++
		return true
	}

	for i := 0; i < 500; i++ {
		bucket := b1
		if i%2 == 1 {
			bucket = b2
		}
		j := i % 4
		if cf.buckets[bucket] != nil && j < len(cf.buckets[bucket]) {
			cf.buckets[bucket][j].finger, fp = fp, cf.buckets[bucket][j].finger
		}
	}

	cf.count++
	return true
}

func (cf *CuckooFilter) Contains(item string) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	h := cf.hashItem(item)
	fp := cf.fingerprint(h)
	b1 := cf.bucket(h)
	b2 := cf.altBucket(b1, fp)

	return cf.containsBucket(b1, fp) || cf.containsBucket(b2, fp)
}

func (cf *CuckooFilter) insert(bucket uint, fp uint8) bool {
	if cf.buckets[bucket] == nil {
		cf.buckets[bucket] = make([]entry, 4)
	}
	for i := 0; i < 4; i++ {
		if !cf.buckets[bucket][i].occupied {
			cf.buckets[bucket][i] = entry{hash: 0, finger: fp, occupied: true}
			return true
		}
	}
	return false
}

func (cf *CuckooFilter) containsBucket(bucket uint, fp uint8) bool {
	if cf.buckets[bucket] == nil {
		return false
	}
	for _, e := range cf.buckets[bucket] {
		if e.occupied && e.finger == fp {
			return true
		}
	}
	return false
}

func (cf *CuckooFilter) hashItem(item string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(item))
	return h.Sum32()
}

func (cf *CuckooFilter) fingerprint(h uint32) uint8 {
	return uint8((h >> 24) & 0xFF)
}

func (cf *CuckooFilter) bucket(h uint32) uint {
	return uint(h) % cf.numBuckets
}

func (cf *CuckooFilter) altBucket(b uint, fp uint8) uint {
	return (b ^ uint(fp)) % cf.numBuckets
}

func (cf *CuckooFilter) Count() uint64 {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	return cf.count
}

func (cf *CuckooFilter) Reset() {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.buckets = make([][]entry, cf.numBuckets)
	cf.count = 0
}

type HashFunction func(data []byte) uint32

func FNV32(data []byte) uint32 {
	h := fnv.New32a()
	h.Write(data)
	return h.Sum32()
}

func FNV32a(data []byte) uint32 {
	h := fnv.New32a()
	h.Write(data)
	return h.Sum32()
}

type HashRingWithHealth struct {
	*HashRing
	healthy map[string]bool
	mu      sync.RWMutex
}

func NewHashRingWithHealth(vnodes int) *HashRingWithHealth {
	return &HashRingWithHealth{
		HashRing: NewHashRing(vnodes),
		healthy:  make(map[string]bool),
	}
}

func (hrh *HashRingWithHealth) AddHealthyNode(node string, weight float64) {
	hrh.mu.Lock()
	defer hrh.mu.Unlock()
	hrh.healthy[node] = true
	hrh.HashRing.AddNode(node, weight)
}

func (hrh *HashRingWithHealth) SetHealthy(node string, healthy bool) {
	hrh.mu.Lock()
	defer hrh.mu.Unlock()
	hrh.healthy[node] = healthy
}

func (hrh *HashRingWithHealth) GetHealthyNode(key string) string {
	hrh.mu.RLock()
	defer hrh.mu.RUnlock()

	for i := 0; i < 10; i++ {
		node := hrh.HashRing.GetNode(fmt.Sprintf("%s#%d", key, i))
		if node == "" {
			break
		}
		if hrh.healthy[node] {
			return node
		}
	}
	return ""
}
