package pattern

import (
	"time"

	"github.com/rs/zerolog"
)

// TransactionProcessor is the base interface for the Decorator pattern.
type TransactionProcessor interface {
	Process(data []byte) ([]byte, error)
}

// BaseProcessor is a simple processor that passes data through.
type BaseProcessor struct{}

func (p *BaseProcessor) Process(data []byte) ([]byte, error) {
	return data, nil
}

// --- Decorators ---

// LoggingDecorator wraps a processor with structured logging.
type LoggingDecorator struct {
	inner  TransactionProcessor
	logger zerolog.Logger
}

func NewLoggingDecorator(inner TransactionProcessor, logger zerolog.Logger) *LoggingDecorator {
	return &LoggingDecorator{inner: inner, logger: logger}
}

func (d *LoggingDecorator) Process(data []byte) ([]byte, error) {
	start := time.Now()
	result, err := d.inner.Process(data)
 duration := time.Since(start)

	d.logger.Info().
		Int("input_len", len(data)).
		Int("output_len", len(result)).
		Dur("duration", duration).
		Bool("success", err == nil).
		Msg("transaction processed")

	return result, err
}

// MetricsDecorator wraps a processor with timing metrics.
type MetricsDecorator struct {
	inner      TransactionProcessor
	totalCount int64
	totalTime  time.Duration
	lastErr    error
}

func NewMetricsDecorator(inner TransactionProcessor) *MetricsDecorator {
	return &MetricsDecorator{inner: inner}
}

func (d *MetricsDecorator) Process(data []byte) ([]byte, error) {
	start := time.Now()
	result, err := d.inner.Process(data)
	duration := time.Since(start)

	d.totalCount++
	d.totalTime += duration
	d.lastErr = err

	return result, err
}

func (d *MetricsDecorator) Stats() (count int64, avgTime time.Duration, lastErr error) {
	if d.totalCount > 0 {
		avgTime = d.totalTime / time.Duration(d.totalCount)
	}
	return d.totalCount, avgTime, d.lastErr
}

// ValidationDecorator wraps a processor with input validation.
type ValidationDecorator struct {
	inner    TransactionProcessor
	maxSize  int
	validate func([]byte) error
}

func NewValidationDecorator(inner TransactionProcessor, maxSize int, validate func([]byte) error) *ValidationDecorator {
	return &ValidationDecorator{inner: inner, maxSize: maxSize, validate: validate}
}

func (d *ValidationDecorator) Process(data []byte) ([]byte, error) {
	if len(data) > d.maxSize {
		return nil, ErrPayloadTooLarge
	}
	if d.validate != nil {
		if err := d.validate(data); err != nil {
			return nil, err
		}
	}
	return d.inner.Process(data)
}

// RetryDecorator wraps a processor with retry logic.
type RetryDecorator struct {
	inner      TransactionProcessor
	maxRetries int
	delay      time.Duration
}

func NewRetryDecorator(inner TransactionProcessor, maxRetries int, delay time.Duration) *RetryDecorator {
	return &RetryDecorator{inner: inner, maxRetries: maxRetries, delay: delay}
}

func (d *RetryDecorator) Process(data []byte) ([]byte, error) {
	var lastErr error
	for i := 0; i <= d.maxRetries; i++ {
		result, err := d.inner.Process(data)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < d.maxRetries {
			time.Sleep(d.delay * time.Duration(i+1))
		}
	}
	return nil, lastErr
}

// CacheDecorator wraps a processor with result caching.
type CacheDecorator struct {
	inner TransactionProcessor
	cache map[string][]byte
}

func NewCacheDecorator(inner TransactionProcessor) *CacheDecorator {
	return &CacheDecorator{inner: inner, cache: make(map[string][]byte)}
}

func (d *CacheDecorator) Process(data []byte) ([]byte, error) {
	key := string(data)
	if cached, ok := d.cache[key]; ok {
		return cached, nil
	}
	result, err := d.inner.Process(data)
	if err == nil {
		d.cache[key] = result
	}
	return result, err
}
