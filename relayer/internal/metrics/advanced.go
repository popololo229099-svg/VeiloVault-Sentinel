package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type MetricsSnapshot struct {
	Timestamp time.Time
	Counters  map[string]float64
	Gauges    map[string]float64
	Histograms map[string]HistogramSnapshot
}

type HistogramSnapshot struct {
	Count int64
	Mean  float64
	P50   float64
	P95   float64
	P99   float64
	Min   float64
	Max   float64
}

type MetricsExporter struct {
	snapshots []MetricsSnapshot
	maxSize   int
	mu        sync.Mutex
}

func NewMetricsExporter(maxSize int) *MetricsExporter {
	return &MetricsExporter{
		snapshots: make([]MetricsSnapshot, 0),
		maxSize:   maxSize,
	}
}

func (me *MetricsExporter) Record(snapshot MetricsSnapshot) {
	me.mu.Lock()
	defer me.mu.Unlock()
	if len(me.snapshots) >= me.maxSize {
		me.snapshots = me.snapshots[1:]
	}
	me.snapshots = append(me.snapshots, snapshot)
}

func (me *MetricsExporter) Latest() *MetricsSnapshot {
	me.mu.Lock()
	defer me.mu.Unlock()
	if len(me.snapshots) == 0 {
		return nil
	}
	s := me.snapshots[len(me.snapshots)-1]
	return &s
}

func (me *MetricsExporter) History(n int) []MetricsSnapshot {
	me.mu.Lock()
	defer me.mu.Unlock()
	if n > len(me.snapshots) {
		n = len(me.snapshots)
	}
	result := make([]MetricsSnapshot, n)
	copy(result, me.snapshots[len(me.snapshots)-n:])
	return result
}

func (me *MetricsExporter) ExportPrometheus() string {
	me.mu.Lock()
	defer me.mu.Unlock()
	var sb strings.Builder
	if len(me.snapshots) == 0 {
		return ""
	}
	latest := me.snapshots[len(me.snapshots)-1]
	for name, val := range latest.Counters {
		sb.WriteString(fmt.Sprintf("veilo_%s %f\n", name, val))
	}
	for name, val := range latest.Gauges {
		sb.WriteString(fmt.Sprintf("veilo_%s %f\n", name, val))
	}
	for name, hist := range latest.Histograms {
		sb.WriteString(fmt.Sprintf("veilo_%s_count %d\n", name, hist.Count))
		sb.WriteString(fmt.Sprintf("veilo_%s_sum %f\n", name, hist.Mean*float64(hist.Count)))
	}
	return sb.String()
}

type AlertRule struct {
	Name      string
	Metric    string
	Condition string
	Threshold float64
	Duration  time.Duration
}

type AlertManager struct {
	rules    []AlertRule
	active   map[string]*Alert
	history  []Alert
	mu       sync.RWMutex
}

type Alert struct {
	Rule      AlertRule
	Value     float64
	Triggered time.Time
	Resolved  *time.Time
}

func NewAlertManager() *AlertManager {
	return &AlertManager{
		rules:  make([]AlertRule, 0),
		active: make(map[string]*Alert),
	}
}

func (am *AlertManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

func (am *AlertManager) Evaluate(metrics map[string]float64) []Alert {
	am.mu.Lock()
	defer am.mu.Unlock()
	var triggered []Alert
	for _, rule := range am.rules {
		val, ok := metrics[rule.Metric]
		if !ok {
			continue
		}
		exceeds := false
		switch rule.Condition {
		case "gt":
			exceeds = val > rule.Threshold
		case "lt":
			exceeds = val < rule.Threshold
		case "gte":
			exceeds = val >= rule.Threshold
		case "lte":
			exceeds = val <= rule.Threshold
		}
		if exceeds {
			if _, active := am.active[rule.Name]; !active {
				alert := Alert{Rule: rule, Value: val, Triggered: time.Now()}
				am.active[rule.Name] = &alert
				am.history = append(am.history, alert)
				triggered = append(triggered, alert)
			}
		} else {
			if alert, active := am.active[rule.Name]; active {
				now := time.Now()
				alert.Resolved = &now
				delete(am.active, rule.Name)
			}
		}
	}
	return triggered
}

func (am *AlertManager) ActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]Alert, 0, len(am.active))
	for _, a := range am.active {
		result = append(result, *a)
	}
	return result
}

func (am *AlertManager) AlertHistory(n int) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	if n > len(am.history) {
		n = len(am.history)
	}
	result := make([]Alert, n)
	copy(result, am.history[len(am.history)-n:])
	return result
}

type PercentileCalculator struct {
	values []float64
	mu     sync.Mutex
}

func NewPercentileCalculator() *PercentileCalculator {
	return &PercentileCalculator{values: make([]float64, 0)}
}

func (pc *PercentileCalculator) Add(value float64) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.values = append(pc.values, value)
}

func (pc *PercentileCalculator) P50() float64  { return pc.percentile(0.50) }
func (pc *PercentileCalculator) P90() float64  { return pc.percentile(0.90) }
func (pc *PercentileCalculator) P95() float64  { return pc.percentile(0.95) }
func (pc *PercentileCalculator) P99() float64  { return pc.percentile(0.99) }

func (pc *PercentileCalculator) percentile(p float64) float64 {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if len(pc.values) == 0 {
		return 0
	}
	sorted := make([]float64, len(pc.values))
	copy(sorted, pc.values)
	sort.Float64s(sorted)
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func (pc *PercentileCalculator) Reset() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.values = pc.values[:0]
}

func (pc *PercentileCalculator) Count() int {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return len(pc.values)
}

type MetricPath struct {
	Service string
	Method  string
	Status  string
}

func (mp MetricPath) String() string {
	return fmt.Sprintf("%s.%s.%s", mp.Service, mp.Method, mp.Status)
}

type LatencyTracker struct {
	tracks map[string]*PercentileCalculator
	mu     sync.RWMutex
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{tracks: make(map[string]*PercentileCalculator)}
}

func (lt *LatencyTracker) Record(path string, duration time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if _, ok := lt.tracks[path]; !ok {
		lt.tracks[path] = NewPercentileCalculator()
	}
	lt.tracks[path].Add(float64(duration.Microseconds()))
}

func (lt *LatencyTracker) Snapshot(path string) map[string]float64 {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	pc, ok := lt.tracks[path]
	if !ok {
		return nil
	}
	return map[string]float64{
		"p50": pc.P50(),
		"p90": pc.P90(),
		"p95": pc.P95(),
		"p99": pc.P99(),
	}
}
