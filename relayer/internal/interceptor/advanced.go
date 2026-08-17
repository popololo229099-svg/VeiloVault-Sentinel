package interceptor

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

type RequestLogger struct {
	logFunc func(string, ...interface{})
	mu      sync.RWMutex
}

func NewRequestLogger(logFunc func(string, ...interface{})) *RequestLogger {
	if logFunc == nil {
		logFunc = func(format string, args ...interface{}) {}
	}
	return &RequestLogger{logFunc: logFunc}
}

func (rl *RequestLogger) LogRequest(req *Request) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	body := ""
	if req.Body != nil {
		body = string(req.Body)
	}
	rl.logFunc("Request: %s %s body=%s headers=%v", req.Method, req.Path, body, req.Headers)
}

func (rl *RequestLogger) LogResponse(resp *Response) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	body := ""
	if resp.Body != nil {
		body = string(resp.Body)
	}
	rl.logFunc("Response: status=%d body=%s duration=%v", resp.StatusCode, body, resp.Duration)
}

type RequestValidatorInterceptor struct {
	rules   []ValidationRule
	mu      sync.RWMutex
}

type ValidationRule interface {
	Validate(req *Request) error
	Name() string
}

func NewRequestValidatorInterceptor(rules ...ValidationRule) *RequestValidatorInterceptor {
	return &RequestValidatorInterceptor{rules: rules}
}

func (rvi *RequestValidatorInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	rvi.mu.RLock()
	defer rvi.mu.RUnlock()

	for _, rule := range rvi.rules {
		if err := rule.Validate(req); err != nil {
			return &Response{
				StatusCode: 400,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       []byte(`{"error":"` + err.Error() + `"}`),
			}, err
		}
	}
	return next(req)
}

func (rvi *RequestValidatorInterceptor) Name() string { return "request_validator" }

type MethodValidator struct {
	allowedMethods map[string]bool
	mu             sync.RWMutex
}

func NewMethodValidator(methods ...string) *MethodValidator {
	allowed := make(map[string]bool, len(methods))
	for _, m := range methods {
		allowed[m] = true
	}
	return &MethodValidator{allowedMethods: allowed}
}

func (mv *MethodValidator) Validate(req *Request) error {
	mv.mu.RLock()
	defer mv.mu.RUnlock()
	if !mv.allowedMethods[req.Method] {
		return fmt.Errorf("method %s not allowed", req.Method)
	}
	return nil
}

func (mv *MethodValidator) Name() string { return "method_validator" }

type PathValidator struct {
	allowedPaths map[string]bool
	mu           sync.RWMutex
}

func NewPathValidator(paths ...string) *PathValidator {
	allowed := make(map[string]bool, len(paths))
	for _, p := range paths {
		allowed[p] = true
	}
	return &PathValidator{allowedPaths: allowed}
}

func (pv *PathValidator) Validate(req *Request) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()
	if !pv.allowedPaths[req.Path] {
		return fmt.Errorf("path %s not allowed", req.Path)
	}
	return nil
}

func (pv *PathValidator) Name() string { return "path_validator" }

type BodySizeValidator struct {
	maxSize int
	mu      sync.RWMutex
}

func NewBodySizeValidator(maxSize int) *BodySizeValidator {
	if maxSize <= 0 {
		maxSize = 1024 * 1024
	}
	return &BodySizeValidator{maxSize: maxSize}
}

func (bsv *BodySizeValidator) Validate(req *Request) error {
	bsv.mu.RLock()
	defer bsv.mu.RUnlock()
	if req.Body != nil && len(req.Body) > bsv.maxSize {
		return fmt.Errorf("body size %d exceeds max %d", len(req.Body), bsv.maxSize)
	}
	return nil
}

func (bsv *BodySizeValidator) Name() string { return "body_size_validator" }

type HeaderValidator struct {
	requiredHeaders map[string]bool
	mu              sync.RWMutex
}

func NewHeaderValidator(headers ...string) *HeaderValidator {
	required := make(map[string]bool, len(headers))
	for _, h := range headers {
		required[h] = true
	}
	return &HeaderValidator{requiredHeaders: required}
}

