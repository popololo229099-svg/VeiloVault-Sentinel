package telemetry

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeGauge
	MetricTypeHistogram
	MetricTypeSummary
)

type Metric interface {
	Name() string
	Type() MetricType
	Value() float64
	Tags() map[string]string
	Timestamp() time.Time
}

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

func (c *Counter) Inc(delta ...float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := 1.0
	if len(delta) > 0 {
		d = delta[0]
	}
	c.value += d
}

func (c *Counter) Dec(delta ...float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := 1.0
	if len(delta) > 0 {
		d = delta[0]
	}
	c.value -= d
}

func (c *Counter) Name() string            { return c.name }
func (c *Counter) Type() MetricType        { return MetricTypeCounter }
func (c *Counter) Value() float64          { c.mu.RLock(); defer c.mu.RUnlock(); return c.value }
func (c *Counter) Tags() map[string]string { return c.tags }
func (c *Counter) Timestamp() time.Time    { return time.Now() }

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

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

func (g *Gauge) Inc(delta ...float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	d := 1.0
	if len(delta) > 0 {
		d = delta[0]
	}
	g.value += d
}

func (g *Gauge) Dec(delta ...float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	d := 1.0
	if len(delta) > 0 {
		d = delta[0]
	}
	g.value -= d
}

func (g *Gauge) Name() string            { return g.name }
func (g *Gauge) Type() MetricType        { return MetricTypeGauge }
func (g *Gauge) Value() float64          { g.mu.RLock(); defer g.mu.RUnlock(); return g.value }
func (g *Gauge) Tags() map[string]string { return g.tags }
func (g *Gauge) Timestamp() time.Time    { return time.Now() }

type Histogram struct {
	name   string
	bounds []float64
	values []float64
	counts []int64
	sum    float64
	count  int64
	tags   map[string]string
	mu     sync.RWMutex
}

func NewHistogram(name string, bounds []float64, tags map[string]string) *Histogram {
	if tags == nil {
		tags = make(map[string]string)
	}
	if len(bounds) == 0 {
		bounds = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	return &Histogram{
		name:   name,
		bounds: bounds,
		counts: make([]int64, len(bounds)+1),
		tags:   tags,
	}
}

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++

	for i, bound := range h.bounds {
		if value <= bound {
			h.counts[i]++
			return
		}
	}
	h.counts[len(h.bounds)]++
}

func (h *Histogram) Quantile(q float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	target := int64(float64(h.count) * q)
	cumulative := int64(0)

	for i, bound := range h.bounds {
		cumulative += h.counts[i]
		if cumulative >= target {
			return bound
		}
	}

	return h.bounds[len(h.bounds)-1]
}

func (h *Histogram) Mean() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

func (h *Histogram) Count() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}

func (h *Histogram) Name() string            { return h.name }
func (h *Histogram) Type() MetricType        { return MetricTypeHistogram }
func (h *Histogram) Value() float64          { return h.Mean() }
func (h *Histogram) Tags() map[string]string { return h.tags }
func (h *Histogram) Timestamp() time.Time    { return time.Now() }

type Summary struct {
	name    string
	quantiles []float64
	values  []float64
	sum     float64
	count   int64
	tags    map[string]string
	mu      sync.RWMutex
}

func NewSummary(name string, quantiles []float64, tags map[string]string) *Summary {
	if tags == nil {
		tags = make(map[string]string)
	}
	if len(quantiles) == 0 {
		quantiles = []float64{0.5, 0.9, 0.95, 0.99}
	}
	return &Summary{
		name:      name,
		quantiles: quantiles,
		tags:      tags,
	}
}

func (s *Summary) Observe(value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = append(s.values, value)
	s.sum += value
	s.count++
}

func (s *Summary) Quantile(q float64) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.values) == 0 {
		return 0
	}

	sorted := make([]float64, len(s.values))
	copy(sorted, s.values)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	idx := int(float64(len(sorted)) * q)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (s *Summary) Mean() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.count == 0 {
		return 0
	}
	return s.sum / float64(s.count)
}

func (s *Summary) Name() string            { return s.name }
func (s *Summary) Type() MetricType        { return MetricTypeSummary }
func (s *Summary) Value() float64          { return s.Mean() }
func (s *Summary) Tags() map[string]string { return s.tags }
func (s *Summary) Timestamp() time.Time    { return time.Now() }

type MetricsRegistry struct {
	metrics map[string]Metric
	mu      sync.RWMutex
}

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		metrics: make(map[string]Metric),
	}
}

func (mr *MetricsRegistry) Register(m Metric) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.metrics[m.Name()] = m
}

