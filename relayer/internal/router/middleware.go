package router

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type MiddlewareChain struct {
	middlewares []Middleware
	mu         sync.RWMutex
}

func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
	}
}

func (mc *MiddlewareChain) Use(middlewares ...Middleware) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.middlewares = append(mc.middlewares, middlewares...)
}

func (mc *MiddlewareChain) Then(handler http.HandlerFunc) http.HandlerFunc {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var h http.Handler = handler
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		h = mc.middlewares[i](h)
	}
	return func(w http.ResponseWriter, req *http.Request) {
		h.ServeHTTP(w, req)
	}
}

func (mc *MiddlewareChain) Count() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.middlewares)
}

func (mc *MiddlewareChain) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.middlewares = make([]Middleware, 0)
}

type ResponseWriterWrapper struct {
	http.ResponseWriter
	statusCode  int
	written     int64
	wroteHeader bool
	header      http.Header
}

func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		header:         w.Header(),
	}
}

func (rw *ResponseWriterWrapper) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWriterWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func (rw *ResponseWriterWrapper) StatusCode() int {
	return rw.statusCode
}

func (rw *ResponseWriterWrapper) Written() int64 {
	return rw.written
}

func (rw *ResponseWriterWrapper) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

type LoggerMiddleware struct {
	loggerFunc func(string, ...interface{})
	format     string
	mu         sync.RWMutex
}

func NewLoggerMiddleware(loggerFunc func(string, ...interface{})) *LoggerMiddleware {
	return &LoggerMiddleware{
		loggerFunc: loggerFunc,
		format:     "%s %s %d %s",
	}
}

func (lm *LoggerMiddleware) SetFormat(format string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.format = format
}

func (lm *LoggerMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			wrapper := NewResponseWriterWrapper(w)

			next.ServeHTTP(wrapper, req)

			duration := time.Since(start)
			lm.mu.RLock()
			defer lm.mu.RUnlock()

			if lm.loggerFunc != nil {
				lm.loggerFunc(lm.format,
					req.Method,
					req.URL.Path,
					wrapper.StatusCode(),
					duration.String(),
				)
			}
		})
	}
}

type RecoveryMiddleware struct {
	loggerFunc   func(string, ...interface{})
	handlerFunc  func(http.ResponseWriter, *http.Request, interface{})
	printStack   bool
	mu           sync.RWMutex
}

func NewRecoveryMiddleware() *RecoveryMiddleware {
	return &RecoveryMiddleware{
		printStack: false,
	}
}

func (rm *RecoveryMiddleware) SetLogger(loggerFunc func(string, ...interface{})) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.loggerFunc = loggerFunc
}

func (rm *RecoveryMiddleware) SetErrorHandler(handlerFunc func(http.ResponseWriter, *http.Request, interface{})) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.handlerFunc = handlerFunc
}

func (rm *RecoveryMiddleware) SetPrintStack(print bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.printStack = print
}

func (rm *RecoveryMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					rm.mu.RLock()
					if rm.loggerFunc != nil {
						rm.loggerFunc("panic recovered: %v", err)
					}
					handler := rm.handlerFunc
					rm.mu.RUnlock()

					if handler != nil {
						handler(w, req, err)
					} else {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(w, req)
		})
	}
}

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
		MaxAge:       12 * time.Hour,
	}
}

type CORSMiddleware struct {
	config CORSConfig
	mu     sync.RWMutex
}

func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{config: config}
}

func (cm *CORSMiddleware) SetConfig(config CORSConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config = config
}

func (cm *CORSMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			cm.mu.RLock()
			config := cm.config
			cm.mu.RUnlock()

			origin := req.Header.Get("Origin")
			allowed := false

			for _, o := range config.AllowOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
				if strings.Contains(o, "*") {
					pattern := strings.ReplaceAll(o, ".", "\\.")
					pattern = strings.ReplaceAll(pattern, "*", ".*")
					if matched, _ := regexp.MatchString(pattern, origin); matched {
						allowed = true
						break
					}
				}
			}

			if allowed {
				if len(config.AllowOrigins) == 1 && config.AllowOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}

			if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
				if allowed {
					methods := strings.Join(config.AllowMethods, ", ")
					headers := strings.Join(config.AllowHeaders, ", ")
					w.Header().Set("Access-Control-Allow-Methods", methods)
					w.Header().Set("Access-Control-Allow-Headers", headers)
					if config.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
					if config.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age",
							fmt.Sprintf("%d", int(config.MaxAge.Seconds())))
					}
					if len(config.ExposeHeaders) > 0 {
						w.Header().Set("Access-Control-Expose-Headers",
							strings.Join(config.ExposeHeaders, ", "))
					}
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

type ContentTypeMiddleware struct {
	defaultType string
	mu          sync.RWMutex
}

func NewContentTypeMiddleware(defaultType string) *ContentTypeMiddleware {
	if defaultType == "" {
		defaultType = "application/json"
	}
	return &ContentTypeMiddleware{defaultType: defaultType}
}

func (ctm *ContentTypeMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctm.mu.RLock()
			defaultType := ctm.defaultType
			ctm.mu.RUnlock()

			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", defaultType)
			}
			next.ServeHTTP(w, req)
		})
	}
}

type CompressMiddleware struct {
	level int
	types []string
	mu    sync.RWMutex
}

func NewCompressMiddleware(types ...string) *CompressMiddleware {
	if len(types) == 0 {
		types = []string{"text/plain", "text/html", "application/json", "application/javascript"}
	}
	return &CompressMiddleware{
		level: gzip.DefaultCompression,
		types: types,
	}
}

func (cm *CompressMiddleware) SetLevel(level int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.level = level
}

