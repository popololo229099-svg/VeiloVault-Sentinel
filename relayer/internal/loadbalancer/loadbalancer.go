package loadbalancer

import (
	"hash/fnv"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Backend struct {
	Name            string
	Address         string
	Port            int
	Weight          int
	Health          bool
	ActiveConns     int64
	TotalRequests   int64
	FailureCount    int64
	LastCheck       time.Time
	ResponseTime    time.Duration
	Metadata        map[string]string
	mu              sync.RWMutex
}

func NewBackend(name, address string, port, weight int) *Backend {
	return &Backend{
		Name:     name,
		Address:  address,
		Port:     port,
		Weight:   weight,
		Health:   true,
		Metadata: make(map[string]string),
	}
}

func (b *Backend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Health
}

func (b *Backend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Health = healthy
}

func (b *Backend) IncrementConns() {
	atomic.AddInt64(&b.ActiveConns, 1)
}

func (b *Backend) DecrementConns() {
	atomic.AddInt64(&b.ActiveConns, -1)
}

func (b *Backend) ActiveConnections() int64 {
	return atomic.LoadInt64(&b.ActiveConns)
}

func (b *Backend) IncrementRequests() {
	atomic.AddInt64(&b.TotalRequests, 1)
}

func (b *Backend) SetResponseTime(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ResponseTime = d
}

func (b *Backend) Endpoint() string {
	return b.Address
}

type LoadBalancer interface {
	Next() *Backend
	Add(backend *Backend)
	Remove(name string)
	Get(name string) *Backend
	List() []*Backend
	Healthy() []*Backend
	Count() int
	HealthyCount() int
}

type RoundRobinBalancer struct {
	backends []*Backend
	current  uint64
	mu       sync.RWMutex
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		backends: make([]*Backend, 0),
	}
}

func (rrb *RoundRobinBalancer) Next() *Backend {
	rrb.mu.RLock()
	defer rrb.mu.RUnlock()

	healthy := rrb.healthy()
	if len(healthy) == 0 {
		return nil
	}

	idx := atomic.AddUint64(&rrb.current, 1)
	return healthy[idx%uint64(len(healthy))]
}

func (rrb *RoundRobinBalancer) Add(backend *Backend) {
	rrb.mu.Lock()
	defer rrb.mu.Unlock()
	rrb.backends = append(rrb.backends, backend)
}

func (rrb *RoundRobinBalancer) Remove(name string) {
	rrb.mu.Lock()
	defer rrb.mu.Unlock()
	for i, b := range rrb.backends {
		if b.Name == name {
			rrb.backends = append(rrb.backends[:i], rrb.backends[i+1:]...)
			return
		}
	}
}

