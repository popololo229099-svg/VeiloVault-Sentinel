package metrics

import (
	"sync"
	"time"
)

type Counter struct {
	name  string
	value float64
	tags  map[string]string
	mu    sync.RWMutex
}

func NewCounter(name string, tags map[string]string) *Counter {
	if tags == nil {
		tags = make(map[string]string)
	}
	return &Counter{name: name, tags: tags}
}

func (c *Counter) Inc()             { c.Add(1) }
func (c *Counter) Dec()             { c.Add(-1) }
func (c *Counter) Add(value float64) { c.mu.Lock(); c.value += value; c.mu.Unlock() }
func (c *Counter) Get() float64     { c.mu.RLock(); defer c.mu.RUnlock(); return c.value }
func (c *Counter) Reset()           { c.mu.Lock(); c.value = 0; c.mu.Unlock() }
func (c *Counter) Name() string     { return c.name }

type Gauge struct {
	name  string
	value float64
	tags  map[string]string
	mu    sync.RWMutex
}

func NewGauge(name string, tags map[string]string) *Gauge {
	if tags == nil {
		tags = make(map[string]string)
	}
	return &Gauge{name: name, tags: tags}
}

func (g *Gauge) Set(value float64) { g.mu.Lock(); g.value = value; g.mu.Unlock() }
func (g *Gauge) Inc()              { g.Add(1) }
func (g *Gauge) Dec()              { g.Add(-1) }
func (g *Gauge) Add(value float64) { g.mu.Lock(); g.value += value; g.mu.Unlock() }
func (g *Gauge) Get() float64      { g.mu.RLock(); defer g.mu.RUnlock(); return g.value }
func (g *Gauge) Name() string      { return g.name }

type Histogram struct {
	name   string
	bounds []float64
	counts []int64
	sum    float64
	total  int64
	mu     sync.RWMutex
}

func NewHistogram(name string, bounds []float64) *Histogram {
	return &Histogram{
		name:   name,
		bounds: bounds,
		counts: make([]int64, len(bounds)+1),
	}
}

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += value
	h.total++
	for i, b := range h.bounds {
		if value <= b {
			h.counts[i]++
			return
		}
	}
	h.counts[len(h.bounds)]++
}

func (h *Histogram) Mean() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.total == 0 {
		return 0
	}
	return h.sum / float64(h.total)
}

func (h *Histogram) Count() int64 { h.mu.RLock(); defer h.mu.RUnlock(); return h.total }

type Timer struct {
	name    string
	hist    *Histogram
	started time.Time
}

func NewTimer(name string) *Timer {
	return &Timer{
		name: name,
		hist: NewHistogram(name+"_duration_ms", []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000}),
	}
}

func (t *Timer) Start() { t.started = time.Now() }

func (t *Timer) Stop() time.Duration {
	d := time.Since(t.started)
	t.hist.Observe(float64(d.Microseconds()) / 1000.0)
	return d
}

func (t *Timer) Measure(fn func()) time.Duration {
	t.Start()
	fn()
	return t.Stop()
}

type MetricsCollector struct {
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
	timers     map[string]*Timer
	mu         sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
		timers:     make(map[string]*Timer),
	}
}

func (mc *MetricsCollector) Counter(name string, tags map[string]string) *Counter {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if c, ok := mc.counters[name]; ok {
		return c
	}
	c := NewCounter(name, tags)
	mc.counters[name] = c
	return c
}

func (mc *MetricsCollector) Gauge(name string, tags map[string]string) *Gauge {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if g, ok := mc.gauges[name]; ok {
		return g
	}
	g := NewGauge(name, tags)
	mc.gauges[name] = g
	return g
}

func (mc *MetricsCollector) Histogram(name string, bounds []float64) *Histogram {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if h, ok := mc.histograms[name]; ok {
		return h
	}
	h := NewHistogram(name, bounds)
	mc.histograms[name] = h
	return h
}

func (mc *MetricsCollector) Timer(name string) *Timer {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if t, ok := mc.timers[name]; ok {
		return t
	}
	t := NewTimer(name)
	mc.timers[name] = t
	return t
}

func (mc *MetricsCollector) Snapshot() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	snap := make(map[string]interface{})
	counters := make(map[string]float64)
	for k, v := range mc.counters {
		counters[k] = v.Get()
	}
	gauges := make(map[string]float64)
	for k, v := range mc.gauges {
		gauges[k] = v.Get()
	}
	histograms := make(map[string]map[string]interface{})
	for k, v := range mc.histograms {
		histograms[k] = map[string]interface{}{
			"count": v.Count(),
			"mean":  v.Mean(),
		}
	}
	snap["counters"] = counters
	snap["gauges"] = gauges
	snap["histograms"] = histograms
	return snap
}
