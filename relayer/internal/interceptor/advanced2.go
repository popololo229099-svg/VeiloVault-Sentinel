package interceptor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type RequestIDInterceptor struct {
	header  string
	mu      sync.RWMutex
}

func NewRequestIDInterceptor(header string) *RequestIDInterceptor {
	if header == "" {
		header = "X-Request-ID"
	}
	return &RequestIDInterceptor{header: header}
}

func (ri *RequestIDInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	ri.mu.RLock()
	header := ri.header
	ri.mu.RUnlock()

	if _, ok := req.Headers[header]; !ok {
		id := make([]byte, 16)
		_, _ = rand.Read(id)
		req.Headers[header] = hex.EncodeToString(id)
	}

	req.Metadata["request_id"] = req.Headers[header]
	return next(req)
}

func (ri *RequestIDInterceptor) Name() string { return "request_id" }

type MethodOverrideInterceptor struct {
	allowedMethods map[string]bool
	mu             sync.RWMutex
}

func NewMethodOverrideInterceptor() *MethodOverrideInterceptor {
	return &MethodOverrideInterceptor{
		allowedMethods: map[string]bool{"POST": true},
	}
}

func (mor *MethodOverrideInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	mor.mu.RLock()
	defer mor.mu.RUnlock()

	if req.Method == "POST" {
		if override := req.Headers["X-HTTP-Method-Override"]; override != "" {
			if mor.allowedMethods[override] {
				req.Method = override
			}
		}
		if override := req.Headers["X-HTTP-Method"]; override != "" {
			if mor.allowedMethods[override] {
				req.Method = override
			}
		}
	}

	return next(req)
}

func (mor *MethodOverrideInterceptor) Name() string { return "method_override" }

type BodyChecksumInterceptor struct {
	mu sync.RWMutex
}

func NewBodyChecksumInterceptor() *BodyChecksumInterceptor {
	return &BodyChecksumInterceptor{}
}

func (bci *BodyChecksumInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	bci.mu.RLock()
	defer bci.mu.RUnlock()

	if req.Body != nil {
		checksum := uint32(0)
		for _, b := range req.Body {
			checksum += uint32(b)
		}
		req.Headers["X-Body-Checksum"] = fmt.Sprintf("%08x", checksum)
	}

	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	if resp != nil && resp.Body != nil {
		checksum := uint32(0)
		for _, b := range resp.Body {
			checksum += uint32(b)
		}
		if resp.Headers == nil {
			resp.Headers = make(map[string]string)
		}
		resp.Headers["X-Response-Checksum"] = fmt.Sprintf("%08x", checksum)
	}

	return resp, err
}

func (bci *BodyChecksumInterceptor) Name() string { return "body_checksum" }

type ContentNegotiationInterceptor struct {
	supportedTypes []string
	mu             sync.RWMutex
}

func NewContentNegotiationInterceptor(types ...string) *ContentNegotiationInterceptor {
	if len(types) == 0 {
		types = []string{"application/json", "text/plain"}
	}
	return &ContentNegotiationInterceptor{supportedTypes: types}
}

func (cni *ContentNegotiationInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	cni.mu.RLock()
	defer cni.mu.RUnlock()

	accept := req.Headers["Accept"]
	if accept == "" {
		req.Headers["Accept"] = cni.supportedTypes[0]
	}

	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	if resp != nil && resp.Headers != nil {
		if ct, ok := resp.Headers["Content-Type"]; ok {
			supported := false
			for _, st := range cni.supportedTypes {
				if ct == st {
					supported = true
					break
				}
			}
			if !supported {
				resp.StatusCode = 406
				resp.Body = []byte(`{"error":"not acceptable"}`)
			}
		}
	}

	return resp, err
}

func (cni *ContentNegotiationInterceptor) Name() string { return "content_negotiation" }

type SecurityHeadersInterceptor struct {
	headers map[string]string
	mu      sync.RWMutex
}

func NewSecurityHeadersInterceptor() *SecurityHeadersInterceptor {
	return &SecurityHeadersInterceptor{
		headers: map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		},
	}
}

func (shi *SecurityHeadersInterceptor) AddHeader(key, value string) {
	shi.mu.Lock()
	defer shi.mu.Unlock()
	shi.headers[key] = value
}

func (shi *SecurityHeadersInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	if resp != nil {
		shi.mu.RLock()
		defer shi.mu.RUnlock()
		if resp.Headers == nil {
			resp.Headers = make(map[string]string)
		}
		for k, v := range shi.headers {
			resp.Headers[k] = v
		}
	}

	return resp, err
}

func (shi *SecurityHeadersInterceptor) Name() string { return "security_headers" }

type RequestTimingInterceptor struct {
	mu sync.RWMutex
}

func NewRequestTimingInterceptor() *RequestTimingInterceptor {
	return &RequestTimingInterceptor{}
}

func (rti *RequestTimingInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	rti.mu.RLock()
	defer rti.mu.RUnlock()

	start := time.Now()
	resp, err := next(req)
	duration := time.Since(start)

	req.Metadata["start_time"] = start
	req.Metadata["duration"] = duration

	if resp != nil {
		if resp.Headers == nil {
			resp.Headers = make(map[string]string)
		}
		resp.Headers["X-Request-Duration"] = duration.String()
		resp.Duration = duration
	}

	return resp, err
}