func (rrb *RoundRobinBalancer) Get(name string) *Backend {
	rrb.mu.RLock()
	defer rrb.mu.RUnlock()
	for _, b := range rrb.backends {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func (rrb *RoundRobinBalancer) List() []*Backend {
	rrb.mu.RLock()
	defer rrb.mu.RUnlock()
	result := make([]*Backend, len(rrb.backends))
	copy(result, rrb.backends)
	return result
}

func (rrb *RoundRobinBalancer) Healthy() []*Backend {
	rrb.mu.RLock()
	defer rrb.mu.RUnlock()
	return rrb.healthy()
}

func (rrb *RoundRobinBalancer) healthy() []*Backend {
	result := make([]*Backend, 0)
	for _, b := range rrb.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

func (rrb *RoundRobinBalancer) Count() int {
	rrb.mu.RLock()
	defer rrb.mu.RUnlock()
	return len(rrb.backends)
}

func (rrb *RoundRobinBalancer) HealthyCount() int {
	return len(rrb.Healthy())
}

type WeightedRoundRobinBalancer struct {
	backends   []*Backend
	current    int
	weights    []int
	effectiveW []int
	mu         sync.RWMutex
}

func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		backends: make([]*Backend, 0),
		weights:  make([]int, 0),
	}
}

func (wrrb *WeightedRoundRobinBalancer) Next() *Backend {
	wrrb.mu.Lock()
	defer wrrb.mu.Unlock()

	healthy := wrrb.healthy()
	if len(healthy) == 0 {
		return nil
	}

	for {
		wrrb.current = (wrrb.current + 1) % len(wrrb.backends)
		if wrrb.backends[wrrb.current].IsHealthy() {
			return wrrb.backends[wrrb.current]
		}
	}
}

func (wrrb *WeightedRoundRobinBalancer) Add(backend *Backend) {
	wrrb.mu.Lock()
	defer wrrb.mu.Unlock()
	wrrb.backends = append(wrrb.backends, backend)
	wrrb.weights = append(wrrb.weights, backend.Weight)
}

func (wrrb *WeightedRoundRobinBalancer) Remove(name string) {
	wrrb.mu.Lock()
	defer wrrb.mu.Unlock()
	for i, b := range wrrb.backends {
		if b.Name == name {
			wrrb.backends = append(wrrb.backends[:i], wrrb.backends[i+1:]...)
			if i < len(wrrb.weights) {
				wrrb.weights = append(wrrb.weights[:i], wrrb.weights[i+1:]...)
			}
			return
		}
	}
}

func (wrrb *WeightedRoundRobinBalancer) Get(name string) *Backend {
	wrrb.mu.RLock()
	defer wrrb.mu.RUnlock()
	for _, b := range wrrb.backends {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func (wrrb *WeightedRoundRobinBalancer) List() []*Backend {
	wrrb.mu.RLock()
	defer wrrb.mu.RUnlock()
	result := make([]*Backend, len(wrrb.backends))
	copy(result, wrrb.backends)
	return result
}

func (wrrb *WeightedRoundRobinBalancer) Healthy() []*Backend {
	wrrb.mu.RLock()
	defer wrrb.mu.RUnlock()
	return wrrb.healthy()
}

func (wrrb *WeightedRoundRobinBalancer) healthy() []*Backend {
	result := make([]*Backend, 0)
	for _, b := range wrrb.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

func (wrrb *WeightedRoundRobinBalancer) Count() int {
	wrrb.mu.RLock()
	defer wrrb.mu.RUnlock()
	return len(wrrb.backends)
}

func (wrrb *WeightedRoundRobinBalancer) HealthyCount() int {
	return len(wrrb.Healthy())
}

type LeastConnectionsBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		backends: make([]*Backend, 0),
	}
}

func (lcb *LeastConnectionsBalancer) Next() *Backend {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()

	healthy := lcb.healthy()
	if len(healthy) == 0 {
		return nil
	}

	var best *Backend
	for _, b := range healthy {
		if best == nil || b.ActiveConnections() < best.ActiveConnections() {
			best = b
		}
	}
	return best
}

func (lcb *LeastConnectionsBalancer) Add(backend *Backend) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()
	lcb.backends = append(lcb.backends, backend)
}

func (lcb *LeastConnectionsBalancer) Remove(name string) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()
	for i, b := range lcb.backends {
		if b.Name == name {
			lcb.backends = append(lcb.backends[:i], lcb.backends[i+1:]...)
			return
		}
	}
}

func (lcb *LeastConnectionsBalancer) Get(name string) *Backend {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()
	for _, b := range lcb.backends {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func (lcb *LeastConnectionsBalancer) List() []*Backend {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()
	result := make([]*Backend, len(lcb.backends))
	copy(result, lcb.backends)
	return result
}

func (lcb *LeastConnectionsBalancer) Healthy() []*Backend {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()
	return lcb.healthy()
}

func (lcb *LeastConnectionsBalancer) healthy() []*Backend {
	result := make([]*Backend, 0)
	for _, b := range lcb.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

func (lcb *LeastConnectionsBalancer) Count() int {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()
	return len(lcb.backends)
}

func (lcb *LeastConnectionsBalancer) HealthyCount() int {
	return len(lcb.Healthy())
}

type RandomBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		backends: make([]*Backend, 0),
	}
}

func (rb *RandomBalancer) Next() *Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	healthy := rb.healthy()
	if len(healthy) == 0 {
		return nil
	}

	return healthy[rand.Intn(len(healthy))]
}

