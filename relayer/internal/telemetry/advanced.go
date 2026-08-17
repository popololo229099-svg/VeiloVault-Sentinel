package telemetry

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type DistributedTracer struct {
	spans    map[string]*TraceSpan
	services map[string]string
	mu       sync.RWMutex
}

func NewDistributedTracer() *DistributedTracer {
	return &DistributedTracer{
		spans:    make(map[string]*TraceSpan),
		services: make(map[string]string),
	}
}

func (dt *DistributedTracer) RegisterService(name, address string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.services[name] = address
}

func (dt *DistributedTracer) StartSpan(traceID, spanID, parentID, name, service string) *TraceSpan {
	span := NewTraceSpan(traceID, spanID, name)
	if parentID != "" {
		span.SetParentID(parentID)
	}
	span.SetTag("service", service)

	dt.mu.Lock()
	dt.spans[spanID] = span
	dt.mu.Unlock()

	return span
}

func (dt *DistributedTracer) FinishSpan(spanID string) {
	dt.mu.RLock()
	span, exists := dt.spans[spanID]
	dt.mu.RUnlock()
	if exists {
		span.Finish()
	}
}

func (dt *DistributedTracer) GetTrace(traceID string) []*TraceSpan {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	var result []*TraceSpan
	for _, span := range dt.spans {
		if span.TraceID() == traceID {
			result = append(result, span)
		}
	}
	return result
}

func (dt *DistributedTracer) GetServiceSpans(service string) []*TraceSpan {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	var result []*TraceSpan
	for _, span := range dt.spans {
		if svc, ok := span.Tags()["service"]; ok && svc == service {
			result = append(result, span)
		}
	}
	return result
}

type MetricsCollector struct {
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
	summaries  map[string]*Summary
	mu         sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
		summaries:  make(map[string]*Summary),
	}
}

func (mc *MetricsCollector) Counter(name string, tags map[string]string) *Counter {
	mc.mu.RLock()
	if c, ok := mc.counters[name]; ok {
		mc.mu.RUnlock()
		return c
	}
	mc.mu.RUnlock()

	c := NewCounter(name, tags)
	mc.mu.Lock()
	mc.counters[name] = c
	mc.mu.Unlock()
	return c
}

func (mc *MetricsCollector) Gauge(name string, tags map[string]string) *Gauge {
	mc.mu.RLock()
	if g, ok := mc.gauges[name]; ok {
		mc.mu.RUnlock()
		return g
	}
	mc.mu.RUnlock()

	g := NewGauge(name, tags)
	mc.mu.Lock()
	mc.gauges[name] = g
	mc.mu.Unlock()
	return g
}

func (mc *MetricsCollector) Histogram(name string, bounds []float64, tags map[string]string) *Histogram {
	mc.mu.RLock()
	if h, ok := mc.histograms[name]; ok {
		mc.mu.RUnlock()
		return h
	}
	mc.mu.RUnlock()

	h := NewHistogram(name, bounds, tags)
	mc.mu.Lock()
	mc.histograms[name] = h
	mc.mu.Unlock()
	return h
}

func (mc *MetricsCollector) Summary(name string, quantiles []float64, tags map[string]string) *Summary {
	mc.mu.RLock()
	if s, ok := mc.summaries[name]; ok {
		mc.mu.RUnlock()
		return s
	}
	mc.mu.RUnlock()

	s := NewSummary(name, quantiles, tags)
	mc.mu.Lock()
	mc.summaries[name] = s
	mc.mu.Unlock()
	return s
}

func (mc *MetricsCollector) Snapshot() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := make(map[string]interface{})
	for name, c := range mc.counters {
		snapshot["counter_"+name] = c.Value()
	}
	for name, g := range mc.gauges {
		snapshot["gauge_"+name] = g.Value()
	}
	for name, h := range mc.histograms {
		snapshot["histogram_"+name+"_mean"] = h.Mean()
		snapshot["histogram_"+name+"_count"] = h.Count()
	}
	for name, s := range mc.summaries {
		snapshot["summary_"+name+"_mean"] = s.Mean()
	}
	return snapshot
}

type SpanContext struct {
	TraceID    string
	SpanID     string
	ParentID   string
	SampleRate float64
	Baggage    map[string]string
	mu         sync.RWMutex
}

func NewSpanContext(traceID, spanID string) *SpanContext {
	return &SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		SampleRate: 1.0,
		Baggage:    make(map[string]string),
	}
}

func (sc *SpanContext) SetBaggage(key, value string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Baggage[key] = value
}

func (sc *SpanContext) GetBaggage(key string) string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.Baggage[key]
}

type SpanLink struct {
	TraceID string
	SpanID  string
}

type EnrichedSpan struct {
	*TraceSpan
	Context   *SpanContext
	Links     []SpanLink
	Resources map[string]string
	mu        sync.RWMutex
}

func NewEnrichedSpan(traceID, spanID, name string) *EnrichedSpan {
	return &EnrichedSpan{
		TraceSpan: NewTraceSpan(traceID, spanID, name),
		Context:   NewSpanContext(traceID, spanID),
		Links:     make([]SpanLink, 0),
		Resources: make(map[string]string),
	}
}

func (es *EnrichedSpan) AddLink(link SpanLink) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.Links = append(es.Links, link)
}

func (es *EnrichedSpan) SetResource(key, value string) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.Resources[key] = value
}

type SamplingStrategy interface {
	ShouldSample(traceID string) bool
	Name() string
}

type ProbabilisticSampling struct {
	rate float64
	mu   sync.RWMutex
}

func NewProbabilisticSampling(rate float64) *ProbabilisticSampling {
	if rate <= 0 || rate > 1 {
		rate = 0.1
	}
	return &ProbabilisticSampling{rate: rate}
}

