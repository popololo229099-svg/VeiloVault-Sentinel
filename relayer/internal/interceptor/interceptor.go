package interceptor

import (
	"fmt"
	"sync"
	"time"
)

type Request struct {
	Method    string
	Path      string
	Headers   map[string]string
	Body      []byte
	Timestamp time.Time
	Metadata  map[string]interface{}
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Duration   time.Duration
	Error      error
}

type Handler func(req *Request) (*Response, error)

type Interceptor interface {
	Intercept(req *Request, next Handler) (*Response, error)
	Name() string
}

type InterceptorChain struct {
	interceptors []Interceptor
	mu           sync.RWMutex
}

func NewInterceptorChain(interceptors ...Interceptor) *InterceptorChain {
	return &InterceptorChain{
		interceptors: interceptors,
	}
}

func (ic *InterceptorChain) Add(interceptor Interceptor) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.interceptors = append(ic.interceptors, interceptor)
}

func (ic *InterceptorChain) Execute(req *Request, final Handler) (*Response, error) {
	ic.mu.RLock()
	interceptors := make([]Interceptor, len(ic.interceptors))
	copy(interceptors, ic.interceptors)
	ic.mu.RUnlock()

	handler := final
	for i := len(interceptors) - 1; i >= 0; i-- {
		interceptor := interceptors[i]
		next := handler
		handler = func(r *Request) (*Response, error) {
			return interceptor.Intercept(r, next)
		}
	}

	return handler(req)
}

type LoggingInterceptor struct {
	logFunc func(string, ...interface{})
	mu      sync.RWMutex
}

func NewLoggingInterceptor(logFunc func(string, ...interface{})) *LoggingInterceptor {
	if logFunc == nil {
		logFunc = func(format string, args ...interface{}) {}
	}
	return &LoggingInterceptor{logFunc: logFunc}
}

func (li *LoggingInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	li.mu.RLock()
	defer li.mu.RUnlock()

	start := time.Now()
	li.logFunc("request started: %s %s", req.Method, req.Path)

	resp, err := next(req)

	duration := time.Since(start)
	if err != nil {
		li.logFunc("request failed: %s %s in %v: %v", req.Method, req.Path, duration, err)
	} else {
		li.logFunc("request completed: %s %s in %v with status %d", req.Method, req.Path, duration, resp.StatusCode)
	}

	return resp, err
}

func (li *LoggingInterceptor) Name() string { return "logging" }

type MetricsInterceptor struct {
	requestCount    int64
	errorCount      int64
	totalDuration   time.Duration
	maxDuration     time.Duration
	mu              sync.RWMutex
}

func NewMetricsInterceptor() *MetricsInterceptor {
	return &MetricsInterceptor{}
}

func (mi *MetricsInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	mi.mu.Lock()
	start := time.Now()

	resp, err := next(req)

	duration := time.Since(start)
	mi.requestCount++
	mi.totalDuration += duration
	if duration > mi.maxDuration {
		mi.maxDuration = duration
	}
	if err != nil {
		mi.errorCount++
	}
	mi.mu.Unlock()

	return resp, err
}

func (mi *MetricsInterceptor) Metrics() (count, errors int64, avgDuration time.Duration, maxDuration time.Duration) {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	count = mi.requestCount
	errors = mi.errorCount
	maxDuration = mi.maxDuration
	if mi.requestCount > 0 {
		avgDuration = mi.totalDuration / time.Duration(mi.requestCount)
	}
	return
}

func (mi *MetricsInterceptor) Name() string { return "metrics" }

type AuthInterceptor struct {
	validateToken func(string) (interface{}, error)
	mu            sync.RWMutex
}

func NewAuthInterceptor(validateToken func(string) (interface{}, error)) *AuthInterceptor {
	return &AuthInterceptor{validateToken: validateToken}
}

func (ai *AuthInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	token := req.Headers["Authorization"]
	if token == "" {
		return &Response{
			StatusCode: 401,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"error":"unauthorized"}`),
		}, fmt.Errorf("missing authorization token")
	}

	user, err := ai.validateToken(token)
	if err != nil {
		return &Response{
			StatusCode: 401,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"error":"invalid token"}`),
		}, fmt.Errorf("invalid token: %w", err)
	}

	req.Metadata["user"] = user
	return next(req)
}

func (ai *AuthInterceptor) Name() string { return "auth" }

type RateLimitInterceptor struct {
	limit    int
	current  int
	window   time.Duration
	lastTime time.Time
	mu       sync.RWMutex
}