func (cm *CompressMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, req)
				return
			}

			cm.mu.RLock()
			types := cm.types
			cm.mu.RUnlock()

			contentType := w.Header().Get("Content-Type")
			supported := false
			for _, t := range types {
				if strings.Contains(contentType, t) {
					supported = true
					break
				}
			}

			if !supported {
				next.ServeHTTP(w, req)
				return
			}

			gz, err := gzip.NewWriterLevel(w, gzip.DefaultCompression)
			if err != nil {
				next.ServeHTTP(w, req)
				return
			}
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")

			cw := &compressResponseWriter{ResponseWriter: w, writer: gz}
			next.ServeHTTP(cw, req)
		})
	}
}

type compressResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (cw *compressResponseWriter) Write(b []byte) (int, error) {
	return cw.writer.Write(b)
}

type RequestIDMiddleware struct {
	header  string
	generator func() string
	mu      sync.RWMutex
}

func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{
		header: "X-Request-ID",
		generator: func() string {
			return generateRequestID()
		},
	}
}

func (rim *RequestIDMiddleware) SetHeader(header string) {
	rim.mu.Lock()
	defer rim.mu.Unlock()
	rim.header = header
}

func (rim *RequestIDMiddleware) SetGenerator(gen func() string) {
	rim.mu.Lock()
	defer rim.mu.Unlock()
	rim.generator = gen
}

func (rim *RequestIDMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			rim.mu.RLock()
			header := rim.header
			generator := rim.generator
			rim.mu.RUnlock()

			requestID := req.Header.Get(header)
			if requestID == "" {
				requestID = generator()
			}

			w.Header().Set(header, requestID)
			req.Header.Set(header, requestID)
			next.ServeHTTP(w, req)
		})
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8))
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
		time.Sleep(1)
	}
	return string(b)
}

type SecurityHeadersMiddleware struct {
	headers map[string]string
	mu      sync.RWMutex
}

func NewSecurityHeadersMiddleware() *SecurityHeadersMiddleware {
	return &SecurityHeadersMiddleware{
		headers: map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
			"Referrer-Policy":        "strict-origin-when-cross-origin",
		},
	}
}

func (shm *SecurityHeadersMiddleware) SetHeader(key, value string) {
	shm.mu.Lock()
	defer shm.mu.Unlock()
	shm.headers[key] = value
}

func (shm *SecurityHeadersMiddleware) RemoveHeader(key string) {
	shm.mu.Lock()
	defer shm.mu.Unlock()
	delete(shm.headers, key)
}

func (shm *SecurityHeadersMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			shm.mu.RLock()
			defer shm.mu.RUnlock()
			for key, value := range shm.headers {
				w.Header().Set(key, value)
			}
			next.ServeHTTP(w, req)
		})
	}
}

type TimeoutMiddleware struct {
	timeout time.Duration
	message string
	mu      sync.RWMutex
}

func NewTimeoutMiddleware(timeout time.Duration) *TimeoutMiddleware {
	return &TimeoutMiddleware{
		timeout: timeout,
		message: "Request Timeout",
	}
}

func (tm *TimeoutMiddleware) SetMessage(message string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.message = message
}

func (tm *TimeoutMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			tm.mu.RLock()
			timeout := tm.timeout
			message := tm.message
			tm.mu.RUnlock()

			done := make(chan struct{})

			go func() {
				next.ServeHTTP(w, req)
				close(done)
			}()

			select {
			case <-done:
				return
			case <-time.After(timeout):
				http.Error(w, message, http.StatusRequestTimeout)
			}
		})
	}
}

type IPFilterMiddleware struct {
	allowedIPs []string
	blockedIPs []string
	allowAll   bool
	mu         sync.RWMutex
}

func NewIPFilterMiddleware() *IPFilterMiddleware {
	return &IPFilterMiddleware{
		allowedIPs: make([]string, 0),
		blockedIPs: make([]string, 0),
		allowAll:   true,
	}
}

func (ipf *IPFilterMiddleware) AllowIP(ip string) {
	ipf.mu.Lock()
	defer ipf.mu.Unlock()
	ipf.allowedIPs = append(ipf.allowedIPs, ip)
	ipf.allowAll = false
}

func (ipf *IPFilterMiddleware) BlockIP(ip string) {
	ipf.mu.Lock()
	defer ipf.mu.Unlock()
	ipf.blockedIPs = append(ipf.blockedIPs, ip)
}

func (ipf *IPFilterMiddleware) SetAllowAll(allow bool) {
	ipf.mu.Lock()
	defer ipf.mu.Unlock()
	ipf.allowAll = allow
}

func (ipf *IPFilterMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ipf.mu.RLock()
			defer ipf.mu.RUnlock()

			clientIP := extractIP(req)

			for _, blocked := range ipf.blockedIPs {
				if matchIP(clientIP, blocked) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			if !ipf.allowAll {
				allowed := false
				for _, a := range ipf.allowedIPs {
					if matchIP(clientIP, a) {
						allowed = true
						break
					}
				}
				if !allowed {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, req)
		})
	}
}

func extractIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return req.RemoteAddr
}

func matchIP(ip, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		regex := strings.ReplaceAll(pattern, ".", "\\.")
		regex = strings.ReplaceAll(regex, "*", ".*")
		matched, _ := regexp.MatchString("^"+regex+"$", ip)
		return matched
	}
	return ip == pattern
}

type SortMiddleware struct {
	key  func(http.Request) string
	mu   sync.RWMutex
}

func NewSortMiddleware(keyFunc func(http.Request) string) *SortMiddleware {
	return &SortMiddleware{key: keyFunc}
}

func (sm *SortMiddleware) Handler() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)
		})
	}
}

func SortStrings(strs []string) {
	sort.Strings(strs)
}