func (rb *RandomBalancer) Add(backend *Backend) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.backends = append(rb.backends, backend)
}

func (rb *RandomBalancer) Remove(name string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for i, b := range rb.backends {
		if b.Name == name {
			rb.backends = append(rb.backends[:i], rb.backends[i+1:]...)
			return
		}
	}
}

func (rb *RandomBalancer) Get(name string) *Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	for _, b := range rb.backends {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func (rb *RandomBalancer) List() []*Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	result := make([]*Backend, len(rb.backends))
	copy(result, rb.backends)
	return result
}

func (rb *RandomBalancer) Healthy() []*Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.healthy()
}

func (rb *RandomBalancer) healthy() []*Backend {
	result := make([]*Backend, 0)
	for _, b := range rb.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

func (rb *RandomBalancer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.backends)
}

func (rb *RandomBalancer) HealthyCount() int {
	return len(rb.Healthy())
}

type IPHashBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewIPHashBalancer() *IPHashBalancer {
	return &IPHashBalancer{
		backends: make([]*Backend, 0),
	}
}

func (ihb *IPHashBalancer) Next(ip string) *Backend {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()

	healthy := ihb.healthy()
	if len(healthy) == 0 {
		return nil
	}

	h := fnv.New32a()
	h.Write([]byte(ip))
	idx := h.Sum32() % uint32(len(healthy))
	return healthy[idx]
}

func (ihb *IPHashBalancer) Add(backend *Backend) {
	ihb.mu.Lock()
	defer ihb.mu.Unlock()
	ihb.backends = append(ihb.backends, backend)
}

func (ihb *IPHashBalancer) Remove(name string) {
	ihb.mu.Lock()
	defer ihb.mu.Unlock()
	for i, b := range ihb.backends {
		if b.Name == name {
			ihb.backends = append(ihb.backends[:i], ihb.backends[i+1:]...)
			return
		}
	}
}