func NewRateLimitInterceptor(limit int, window time.Duration) *RateLimitInterceptor {
	if limit <= 0 {
		limit = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimitInterceptor{
		limit:    limit,
		window:   window,
		lastTime: time.Now(),
	}
}

func (rli *RateLimitInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	rli.mu.Lock()
	now := time.Now()

	if now.Sub(rli.lastTime) > rli.window {
		rli.current = 0
		rli.lastTime = now
	}

	rli.current++
	if rli.current > rli.limit {
		rli.mu.Unlock()
		return &Response{
			StatusCode: 429,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"error":"rate limit exceeded"}`),
		}, fmt.Errorf("rate limit exceeded")
	}
	rli.mu.Unlock()

	return next(req)
}

func (rli *RateLimitInterceptor) Name() string { return "ratelimit" }

type CacheInterceptor struct {
	cache map[string]*Response
	ttl   time.Duration
	mu    sync.RWMutex
}

func NewCacheInterceptor(ttl time.Duration) *CacheInterceptor {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CacheInterceptor{
		cache: make(map[string]*Response),
		ttl:   ttl,
	}
}

func (ci *CacheInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	if req.Method != "GET" {
		return next(req)
	}

	key := req.Path
	ci.mu.RLock()
	if resp, ok := ci.cache[key]; ok {
		ci.mu.RUnlock()
		return resp, nil
	}
	ci.mu.RUnlock()

	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	ci.mu.Lock()
	ci.cache[key] = resp
	ci.mu.Unlock()

	go func() {
		time.Sleep(ci.ttl)
		ci.mu.Lock()
		delete(ci.cache, key)
		ci.mu.Unlock()
	}()

	return resp, nil
}

func (ci *CacheInterceptor) Name() string { return "cache" }

type TimeoutInterceptor struct {
	timeout time.Duration
	mu      sync.RWMutex
}

func NewTimeoutInterceptor(timeout time.Duration) *TimeoutInterceptor {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &TimeoutInterceptor{timeout: timeout}
}

func (ti *TimeoutInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
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
	case <-time.After(ti.timeout):
		return &Response{
			StatusCode: 408,
			Error:      fmt.Errorf("request timeout"),
		}, fmt.Errorf("request timeout after %v", ti.timeout)
	}
}

func (ti *TimeoutInterceptor) Name() string { return "timeout" }

type RecoveryInterceptor struct {
	handler func(interface{})
	mu      sync.RWMutex
}

func NewRecoveryInterceptor(handler func(interface{})) *RecoveryInterceptor {
	if handler == nil {
		handler = func(r interface{}) {}
	}
	return &RecoveryInterceptor{handler: handler}
}

func (ri *RecoveryInterceptor) Intercept(req *Request, next Handler) (resp *Response, err error) {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	defer func() {
		if r := recover(); r != nil {
			ri.handler(r)
			resp = &Response{
				StatusCode: 500,
				Error:      fmt.Errorf("panic recovered: %v", r),
			}
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return next(req)
}

func (ri *RecoveryInterceptor) Name() string { return "recovery" }

type CORSInterceptor struct {
	allowOrigin  string
	allowMethods string
	allowHeaders string
	mu           sync.RWMutex
}

func NewCORSInterceptor(origin, methods, headers string) *CORSInterceptor {
	if origin == "" {
		origin = "*"
	}
	if methods == "" {
		methods = "GET, POST, PUT, DELETE, OPTIONS"
	}
	if headers == "" {
		headers = "Content-Type, Authorization"
	}
	return &CORSInterceptor{
		allowOrigin:  origin,
		allowMethods: methods,
		allowHeaders: headers,
	}
}

func (ci *CORSInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if req.Method == "OPTIONS" {
		return &Response{
			StatusCode: 200,
			Headers: map[string]string{
				"Access-Control-Allow-Origin":  ci.allowOrigin,
				"Access-Control-Allow-Methods": ci.allowMethods,
				"Access-Control-Allow-Headers": ci.allowHeaders,
			},
		}, nil
	}

	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	resp.Headers["Access-Control-Allow-Origin"] = ci.allowOrigin
	resp.Headers["Access-Control-Allow-Methods"] = ci.allowMethods
	resp.Headers["Access-Control-Allow-Headers"] = ci.allowHeaders

	return resp, nil
}

func (ci *CORSInterceptor) Name() string { return "cors" }
