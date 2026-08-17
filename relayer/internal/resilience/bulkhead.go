package resilience

import (
	"errors"
	"sync"
	"time"
)

var ErrBulkheadFull = errors.New("bulkhead is full")

type Bulkhead struct {
	mu         sync.Mutex
	sem        chan struct{}
	maxConc    int
	maxQueue   int
	queue      chan struct{}
	timeout    time.Duration
	waiting    int
	active     int
	totalWait  time.Duration
}

type BulkheadConfig struct {
	MaxConcurrency int
	MaxQueue       int
	Timeout        time.Duration
}

func NewBulkhead(cfg BulkheadConfig) *Bulkhead {
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 10
	}
	if cfg.MaxQueue <= 0 {
		cfg.MaxQueue = 50
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Bulkhead{
		sem:      make(chan struct{}, cfg.MaxConcurrency),
		maxConc:  cfg.MaxConcurrency,
		maxQueue: cfg.MaxQueue,
		queue:    make(chan struct{}, cfg.MaxQueue),
		timeout:  cfg.Timeout,
	}
}

func (b *Bulkhead) Execute(fn func() error) error {
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
		return fn()
	default:
	}

	select {
	case b.queue <- struct{}{}:
		b.mu.Lock()
		b.waiting++
		b.mu.Unlock()
		defer func() {
			<-b.queue
			b.mu.Lock()
			b.waiting--
			b.mu.Unlock()
		}()
	case <-time.After(b.timeout):
		return ErrBulkheadFull
	}

	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
		return fn()
	case <-time.After(b.timeout):
		return ErrBulkheadFull
	}
}

func (b *Bulkhead) Stats() (active, waiting, capacity int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.sem), b.waiting, b.maxConc
}

// NamedBulkhead manages multiple bulkheads by key.
type NamedBulkhead struct {
	bulkheads map[string]*Bulkhead
	cfg       BulkheadConfig
	mu        sync.RWMutex
}

func NewNamedBulkhead(cfg BulkheadConfig) *NamedBulkhead {
	return &NamedBulkhead{
		bulkheads: make(map[string]*Bulkhead),
		cfg:       cfg,
	}
}

func (nb *NamedBulkhead) Get(name string) *Bulkhead {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	if b, ok := nb.bulkheads[name]; ok {
		return b
	}
	b := NewBulkhead(nb.cfg)
	nb.bulkheads[name] = b
	return b
}

func (nb *NamedBulkhead) Execute(name string, fn func() error) error {
	return nb.Get(name).Execute(fn)
}