func (mr *MetricsRegistry) Get(name string) Metric {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return mr.metrics[name]
}

func (mr *MetricsRegistry) List() []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	names := make([]string, 0, len(mr.metrics))
	for name := range mr.metrics {
		names = append(names, name)
	}
	return names
}

type TraceSpan struct {
	traceID   string
	spanID    string
	parentID  string
	name      string
	startTime time.Time
	endTime   time.Time
	tags      map[string]string
	events    []TraceEvent
	status    string
	mu        sync.RWMutex
}

type TraceEvent struct {
	Name      string
	Timestamp time.Time
	Attributes map[string]string
}

func NewTraceSpan(traceID, spanID, name string) *TraceSpan {
	return &TraceSpan{
		traceID:   traceID,
		spanID:    spanID,
		name:      name,
		startTime: time.Now(),
		tags:      make(map[string]string),
		status:    "ok",
	}
}

func (ts *TraceSpan) SetParentID(parentID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.parentID = parentID
}

func (ts *TraceSpan) SetTag(key, value string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tags[key] = value
}

func (ts *TraceSpan) AddEvent(name string, attrs map[string]string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.events = append(ts.events, TraceEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

func (ts *TraceSpan) Finish() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.endTime = time.Now()
}

func (ts *TraceSpan) Duration() time.Duration {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if ts.endTime.IsZero() {
		return time.Since(ts.startTime)
	}
	return ts.endTime.Sub(ts.startTime)
}

func (ts *TraceSpan) TraceID() string     { return ts.traceID }
func (ts *TraceSpan) SpanID() string      { return ts.spanID }
func (ts *TraceSpan) ParentID() string    { ts.mu.RLock(); defer ts.mu.RUnlock(); return ts.parentID }
func (ts *TraceSpan) Name() string        { return ts.name }
func (ts *TraceSpan) Tags() map[string]string { ts.mu.RLock(); defer ts.mu.RUnlock(); return ts.tags }

type Tracer struct {
	spans map[string]*TraceSpan
	mu    sync.RWMutex
}

func NewTracer() *Tracer {
	return &Tracer{
		spans: make(map[string]*TraceSpan),
	}
}

func (t *Tracer) StartSpan(traceID, spanID, name string) *TraceSpan {
	span := NewTraceSpan(traceID, spanID, name)
	t.mu.Lock()
	t.spans[spanID] = span
	t.mu.Unlock()
	return span
}

func (t *Tracer) FinishSpan(spanID string) {
	t.mu.RLock()
	span, exists := t.spans[spanID]
	t.mu.RUnlock()
	if exists {
		span.Finish()
	}
}

func (t *Tracer) GetSpan(spanID string) *TraceSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.spans[spanID]
}

func (t *Tracer) Export() []*TraceSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()
	spans := make([]*TraceSpan, 0, len(t.spans))
	for _, span := range t.spans {
		spans = append(spans, span)
	}
	return spans
}

type Reporter interface {
	Report(metrics []Metric) error
	Name() string
}

type LogReporter struct {
	mu sync.RWMutex
}

func NewLogReporter() *LogReporter {
	return &LogReporter{}
}

func (lr *LogReporter) Report(metrics []Metric) error {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	for _, m := range metrics {
		fmt.Printf("[%s] %s = %.2f tags=%v\n", m.Type(), m.Name(), m.Value(), m.Tags())
	}
	return nil
}

func (lr *LogReporter) Name() string { return "log" }

type Aggregator struct {
	metrics map[string][]float64
	mu      sync.RWMutex
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		metrics: make(map[string][]float64),
	}
}

func (a *Aggregator) Add(name string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.metrics[name] = append(a.metrics[name], value)
}

func (a *Aggregator) GetStats(name string) (min, max, mean, stddev float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	values, exists := a.metrics[name]
	if !exists || len(values) == 0 {
		return 0, 0, 0, 0
	}

	min = values[0]
	max = values[0]
	sum := 0.0

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	mean = sum / float64(len(values))

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	stddev = math.Sqrt(variance)

	return
}

type SystemMetrics struct {
	mu sync.RWMutex
}

func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{}
}

func (sm *SystemMetrics) Collect() map[string]float64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]float64{
		"go.mem.alloc":      float64(m.Alloc),
		"go.mem.total_alloc": float64(m.TotalAlloc),
		"go.mem.heap_alloc":  float64(m.HeapAlloc),
		"go.mem.heap_sys":    float64(m.HeapSys),
		"go.num_goroutines":  float64(runtime.NumGoroutine()),
		"go.num_cpu":         float64(runtime.NumCPU()),
	}
}

func (sm *SystemMetrics) Name() string { return "system" }
