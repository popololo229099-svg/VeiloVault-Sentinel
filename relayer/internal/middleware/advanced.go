package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
	"time"
)

type CompressionConfig struct {
	Level     int
	MinLength int
	Methods   []string
}

type CompressMiddleware struct {
	config CompressionConfig
}

func NewCompressMiddleware(config CompressionConfig) *CompressMiddleware {
	if config.MinLength == 0 {
		config.MinLength = 1024
	}
	return &CompressMiddleware{config: config}
}

func (cm *CompressMiddleware) ShouldCompress(method string) bool {
	if len(cm.config.Methods) == 0 {
		return true
	}
	for _, m := range cm.config.Methods {
		if m == method {
			return true
		}
	}
	return false
}

type RequestBody struct {
	Data      []byte
	Headers   map[string]string
	Size      int
	ReadOnly  bool
	mu        sync.RWMutex
}

func NewRequestBody(data []byte) *RequestBody {
	return &RequestBody{
		Data:    data,
		Headers: make(map[string]string),
		Size:    len(data),
	}
}

func (rb *RequestBody) Read(p []byte) (int, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.Size == 0 {
		return 0, io.EOF
	}
	n := copy(p, rb.Data)
	return n, nil
}

func (rb *RequestBody) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

type MetricsMiddleware struct {
	RequestCount   int64
	ErrorCount     int64
	TotalDuration  time.Duration
	mu             sync.RWMutex
	statusCounts   map[int]int64
	methodCounts   map[string]int64
}

func NewMetricsMiddleware() *MetricsMiddleware {
	return &MetricsMiddleware{
		statusCounts: make(map[int]int64),
		methodCounts: make(map[string]int64),
	}
}

func (mm *MetricsMiddleware) RecordRequest(method string, status int, duration time.Duration) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.RequestCount++
	mm.TotalDuration += duration
	mm.statusCounts[status]++
	mm.methodCounts[method]++
}

func (mm *MetricsMiddleware) RecordError() {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.ErrorCount++
}

func (mm *MetricsMiddleware) Stats() map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	avgDuration := time.Duration(0)
	if mm.RequestCount > 0 {
		avgDuration = mm.TotalDuration / time.Duration(mm.RequestCount)
	}
	return map[string]interface{}{
		"request_count":   mm.RequestCount,
		"error_count":     mm.ErrorCount,
		"avg_duration_ms": avgDuration.Milliseconds(),
		"status_counts":   mm.statusCounts,
		"method_counts":   mm.methodCounts,
	}
}

func (mm *MetricsMiddleware) Reset() {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.RequestCount = 0
	mm.ErrorCount = 0
	mm.TotalDuration = 0
	mm.statusCounts = make(map[int]int64)
	mm.methodCounts = make(map[string]int64)
}

type IPFilterConfig struct {
	AllowedIPs   []string
	BlockedIPs   []string
	AllowPrivate bool
}

type IPFilter struct {
	config IPFilterConfig
}

func NewIPFilter(config IPFilterConfig) *IPFilter {
	return &IPFilter{config: config}
}

func (f *IPFilter) IsAllowed(ip string) bool {
	for _, blocked := range f.config.BlockedIPs {
		if ip == blocked {
			return false
		}
	}
	if len(f.config.AllowedIPs) == 0 {
		return true
	}
	for _, allowed := range f.config.AllowedIPs {
		if ip == allowed {
			return true
		}
	}
	return false
}

type TraceIDGenerator struct{}

func (t *TraceIDGenerator) Generate() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type ContextKey string

const (
	TraceIDKey    ContextKey = "trace_id"
	RequestTimeKey ContextKey = "request_time"
)

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

func WithRequestTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, RequestTimeKey, t)
}

func GetRequestTime(ctx context.Context) time.Time {
	if v, ok := ctx.Value(RequestTimeKey).(time.Time); ok {
		return v
	}
	return time.Time{}
}
