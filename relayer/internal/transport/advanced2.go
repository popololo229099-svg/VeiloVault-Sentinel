package transport

import (
	"net"
	"sync"
	"time"
)

type ConnectionFilter func(Connection) bool

type FilteredListener struct {
	inner   Listener
	filters []ConnectionFilter
	mu      sync.RWMutex
}

func NewFilteredListener(inner Listener, filters ...ConnectionFilter) *FilteredListener {
	return &FilteredListener{
		inner:   inner,
		filters: filters,
	}
}

func (fl *FilteredListener) Accept() (Connection, error) {
	for {
		conn, err := fl.inner.Accept()
		if err != nil {
			return nil, err
		}

		fl.mu.RLock()
		accepted := true
		for _, filter := range fl.filters {
			if !filter(conn) {
				accepted = false
				break
			}
		}
		fl.mu.RUnlock()

		if accepted {
			return conn, nil
		}
		conn.Close()
	}
}

func (fl *FilteredListener) Close() error { return fl.inner.Close() }
func (fl *FilteredListener) Addr() net.Addr { return fl.inner.Addr() }

func (fl *FilteredListener) AddFilter(f ConnectionFilter) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.filters = append(fl.filters, f)
}

type WhitelistFilter struct {
	allowed map[string]bool
	mu      sync.RWMutex
}

func NewWhitelistFilter(addrs ...string) *WhitelistFilter {
	allowed := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		allowed[a] = true
	}
	return &WhitelistFilter{allowed: allowed}
}

func (wf *WhitelistFilter) Allow(addr string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()
	wf.allowed[addr] = true
}

func (wf *WhitelistFilter) Deny(addr string) {
	wf.mu.Lock()
	defer wf.mu.Unlock()
	delete(wf.allowed, addr)
}

func (wf *WhitelistFilter) Filter(conn Connection) bool {
	wf.mu.RLock()
	defer wf.mu.RUnlock()
	if conn.RemoteAddr() == nil {
		return false
	}
	return wf.allowed[conn.RemoteAddr().String()]
}

type BlacklistFilter struct {
	blocked map[string]bool
	mu      sync.RWMutex
}

func NewBlacklistFilter(addrs ...string) *BlacklistFilter {
	blocked := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		blocked[a] = true
	}
	return &BlacklistFilter{blocked: blocked}
}

func (bf *BlacklistFilter) Block(addr string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.blocked[addr] = true
}

func (bf *BlacklistFilter) Unblock(addr string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	delete(bf.blocked, addr)
}

func (bf *BlacklistFilter) Filter(conn Connection) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	if conn.RemoteAddr() == nil {
		return false
	}
	return !bf.blocked[conn.RemoteAddr().String()]
}

type RateLimitFilter struct {
	maxPerSec int
	counts    map[string]int
	lastReset map[string]time.Time
	mu        sync.Mutex
}

func NewRateLimitFilter(maxPerSec int) *RateLimitFilter {
	if maxPerSec <= 0 {
		maxPerSec = 100
	}
	return &RateLimitFilter{
		maxPerSec: maxPerSec,
		counts:    make(map[string]int),
		lastReset: make(map[string]time.Time),
	}
}

func (rlf *RateLimitFilter) Filter(conn Connection) bool {
	rlf.mu.Lock()
	defer rlf.mu.Unlock()

	if conn.RemoteAddr() == nil {
		return false
	}

	addr := conn.RemoteAddr().String()
	now := time.Now()

	if now.Sub(rlf.lastReset[addr]) >= time.Second {
		rlf.counts[addr] = 0
		rlf.lastReset[addr] = now
	}

	rlf.counts[addr]++
	return rlf.counts[addr] <= rlf.maxPerSec
}

type ConcurrencyFilter struct {
	maxConcurrent int
	current       map[string]int
	mu            sync.Mutex
}

func NewConcurrencyFilter(maxConcurrent int) *ConcurrencyFilter {
	if maxConcurrent <= 0 {
		maxConcurrent = 100
	}
	return &ConcurrencyFilter{
		maxConcurrent: maxConcurrent,
		current:       make(map[string]int),
	}
}