func (ps *ProbabilisticSampling) ShouldSample(traceID string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	hash := uint32(0)
	for _, c := range traceID {
		hash = hash*31 + uint32(c)
	}
	return float64(hash%1000)/1000.0 < ps.rate
}

func (ps *ProbabilisticSampling) Name() string { return "probabilistic" }

type RateLimitSampling struct {
	maxPerSecond int
	current      int
	lastReset    time.Time
	mu           sync.Mutex
}

func NewRateLimitSampling(maxPerSecond int) *RateLimitSampling {
	if maxPerSecond <= 0 {
		maxPerSecond = 100
	}
	return &RateLimitSampling{
		maxPerSecond: maxPerSecond,
		lastReset:    time.Now(),
	}
}

func (rls *RateLimitSampling) ShouldSample(traceID string) bool {
	rls.mu.Lock()
	defer rls.mu.Unlock()

	now := time.Now()
	if now.Sub(rls.lastReset) >= time.Second {
		rls.current = 0
		rls.lastReset = now
	}

	rls.current++
	return rls.current <= rls.maxPerSecond
}

func (rls *RateLimitSampling) Name() string { return "rate_limit" }

type AdaptiveSampling struct {
	baseRate      float64
	currentRate   float64
	errorThreshold float64
	mu            sync.RWMutex
}

func NewAdaptiveSampling(baseRate, errorThreshold float64) *AdaptiveSampling {
	if baseRate <= 0 || baseRate > 1 {
		baseRate = 0.1
	}
	if errorThreshold <= 0 {
		errorThreshold = 0.01
	}
	return &AdaptiveSampling{
		baseRate:       baseRate,
		currentRate:    baseRate,
		errorThreshold: errorThreshold,
	}
}

func (as *AdaptiveSampling) ShouldSample(traceID string) bool {
	as.mu.RLock()
	rate := as.currentRate
	as.mu.RUnlock()

	hash := uint32(0)
	for _, c := range traceID {
		hash = hash*31 + uint32(c)
	}
	return float64(hash%1000)/1000.0 < rate
}

func (as *AdaptiveSampling) Adjust(errorRate float64) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if errorRate > as.errorThreshold {
		as.currentRate = math.Max(as.currentRate*0.5, 0.001)
	} else {
		as.currentRate = math.Min(as.currentRate*1.5, as.baseRate*10)
	}
}

func (as *AdaptiveSampling) Name() string { return "adaptive" }

type MetricExporter interface {
	Export(metrics []Metric) error
	Name() string
}

type InfluxDBExporter struct {
	url    string
	token  string
	org    string
	bucket string
	mu     sync.RWMutex
}

func NewInfluxDBExporter(url, token, org, bucket string) *InfluxDBExporter {
	return &InfluxDBExporter{
		url:    url,
		token:  token,
		org:    org,
		bucket: bucket,
	}
}

func (ie *InfluxDBExporter) Export(metrics []Metric) error {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	for _, m := range metrics {
		line := fmt.Sprintf("%s,service=relayer value=%.2f %d",
			m.Name(), m.Value(), time.Now().UnixNano())
		_ = line
	}
	return nil
}

func (ie *InfluxDBExporter) Name() string { return "influxdb" }

type PrometheusExporter struct {
	namespace string
	subsystem string
	mu        sync.RWMutex
}

func NewPrometheusExporter(namespace, subsystem string) *PrometheusExporter {
	return &PrometheusExporter{
		namespace: namespace,
		subsystem: subsystem,
	}
}

func (pe *PrometheusExporter) Export(metrics []Metric) error {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	for _, m := range metrics {
		line := fmt.Sprintf("%s_%s_%s %f %d",
			pe.namespace, pe.subsystem, m.Name(), m.Value(), time.Now().UnixNano())
		_ = line
	}
	return nil
}

func (pe *PrometheusExporter) Name() string { return "prometheus" }

type MetricBuffer struct {
	buffer   []Metric
	maxSize  int
	flushFn  func([]Metric) error
	mu       sync.RWMutex
}

func NewMetricBuffer(maxSize int, flushFn func([]Metric) error) *MetricBuffer {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MetricBuffer{
		buffer:  make([]Metric, 0, maxSize),
		maxSize: maxSize,
		flushFn: flushFn,
	}
}

func (mb *MetricBuffer) Add(m Metric) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.buffer = append(mb.buffer, m)

	if len(mb.buffer) >= mb.maxSize {
		mb.flush()
	}
}

func (mb *MetricBuffer) Flush() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.flush()
}

func (mb *MetricBuffer) flush() {
	if mb.flushFn != nil && len(mb.buffer) > 0 {
		batch := make([]Metric, len(mb.buffer))
		copy(batch, mb.buffer)
		mb.buffer = mb.buffer[:0]
		mb.mu.Unlock()
		_ = mb.flushFn(batch)
		mb.mu.Lock()
	}
}

func (mb *MetricBuffer) Size() int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return len(mb.buffer)
}

type CorrelationID struct {
	id       string
	parentID string
	service  string
	mu       sync.RWMutex
}

func NewCorrelationID(id, parentID, service string) *CorrelationID {
	return &CorrelationID{
		id:       id,
		parentID: parentID,
		service:  service,
	}
}

func (cid *CorrelationID) ID() string       { cid.mu.RLock(); defer cid.mu.RUnlock(); return cid.id }
func (cid *CorrelationID) ParentID() string { cid.mu.RLock(); defer cid.mu.RUnlock(); return cid.parentID }
func (cid *CorrelationID) Service() string  { cid.mu.RLock(); defer cid.mu.RUnlock(); return cid.service }