func (rti *RequestTimingInterceptor) Name() string { return "request_timing" }

type PanicRecoveryInterceptor struct {
	handler func(interface{})
	mu      sync.RWMutex
}

func NewPanicRecoveryInterceptor(handler func(interface{})) *PanicRecoveryInterceptor {
	if handler == nil {
		handler = func(r interface{}) {}
	}
	return &PanicRecoveryInterceptor{handler: handler}
}

func (pri *PanicRecoveryInterceptor) Intercept(req *Request, next Handler) (resp *Response, err error) {
	pri.mu.RLock()
	handler := pri.handler
	pri.mu.RUnlock()

	defer func() {
		if r := recover(); r != nil {
			handler(r)
			resp = &Response{
				StatusCode: 500,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       []byte(`{"error":"internal server error"}`),
			}
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return next(req)
}

func (pri *PanicRecoveryInterceptor) Name() string { return "panic_recovery" }

type IPFilterInterceptor struct {
	allowedIPs map[string]bool
	blockedIPs map[string]bool
	mu         sync.RWMutex
}

func NewIPFilterInterceptor() *IPFilterInterceptor {
	return &IPFilterInterceptor{
		allowedIPs: make(map[string]bool),
		blockedIPs: make(map[string]bool),
	}
}

func (ipfi *IPFilterInterceptor) AllowIP(ip string) {
	ipfi.mu.Lock()
	defer ipfi.mu.Unlock()
	ipfi.allowedIPs[ip] = true
}

func (ipfi *IPFilterInterceptor) BlockIP(ip string) {
	ipfi.mu.Lock()
	defer ipfi.mu.Unlock()
	ipfi.blockedIPs[ip] = true
}

func (ipfi *IPFilterInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	ipfi.mu.RLock()
	defer ipfi.mu.RUnlock()

	clientIP := req.Headers["X-Forwarded-For"]
	if clientIP == "" {
		clientIP = req.Headers["X-Real-IP"]
	}

	if _, blocked := ipfi.blockedIPs[clientIP]; blocked {
		return &Response{
			StatusCode: 403,
			Body:       []byte(`{"error":"forbidden"}`),
		}, fmt.Errorf("IP blocked: %s", clientIP)
	}

	if len(ipfi.allowedIPs) > 0 {
		if _, allowed := ipfi.allowedIPs[clientIP]; !allowed {
			return &Response{
				StatusCode: 403,
				Body:       []byte(`{"error":"forbidden"}`),
			}, fmt.Errorf("IP not in whitelist: %s", clientIP)
		}
	}

	return next(req)
}

func (ipfi *IPFilterInterceptor) Name() string { return "ip_filter" }

type MaxRequestSizeInterceptor struct {
	maxSize int64
	mu      sync.RWMutex
}

func NewMaxRequestSizeInterceptor(maxSize int64) *MaxRequestSizeInterceptor {
	if maxSize <= 0 {
		maxSize = 1024 * 1024
	}
	return &MaxRequestSizeInterceptor{maxSize: maxSize}
}

func (mrsi *MaxRequestSizeInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	mrsi.mu.RLock()
	defer mrsi.mu.RUnlock()

	if req.Body != nil && int64(len(req.Body)) > mrsi.maxSize {
		return &Response{
			StatusCode: 413,
			Body:       []byte(`{"error":"payload too large"}`),
		}, fmt.Errorf("request body exceeds max size %d", mrsi.maxSize)
	}

	return next(req)
}

func (mrsi *MaxRequestSizeInterceptor) Name() string { return "max_request_size" }

type ResponseCachingInterceptor struct {
	cache map[string]*cachedResponse
	ttl   time.Duration
	mu    sync.RWMutex
}

type cachedResponse struct {
	response  *Response
	createdAt time.Time
}

func NewResponseCachingInterceptor(ttl time.Duration) *ResponseCachingInterceptor {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ResponseCachingInterceptor{
		cache: make(map[string]*cachedResponse),
		ttl:   ttl,
	}
}

func (rci *ResponseCachingInterceptor) Intercept(req *Request, next Handler) (*Response, error) {
	if req.Method != "GET" {
		return next(req)
	}

	rci.mu.RLock()
	cached, ok := rci.cache[req.Path]
	if ok && time.Since(cached.createdAt) < rci.ttl {
		rci.mu.RUnlock()
		return cached.response, nil
	}
	rci.mu.RUnlock()

	resp, err := next(req)
	if err != nil {
		return resp, err
	}

	rci.mu.Lock()
	rci.cache[req.Path] = &cachedResponse{
		response:  resp,
		createdAt: time.Now(),
	}
	rci.mu.Unlock()

	return resp, nil
}

func (rci *ResponseCachingInterceptor) Clear() {
	rci.mu.Lock()
	defer rci.mu.Unlock()
	rci.cache = make(map[string]*cachedResponse)
}

func (rci *ResponseCachingInterceptor) Name() string { return "response_caching" }