func (cf *ConcurrencyFilter) Filter(conn Connection) bool {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if conn.RemoteAddr() == nil {
		return false
	}

	addr := conn.RemoteAddr().String()
	if cf.current[addr] >= cf.maxConcurrent {
		return false
	}
	cf.current[addr]++
	return true
}

func (cf *ConcurrencyFilter) Release(conn Connection) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if conn.RemoteAddr() != nil {
		addr := conn.RemoteAddr().String()
		if cf.current[addr] > 0 {
			cf.current[addr]--
		}
	}
}

type TLSFilter struct {
	requireTLS bool
	mu         sync.RWMutex
}

func NewTLSFilter(requireTLS bool) *TLSFilter {
	return &TLSFilter{requireTLS: requireTLS}
}

func (tf *TLSFilter) Filter(conn Connection) bool {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	_ = tf.requireTLS
	return true
}

type TimingFilter struct {
	minDuration time.Duration
	maxDuration time.Duration
	mu          sync.RWMutex
}

func NewTimingFilter(min, max time.Duration) *TimingFilter {
	return &TimingFilter{minDuration: min, maxDuration: max}
}

func (tf *TimingFilter) Filter(conn Connection) bool {
	return true
}

type CompositeTransportFilter struct {
	filters []ConnectionFilter
	mode    string
	mu      sync.RWMutex
}

func NewCompositeTransportFilter(mode string, filters ...ConnectionFilter) *CompositeTransportFilter {
	return &CompositeTransportFilter{filters: filters, mode: mode}
}

func (ctf *CompositeTransportFilter) Filter(conn Connection) bool {
	ctf.mu.RLock()
	defer ctf.mu.RUnlock()

	switch ctf.mode {
	case "and":
		for _, f := range ctf.filters {
			if !f(conn) {
				return false
			}
		}
		return true
	case "or":
		for _, f := range ctf.filters {
			if f(conn) {
				return true
			}
		}
		return false
	case "not":
		for _, f := range ctf.filters {
			if f(conn) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type WrappedConnection struct {
	inner     Connection
	onRead    func([]byte) (int, error)
	onWrite   func([]byte) (int, error)
	onClose   func() error
	mu        sync.RWMutex
}

func NewWrappedConnection(conn Connection) *WrappedConnection {
	return &WrappedConnection{inner: conn}
}

func (wc *WrappedConnection) OnRead(fn func([]byte) (int, error)) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.onRead = fn
}

func (wc *WrappedConnection) OnWrite(fn func([]byte) (int, error)) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.onWrite = fn
}

func (wc *WrappedConnection) OnClose(fn func() error) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.onClose = fn
}

func (wc *WrappedConnection) Read(p []byte) (int, error) {
	wc.mu.RLock()
	onRead := wc.onRead
	wc.mu.RUnlock()

	if onRead != nil {
		return onRead(p)
	}
	return wc.inner.Read(p)
}

func (wc *WrappedConnection) Write(p []byte) (int, error) {
	wc.mu.RLock()
	onWrite := wc.onWrite
	wc.mu.RUnlock()

	if onWrite != nil {
		return onWrite(p)
	}
	return wc.inner.Write(p)
}

func (wc *WrappedConnection) Close() error {
	wc.mu.RLock()
	onClose := wc.onClose
	wc.mu.RUnlock()

	if onClose != nil {
		return onClose()
	}
	return wc.inner.Close()
}

func (wc *WrappedConnection) RemoteAddr() net.Addr { return wc.inner.RemoteAddr() }
func (wc *WrappedConnection) LocalAddr() net.Addr  { return wc.inner.LocalAddr() }

func (wc *WrappedConnection) SetDeadline(t time.Time) error {
	return wc.inner.SetDeadline(t)
}

func (wc *WrappedConnection) SetReadDeadline(t time.Time) error {
	return wc.inner.SetReadDeadline(t)
}

func (wc *WrappedConnection) SetWriteDeadline(t time.Time) error {
	return wc.inner.SetWriteDeadline(t)
}