func (ihb *IPHashBalancer) Get(name string) *Backend {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()
	for _, b := range ihb.backends {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func (ihb *IPHashBalancer) List() []*Backend {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()
	result := make([]*Backend, len(ihb.backends))
	copy(result, ihb.backends)
	return result
}

func (ihb *IPHashBalancer) Healthy() []*Backend {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()
	return ihb.healthy()
}

func (ihb *IPHashBalancer) healthy() []*Backend {
	result := make([]*Backend, 0)
	for _, b := range ihb.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

func (ihb *IPHashBalancer) Count() int {
	ihb.mu.RLock()
	defer ihb.mu.RUnlock()
	return len(ihb.backends)
}

func (ihb *IPHashBalancer) HealthyCount() int {
	return len(ihb.Healthy())
}

type ConsistentHashBalancer struct {
	backends      []*Backend
	ring          []uint32
	ringMap       map[uint32]*Backend
	virtualCount  int
	mu            sync.RWMutex
}

func NewConsistentHashBalancer(virtualNodes int) *ConsistentHashBalancer {
	if virtualNodes <= 0 {
		virtualNodes = 150
	}
	return &ConsistentHashBalancer{
		backends:     make([]*Backend, 0),
		ring:         make([]uint32, 0),
		ringMap:      make(map[uint32]*Backend),
		virtualCount: virtualNodes,
	}
}

func (chb *ConsistentHashBalancer) Next(key string) *Backend {
	chb.mu.RLock()
	defer chb.mu.RUnlock()

	if len(chb.ring) == 0 {
		return nil
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	idx := sort.Search(len(chb.ring), func(i int) bool {
		return chb.ring[i] >= hash
	})

	if idx >= len(chb.ring) {
		idx = 0
	}

	return chb.ringMap[chb.ring[idx]]
}

func (chb *ConsistentHashBalancer) Add(backend *Backend) {
	chb.mu.Lock()
	defer chb.mu.Unlock()

	chb.backends = append(chb.backends, backend)

	for i := 0; i < chb.virtualCount; i++ {
		h := fnv.New32a()
		h.Write([]byte(backend.Name + "-" + string(rune(i))))
		hash := h.Sum32()
		chb.ring = append(chb.ring, hash)
		chb.ringMap[hash] = backend
	}

	sort.Slice(chb.ring, func(i, j int) bool {
		return chb.ring[i] < chb.ring[j]
	})
}

func (chb *ConsistentHashBalancer) Remove(name string) {
	chb.mu.Lock()
	defer chb.mu.Unlock()

	var target *Backend
	for _, b := range chb.backends {
		if b.Name == name {
			target = b
			break
		}
	}

	if target == nil {
		return
	}

	newBackends := make([]*Backend, 0)
	for _, b := range chb.backends {
		if b.Name != name {
			newBackends = append(newBackends, b)
		}
	}
	chb.backends = newBackends

	newRing := make([]uint32, 0)
	newRingMap := make(map[uint32]*Backend)
	for _, hash := range chb.ring {
		if chb.ringMap[hash] != target {
			newRing = append(newRing, hash)
			newRingMap[hash] = chb.ringMap[hash]
		}
	}
	chb.ring = newRing
	chb.ringMap = newRingMap
}

func (chb *ConsistentHashBalancer) Get(name string) *Backend {
	chb.mu.RLock()
	defer chb.mu.RUnlock()
	for _, b := range chb.backends {
		if b.Name == name {
			return b
		}
	}
	return nil
}

func (chb *ConsistentHashBalancer) List() []*Backend {
	chb.mu.RLock()
	defer chb.mu.RUnlock()
	result := make([]*Backend, len(chb.backends))
	copy(result, chb.backends)
	return result
}

func (chb *ConsistentHashBalancer) Healthy() []*Backend {
	chb.mu.RLock()
	defer chb.mu.RUnlock()
	result := make([]*Backend, 0)
	for _, b := range chb.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

func (chb *ConsistentHashBalancer) Count() int {
	chb.mu.RLock()
	defer chb.mu.RUnlock()
	return len(chb.backends)
}

func (chb *ConsistentHashBalancer) HealthyCount() int {
	return len(chb.Healthy())
}

type SessionAffinityBalancer struct {
	inner     LoadBalancer
	sessions  map[string]*Backend
	ttl       time.Duration
	mu        sync.RWMutex
}

func NewSessionAffinityBalancer(inner LoadBalancer, ttl time.Duration) *SessionAffinityBalancer {
	return &SessionAffinityBalancer{
		inner:    inner,
		sessions: make(map[string]*Backend),
		ttl:      ttl,
	}
}

func (sab *SessionAffinityBalancer) Next(sessionKey string) *Backend {
	sab.mu.RLock()
	if backend, exists := sab.sessions[sessionKey]; exists {
		sab.mu.RUnlock()
		if backend.IsHealthy() {
			return backend
		}
	}
	sab.mu.RUnlock()

	backend := sab.inner.Next()
	if backend != nil {
		sab.mu.Lock()
		sab.sessions[sessionKey] = backend
		sab.mu.Unlock()
	}
	return backend
}

func (sab *SessionAffinityBalancer) Add(backend *Backend) {
	sab.inner.Add(backend)
}

func (sab *SessionAffinityBalancer) Remove(name string) {
	sab.inner.Remove(name)
	sab.mu.Lock()
	defer sab.mu.Unlock()
	for key, b := range sab.sessions {
		if b.Name == name {
			delete(sab.sessions, key)
		}
	}
}

func (sab *SessionAffinityBalancer) Get(name string) *Backend {
	return sab.inner.Get(name)
}

func (sab *SessionAffinityBalancer) List() []*Backend {
	return sab.inner.List()
}

func (sab *SessionAffinityBalancer) Healthy() []*Backend {
	return sab.inner.Healthy()
}

func (sab *SessionAffinityBalancer) Count() int {
	return sab.inner.Count()
}

func (sab *SessionAffinityBalancer) HealthyCount() int {
	return sab.inner.HealthyCount()
}

func (sab *SessionAffinityBalancer) InvalidateSession(sessionKey string) {
	sab.mu.Lock()
	defer sab.mu.Unlock()
	delete(sab.sessions, sessionKey)
}

func (sab *SessionAffinityBalancer) InvalidateAllSessions() {
	sab.mu.Lock()
	defer sab.mu.Unlock()
	sab.sessions = make(map[string]*Backend)
}

type FailoverBalancer struct {
	primary   LoadBalancer
	fallback  LoadBalancer
	mu        sync.RWMutex
}

func NewFailoverBalancer(primary, fallback LoadBalancer) *FailoverBalancer {
	return &FailoverBalancer{
		primary:  primary,
		fallback: fallback,
	}
}

func (fb *FailoverBalancer) Next() *Backend {
	backend := fb.primary.Next()
	if backend != nil {
		return backend
	}
	return fb.fallback.Next()
}

func (fb *FailoverBalancer) Add(backend *Backend) {
	fb.primary.Add(backend)
}

func (fb *FailoverBalancer) Remove(name string) {
	fb.primary.Remove(name)
}

func (fb *FailoverBalancer) Get(name string) *Backend {
	backend := fb.primary.Get(name)
	if backend != nil {
		return backend
	}
	return fb.fallback.Get(name)
}

func (fb *FailoverBalancer) List() []*Backend {
	primary := fb.primary.List()
	fallback := fb.fallback.List()
	result := make([]*Backend, 0, len(primary)+len(fallback))
	result = append(result, primary...)
	result = append(result, fallback...)
	return result
}

func (fb *FailoverBalancer) Healthy() []*Backend {
	primary := fb.primary.Healthy()
	if len(primary) > 0 {
		return primary
	}
	return fb.fallback.Healthy()
}

func (fb *FailoverBalancer) Count() int {
	return fb.primary.Count() + fb.fallback.Count()
}

func (fb *FailoverBalancer) HealthyCount() int {
	return fb.primary.HealthyCount() + fb.fallback.HealthyCount()
}

type HealthAwareBalancer struct {
	inner       LoadBalancer
	healthCheck func(*Backend) bool
	interval    time.Duration
	mu          sync.RWMutex
	stopCh      chan struct{}
}

func NewHealthAwareBalancer(inner LoadBalancer, healthCheck func(*Backend) bool, interval time.Duration) *HealthAwareBalancer {
	return &HealthAwareBalancer{
		inner:       inner,
		healthCheck: healthCheck,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

func (hab *HealthAwareBalancer) Next() *Backend {
	return hab.inner.Next()
}

func (hab *HealthAwareBalancer) Add(backend *Backend) {
	hab.inner.Add(backend)
}

func (hab *HealthAwareBalancer) Remove(name string) {
	hab.inner.Remove(name)
}

func (hab *HealthAwareBalancer) Get(name string) *Backend {
	return hab.inner.Get(name)
}

func (hab *HealthAwareBalancer) List() []*Backend {
	return hab.inner.List()
}

func (hab *HealthAwareBalancer) Healthy() []*Backend {
	return hab.inner.Healthy()
}

func (hab *HealthAwareBalancer) Count() int {
	return hab.inner.Count()
}

func (hab *HealthAwareBalancer) HealthyCount() int {
	return hab.inner.HealthyCount()
}

func (hab *HealthAwareBalancer) Start() {
	go func() {
		ticker := time.NewTicker(hab.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hab.checkHealth()
			case <-hab.stopCh:
				return
			}
		}
	}()
}

func (hab *HealthAwareBalancer) checkHealth() {
	for _, backend := range hab.inner.List() {
		healthy := hab.healthCheck(backend)
		backend.SetHealthy(healthy)
	}
}

func (hab *HealthAwareBalancer) Stop() {
	close(hab.stopCh)
}

type LoadBalancerConfig struct {
	Type          string
	VirtualNodes  int
	SessionTTL    time.Duration
	HealthCheck   func(*Backend) bool
	HealthInterval time.Duration
}

func NewLoadBalancer(config LoadBalancerConfig) LoadBalancer {
	switch config.Type {
	case "round-robin":
		return NewRoundRobinBalancer()
	case "weighted":
		return NewWeightedRoundRobinBalancer()
	case "least-connections":
		return NewLeastConnectionsBalancer()
	case "random":
		return NewRandomBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}