func (hv *HeaderValidator) Validate(req *Request) error {
	hv.mu.RLock()
	defer hv.mu.RUnlock()
	for header := range hv.requiredHeaders {
		if _, ok := req.Headers[header]; !ok {
			return fmt.Errorf("required header %s missing", header)
		}
	}
	return nil
}

func (hv *HeaderValidator) Name() string { return "header_validator" }

type CircuitBreakerInterceptor struct {
	failureCount  int
	successCount  int
	state         string
	threshold     int
	resetTimeout  time.Duration
	lastFailure   time.Time
	mu            sync.RWMutex
}

func NewCircuitBreakerInterceptor(threshold int, resetTimeout time.Duration) *CircuitBreakerInterceptor {
	if threshold <= 0 {
		threshold = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &CircuitBreakerInterceptor{
		state:        "closed",
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cbi *CircuitBreakerInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	cbi.mu.Lock()

	if cbi.state == "open" {
		if time.Since(cbi.lastFailure) > cbi.resetTimeout {
			cbi.state = "half-open"
			cbi.successCount = 0
		} else {
			cbi.mu.Unlock()
			return &Response{
				StatusCode: 503,
				Error:      fmt.Errorf("circuit breaker is open"),
			}, fmt.Errorf("circuit breaker is open")
		}
	}
	cbi.mu.Unlock()

	resp, err := next(req)

	cbi.mu.Lock()
	defer cbi.mu.Unlock()

	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		cbi.failureCount++
		cbi.lastFailure = time.Now()
		if cbi.failureCount >= cbi.threshold {
			cbi.state = "open"
		}
	} else {
		cbi.successCount++
		if cbi.state == "half-open" {
			cbi.state = "closed"
			cbi.failureCount = 0
		}
	}

	return resp, err
}

func (cbi *CircuitBreakerInterceptor) Name() string { return "circuit_breaker" }

func (cbi *CircuitBreakerInterceptor) State() string {
	cbi.mu.RLock()
	defer cbi.mu.RUnlock()
	return cbi.state
}

type DecompressionInterceptor struct {
	mu sync.RWMutex
}

func NewDecompressionInterceptor() *DecompressionInterceptor {
	return &DecompressionInterceptor{}
}

func (di *DecompressionInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	di.mu.RLock()
	defer di.mu.RUnlock()

	if encoding, ok := req.Headers["Content-Encoding"]; ok {
		switch encoding {
		case "identity":
		default:
			if req.Body != nil {
				req.Body = bytes.TrimSpace(req.Body)
			}
		}
	}

	return next(req)
}

func (di *DecompressionInterceptor) Name() string { return "decompression" }

type ResponseModifierInterceptor struct {
	modifier func(*Response) *Response
	mu       sync.RWMutex
}

func NewResponseModifierInterceptor(modifier func(*Response) *Response) *ResponseModifierInterceptor {
	return &ResponseModifierInterceptor{modifier: modifier}
}

func (rmi *ResponseModifierInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	rmi.mu.RLock()
	modifier := rmi.modifier
	rmi.mu.RUnlock()

	if modifier != nil {
		resp = modifier(resp)
	}

	return resp, nil
}

func (rmi *ResponseModifierInterceptor) Name() string { return "response_modifier" }

type ParallelInterceptor struct {
	interceptors []Interceptor
	mu           sync.RWMutex
}

func NewParallelInterceptor(interceptors ...Interceptor) *ParallelInterceptor {
	return &ParallelInterceptor{interceptors: interceptors}
}

func (pi *ParallelInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	pi.mu.RLock()
	interceptors := make([]Interceptor, len(pi.interceptors))
	copy(interceptors, pi.interceptors)
	pi.mu.RUnlock()

	type result struct {
		resp *Response
		err  error
	}

	results := make(chan result, len(interceptors))
	for _, interceptor := range interceptors {
		go func(i Interceptor) {
			resp, err := i.Intercept(req, next)
			results <- result{resp, err}
		}(interceptor)
	}

	var lastErr error
	var lastResp *Response
	for i := 0; i < len(interceptors); i++ {
		r := <-results
		if r.err != nil {
			lastErr = r.err
		}
		if r.resp != nil {
			lastResp = r.resp
		}
	}

	return lastResp, lastErr
}

func (pi *ParallelInterceptor) Name() string { return "parallel" }

type ConditionalInterceptor struct {
	condition func(*Request) bool
	trueInt   Interceptor
	falseInt  Interceptor
	mu        sync.RWMutex
}

func NewConditionalInterceptor(condition func(*Request) bool, trueInt, falseInt Interceptor) *ConditionalInterceptor {
	return &ConditionalInterceptor{
		condition: condition,
		trueInt:   trueInt,
		falseInt:  falseInt,
	}
}

func (ci *ConditionalInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	ci.mu.RLock()
	cond := ci.condition
	trueInt := ci.trueInt
	falseInt := ci.falseInt
	ci.mu.RUnlock()

	if cond(req) && trueInt != nil {
		return trueInt.Intercept(req, next)
	}
	if falseInt != nil {
		return falseInt.Intercept(req, next)
	}
	return next(req)
}

func (ci *ConditionalInterceptor) Name() string { return "conditional" }

type TimeoutPerPathInterceptor struct {
	timeouts map[string]time.Duration
	default_ time.Duration
	mu       sync.RWMutex
}

func NewTimeoutPerPathInterceptor(defaultTimeout time.Duration) *TimeoutPerPathInterceptor {
	if defaultTimeout <= 0 {
		defaultTimeout = 30 * time.Second
	}
	return &TimeoutPerPathInterceptor{
		timeouts: make(map[string]time.Duration),
		default_: defaultTimeout,
	}
}

func (tpi *TimeoutPerPathInterceptor) SetTimeout(path string, timeout time.Duration) {
	tpi.mu.Lock()
	defer tpi.mu.Unlock()
	tpi.timeouts[path] = timeout
}

func (tpi *TimeoutPerPathInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	tpi.mu.RLock()
	timeout, ok := tpi.timeouts[req.Path]
	if !ok {
		timeout = tpi.default_
	}
	tpi.mu.RUnlock()

	type result struct {
		resp *Response
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		resp, err := next(req)
		ch <- result{resp, err}
	}()

	select {
	case r := <-ch:
		return r.resp, r.err
	case <-time.After(timeout):
		return &Response{
			StatusCode: 408,
			Error:      fmt.Errorf("timeout for path %s", req.Path),
		}, fmt.Errorf("timeout after %v", timeout)
	}
}

func (tpi *TimeoutPerPathInterceptor) Name() string { return "timeout_per_path" }

type RequestClonerInterceptor struct {
	mu sync.RWMutex
}

func NewRequestClonerInterceptor() *RequestClonerInterceptor {
	return &RequestClonerInterceptor{}
}

func (rci *RequestClonerInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	rci.mu.RLock()
	defer rci.mu.RUnlock()

	clone := &Request{
		Method:    req.Method,
		Path:      req.Path,
		Headers:   make(map[string]string),
		Timestamp: req.Timestamp,
		Metadata:  make(map[string]interface{}),
	}

	for k, v := range req.Headers {
		clone.Headers[k] = v
	}

	for k, v := range req.Metadata {
		clone.Metadata[k] = v
	}

	if req.Body != nil {
		clone.Body = make([]byte, len(req.Body))
		copy(clone.Body, req.Body)
	}

	return next(clone)
}

func (rci *RequestClonerInterceptor) Name() string { return "request_cloner" }

type DrainBodyInterceptor struct {
	mu sync.RWMutex
}

func NewDrainBodyInterceptor() *DrainBodyInterceptor {
	return &DrainBodyInterceptor{}
}

func (dbi *DrainBodyInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	resp, err := next(req)

	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, bytes.NewReader(resp.Body))
	}

	return resp, err
}

func (dbi *DrainBodyInterceptor) Name() string { return "drain_body" }
