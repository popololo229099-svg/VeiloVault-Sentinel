package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	server          *http.Server
	config          ServerConfig
	handler         http.Handler
	connState       map[net.Conn]http.ConnState
	connStateMu     sync.RWMutex
	activeConns     int64
	totalConns      int64
	totalRequests   int64
	shutdownCh      chan struct{}
	ready           atomic.Bool
	hooks           *ServerHooks
	transport       *http.Transport
	mu              sync.RWMutex
}

type ServerConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
	TLSConfig         *tls.Config
	EnableTLS         bool
	CertFile          string
	KeyFile           string
	MaxConns          int64
	GracefulTimeout   time.Duration
	EnableShutdown    bool
	KeepAlive         time.Duration
	MaxIdleConns      int
	MaxIdleConnsPerHost int
	DialTimeout       time.Duration
	DisableKeepAlives  bool
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:                ":8080",
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        15 * time.Second,
		IdleTimeout:         60 * time.Second,
		ReadHeaderTimeout:   10 * time.Second,
		MaxHeaderBytes:      1 << 20,
		GracefulTimeout:     30 * time.Second,
		EnableShutdown:      true,
		KeepAlive:           30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		DialTimeout:         10 * time.Second,
	}
}

type ServerHooks struct {
	OnStart      func(*Server)
	OnShutdown   func(*Server)
	OnRequest    func(*http.Request)
	OnResponse   func(http.ResponseWriter, *http.Request)
	OnConnNew    func(net.Conn)
	OnConnClose  func(net.Conn, http.ConnState)
OnError       func(error)
	mu           sync.RWMutex
}

func NewServer(config ServerConfig, handler http.Handler) *Server {
	s := &Server{
		config:    config,
		handler:   handler,
		connState: make(map[net.Conn]http.ConnState),
		shutdownCh: make(chan struct{}),
		hooks:     &ServerHooks{},
	}

	tlsConfig := config.TLSConfig
	if tlsConfig == nil && config.EnableTLS {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		IdleConnTimeout:     config.IdleTimeout,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		DisableKeepAlives:  config.DisableKeepAlives,
		ForceAttemptHTTP2:  true,
	}

	s.server = &http.Server{
		Addr:              config.Addr,
		Handler:           handler,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		TLSConfig:         tlsConfig,
		ConnState:         s.trackConnection,
	}
	s.transport = transport

	return s
}

func (s *Server) trackConnection(conn net.Conn, state http.ConnState) {
	s.connStateMu.Lock()
	defer s.connStateMu.Unlock()

	switch state {
	case http.StateNew:
		atomic.AddInt64(&s.totalConns, 1)
		atomic.AddInt64(&s.activeConns, 1)
		s.connState[conn] = state
		if s.hooks != nil {
			s.hooks.mu.RLock()
			if s.hooks.OnConnNew != nil {
				s.hooks.OnConnNew(conn)
			}
			s.hooks.mu.RUnlock()
		}
	case http.StateActive:
		s.connState[conn] = state
	case http.StateIdle, http.StateHijacked, http.StateClosed:
		atomic.AddInt64(&s.activeConns, -1)
		delete(s.connState, conn)
		if s.hooks != nil {
			s.hooks.mu.RLock()
			if s.hooks.OnConnClose != nil {
				s.hooks.OnConnClose(conn, state)
			}
			s.hooks.mu.RUnlock()
		}
	}
}

func (s *Server) Start() error {
	s.ready.Store(true)

	if s.hooks != nil {
		s.hooks.mu.RLock()
		if s.hooks.OnStart != nil {
			s.hooks.OnStart(s)
		}
		s.hooks.mu.RUnlock()
	}

	var err error
	if s.config.EnableTLS {
		err = s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	} else {
		err = s.server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (s *Server) StartAsync() error {
	s.ready.Store(true)

	if s.hooks != nil {
		s.hooks.mu.RLock()
		if s.hooks.OnStart != nil {
			s.hooks.OnStart(s)
		}
		s.hooks.mu.RUnlock()
	}

	go func() {
		var err error
		if s.config.EnableTLS {
			err = s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			if s.hooks != nil {
				s.hooks.mu.RLock()
				if s.hooks.OnError != nil {
					s.hooks.OnError(err)
				}
				s.hooks.mu.RUnlock()
			}
		}
	}()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.GracefulTimeout)
	defer cancel()

	if s.hooks != nil {
		s.hooks.mu.RLock()
		if s.hooks.OnShutdown != nil {
			s.hooks.OnShutdown(s)
		}
		s.hooks.mu.RUnlock()
	}

	err := s.server.Shutdown(shutdownCtx)
	s.ready.Store(false)
	close(s.shutdownCh)
	return err
}

func (s *Server) Close() error {
	s.ready.Store(false)
	return s.server.Close()
}

func (s *Server) IsReady() bool {
	return s.ready.Load()
}

func (s *Server) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConns)
}

func (s *Server) TotalConnections() int64 {
	return atomic.LoadInt64(&s.totalConns)
}

func (s *Server) TotalRequests() int64 {
	return atomic.LoadInt64(&s.totalRequests)
}

func (s *Server) SetHandler(handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
	s.server.Handler = handler
}

func (s *Server) SetHooks(hooks *ServerHooks) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = hooks
}

func (s *Server) OnRequest(fn func(*http.Request)) {
	s.hooks.mu.Lock()
	defer s.hooks.mu.Unlock()
	s.hooks.OnRequest = fn
}

func (s *Server) OnResponse(fn func(http.ResponseWriter, *http.Request)) {
	s.hooks.mu.Lock()
	defer s.hooks.mu.Unlock()
	s.hooks.OnResponse = fn
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	atomic.AddInt64(&s.totalRequests, 1)
	s.handler.ServeHTTP(w, req)
}

func (s *Server) Addr() string {
	return s.config.Addr
}

func (s *Server) Stats() ServerStats {
	return ServerStats{
		ActiveConnections: atomic.LoadInt64(&s.activeConns),
		TotalConnections:  atomic.LoadInt64(&s.totalConns),
		TotalRequests:     atomic.LoadInt64(&s.totalRequests),
		IsReady:           s.ready.Load(),
	}
}

type ServerStats struct {
	ActiveConnections int64
	TotalConnections  int64
	TotalRequests     int64
	IsReady           bool
}

type RequestWrapper struct {
	*http.Request
	RequestID     string
	StartTime     time.Time
	RemoteAddr    string
	ExtraHeaders  map[string]string
	mu            sync.RWMutex
}

func NewRequestWrapper(req *http.Request) *RequestWrapper {
	return &RequestWrapper{
		Request:      req,
		RequestID:    generateID(),
		StartTime:    time.Now(),
		RemoteAddr:   req.RemoteAddr,
		ExtraHeaders: make(map[string]string),
	}
}

func (rw *RequestWrapper) SetHeader(key, value string) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.ExtraHeaders[key] = value
	rw.Request.Header.Set(key, value)
}

func (rw *RequestWrapper) GetHeader(key string) string {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	if val, exists := rw.ExtraHeaders[key]; exists {
		return val
	}
	return rw.Request.Header.Get(key)
}

func (rw *RequestWrapper) Duration() time.Duration {
	return time.Since(rw.StartTime)
}

type ResponseWrapper struct {
	http.ResponseWriter
	statusCode    int
	written       int64
	wroteHeader   bool
	header        http.Header
	flushed       bool
	closed        bool
	onClose       func()
	mu            sync.RWMutex
}

func NewResponseWrapper(w http.ResponseWriter) *ResponseWrapper {
	return &ResponseWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		header:         w.Header(),
	}
}

func (rw *ResponseWrapper) WriteHeader(code int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWrapper) Write(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func (rw *ResponseWrapper) StatusCode() int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.statusCode
}

func (rw *ResponseWrapper) Written() int64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.written
}

func (rw *ResponseWrapper) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
		rw.mu.Lock()
		rw.flushed = true
		rw.mu.Unlock()
	}
}

func (rw *ResponseWrapper) Close() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if !rw.closed {
		rw.closed = true
		if rw.onClose != nil {
			rw.onClose()
		}
	}
}

func (rw *ResponseWrapper) SetOnClose(fn func()) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.onClose = fn
}

func (rw *ResponseWrapper) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

type StreamingResponse struct {
	writer    http.ResponseWriter
	flusher   http.Flusher
	mu        sync.Mutex
	finished  bool
	bytesSent int64
}

func NewStreamingResponse(w http.ResponseWriter) *StreamingResponse {
	flusher, _ := w.(http.Flusher)
	return &StreamingResponse{
		writer:  w,
		flusher: flusher,
	}
}

func (sr *StreamingResponse) Write(data []byte) (int, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.finished {
		return 0, fmt.Errorf("stream closed")
	}

	n, err := sr.writer.Write(data)
	sr.bytesSent += int64(n)

	if sr.flusher != nil {
		sr.flusher.Flush()
	}

	return n, err
}

func (sr *StreamingResponse) WriteString(s string) (int, error) {
	return sr.Write([]byte(s))
}

func (sr *StreamingResponse) Close() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.finished = true
	return nil
}

func (sr *StreamingResponse) BytesSent() int64 {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.bytesSent
}

type SSEServer struct {
	clients    map[string]*SSEClient
	broadcast  chan []byte
	register   chan *SSEClient
	unregister chan *SSEClient
	mu         sync.RWMutex
}

type SSEClient struct {
	ID     string
	Events chan SSEEvent
	Done   chan struct{}
	mu     sync.RWMutex
}

type SSEEvent struct {
	ID    string
	Event string
	Data  string
	Retry int
}

func NewSSEServer() *SSEServer {
	return &SSEServer{
		clients:    make(map[string]*SSEClient),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *SSEClient),
		unregister: make(chan *SSEClient),
	}
}

func (ss *SSEServer) Register(id string) *SSEClient {
	client := &SSEClient{
		ID:     id,
		Events: make(chan SSEEvent, 64),
		Done:   make(chan struct{}),
	}
	ss.register <- client
	return client
}

func (ss *SSEServer) Unregister(id string) {
	ss.mu.RLock()
	client, exists := ss.clients[id]
	ss.mu.RUnlock()

	if exists {
		ss.unregister <- client
	}
}

func (ss *SSEServer) Send(id string, event SSEEvent) {
	ss.mu.RLock()
	client, exists := ss.clients[id]
	ss.mu.RUnlock()

	if exists {
		select {
		case client.Events <- event:
		default:
		}
	}
}

func (ss *SSEServer) Broadcast(event SSEEvent) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	for _, client := range ss.clients {
		select {
		case client.Events <- event:
		default:
		}
	}
}

func (ss *SSEServer) Run() {
	for {
		select {
		case client := <-ss.register:
			ss.mu.Lock()
			ss.clients[client.ID] = client
			ss.mu.Unlock()

		case client := <-ss.unregister:
			ss.mu.Lock()
			if _, ok := ss.clients[client.ID]; ok {
				delete(ss.clients, client.ID)
				close(client.Events)
			}
			ss.mu.Unlock()

		case <-ss.broadcast:
			// handled via Broadcast method
		}
	}
}

func (ss *SSEServer) ClientCount() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.clients)
}

func (ss *SSEServer) HandleSSE(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	clientID := req.URL.Query().Get("id")
	if clientID == "" {
		clientID = generateID()
	}

	client := ss.Register(clientID)
	defer ss.Unregister(clientID)

	ctx := req.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-client.Events:
			if !ok {
				return
			}
			if event.ID != "" {
				fmt.Fprintf(w, "id: %s\n", event.ID)
			}
			if event.Event != "" {
				fmt.Fprintf(w, "event: %s\n", event.Event)
			}
			if event.Retry > 0 {
				fmt.Fprintf(w, "retry: %d\n", event.Retry)
			}
			fmt.Fprintf(w, "data: %s\n\n", event.Data)
			flusher.Flush()
		}
	}
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

type ConnectionPool struct {
	maxConns      int
	activeConns   int64
	createFunc    func() (net.Conn, error)
	conns         chan net.Conn
	mu            sync.RWMutex
	closed        bool
	totalCreated  int64
	totalReused   int64
	totalFailed   int64
}

type ConnectionPoolConfig struct {
	MaxConns       int
	CreateFunc     func() (net.Conn, error)
	MaxConnAge     time.Duration
	IdleTimeout    time.Duration
	HealthCheck    func(net.Conn) bool
}

func NewConnectionPool(config ConnectionPoolConfig) *ConnectionPool {
	if config.MaxConns <= 0 {
		config.MaxConns = 10
	}

	cp := &ConnectionPool{
		maxConns:   config.MaxConns,
		createFunc: config.CreateFunc,
		conns:      make(chan net.Conn, config.MaxConns),
	}

	return cp
}

func (cp *ConnectionPool) Get() (net.Conn, error) {
	cp.mu.RLock()
	if cp.closed {
		cp.mu.RUnlock()
		return nil, fmt.Errorf("pool closed")
	}
	cp.mu.RUnlock()

	select {
	case conn := <-cp.conns:
		atomic.AddInt64(&cp.totalReused, 1)
		return conn, nil
	default:
	}

	current := atomic.LoadInt64(&cp.activeConns)
	if current >= int64(cp.maxConns) {
		conn := <-cp.conns
		return conn, nil
	}

	conn, err := cp.createFunc()
	if err != nil {
		atomic.AddInt64(&cp.totalFailed, 1)
		return nil, err
	}

	atomic.AddInt64(&cp.activeConns, 1)
	atomic.AddInt64(&cp.totalCreated, 1)
	return conn, nil
}

func (cp *ConnectionPool) Put(conn net.Conn) {
	cp.mu.RLock()
	closed := cp.closed
	cp.mu.RUnlock()

	if closed {
		conn.Close()
		return
	}

	select {
	case cp.conns <- conn:
	default:
		conn.Close()
		atomic.AddInt64(&cp.activeConns, -1)
	}
}

func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.closed = true
	close(cp.conns)

	for {
		select {
		case conn := <-cp.conns:
			conn.Close()
		default:
			return
		}
	}
}

func (cp *ConnectionPool) Stats() ConnectionPoolStats {
	return ConnectionPoolStats{
		MaxConns:     cp.maxConns,
		ActiveConns:  atomic.LoadInt64(&cp.activeConns),
		TotalCreated: atomic.LoadInt64(&cp.totalCreated),
		TotalReused:  atomic.LoadInt64(&cp.totalReused),
		TotalFailed:  atomic.LoadInt64(&cp.totalFailed),
	}
}

type ConnectionPoolStats struct {
	MaxConns     int
	ActiveConns  int64
	TotalCreated int64
	TotalReused  int64
	TotalFailed  int64
}

type TLSConfigBuilder struct {
	minVersion    uint16
	certFile      string
	keyFile       string
	clientAuth    tls.ClientAuthType
	cipherSuites  []uint16
	preferServer  bool
	sessionCache  tls.ClientSessionCache
	mu            sync.RWMutex
}

func NewTLSConfigBuilder() *TLSConfigBuilder {
	return &TLSConfigBuilder{
		minVersion:   tls.VersionTLS12,
		preferServer: true,
	}
}

func (tcb *TLSConfigBuilder) SetMinVersion(version uint16) *TLSConfigBuilder {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.minVersion = version
	return tcb
}

func (tcb *TLSConfigBuilder) SetCertFile(certFile string) *TLSConfigBuilder {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.certFile = certFile
	return tcb
}

func (tcb *TLSConfigBuilder) SetKeyFile(keyFile string) *TLSConfigBuilder {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.keyFile = keyFile
	return tcb
}

func (tcb *TLSConfigBuilder) SetClientAuth(auth tls.ClientAuthType) *TLSConfigBuilder {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.clientAuth = auth
	return tcb
}

func (tcb *TLSConfigBuilder) SetCipherSuites(suites []uint16) *TLSConfigBuilder {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.cipherSuites = make([]uint16, len(suites))
	copy(tcb.cipherSuites, suites)
	return tcb
}

func (tcb *TLSConfigBuilder) Build() *tls.Config {
	tcb.mu.RLock()
	defer tcb.mu.RUnlock()

	config := &tls.Config{
		MinVersion:               tcb.minVersion,
		PreferServerCipherSuites: tcb.preferServer,
		ClientAuth:               tcb.clientAuth,
	}

	if len(tcb.cipherSuites) > 0 {
		config.CipherSuites = make([]uint16, len(tcb.cipherSuites))
		copy(config.CipherSuites, tcb.cipherSuites)
	}

	return config
}

type HealthChecker struct {
	checks    map[string]HealthCheckFunc
	results   map[string]HealthCheckResult
	interval  time.Duration
	stopCh    chan struct{}
	mu        sync.RWMutex
}

type HealthCheckFunc func(ctx context.Context) error

type HealthCheckResult struct {
	Name      string
	Status    string
	Error     string
	Duration  time.Duration
	Timestamp time.Time
}

type HealthStatus struct {
	Status  string
	Checks  map[string]HealthCheckResult
	mu      sync.RWMutex
}

func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		checks:   make(map[string]HealthCheckFunc),
		results:  make(map[string]HealthCheckResult),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (hc *HealthChecker) Register(name string, check HealthCheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

func (hc *HealthChecker) Unregister(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.checks, name)
	delete(hc.results, name)
}

func (hc *HealthChecker) Check(ctx context.Context) *HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := &HealthStatus{
		Status: "healthy",
		Checks: make(map[string]HealthCheckResult),
	}

	for name, check := range hc.checks {
		start := time.Now()
		err := check(ctx)
		duration := time.Since(start)

		result := HealthCheckResult{
			Name:      name,
			Duration:  duration,
			Timestamp: time.Now(),
		}

		if err != nil {
			result.Status = "unhealthy"
			result.Error = err.Error()
			status.Status = "unhealthy"
		} else {
			result.Status = "healthy"
		}

		status.Checks[name] = result
	}

	return status
}

func (hc *HealthChecker) Start() {
	go func() {
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				hc.Check(ctx)
			case <-hc.stopCh:
				return
			}
		}
	}()
}

func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

func (hc *HealthChecker) GetResults() map[string]HealthCheckResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	results := make(map[string]HealthCheckResult)
	for k, v := range hc.results {
		results[k] = v
	}
	return results
}

type MiddlewareHandler struct {
	handler     http.Handler
	middlewares []func(http.Handler) http.Handler
	mu          sync.RWMutex
}

func NewMiddlewareHandler(handler http.Handler) *MiddlewareHandler {
	return &MiddlewareHandler{
		handler:     handler,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

func (mh *MiddlewareHandler) Use(middleware func(http.Handler) http.Handler) {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	mh.middlewares = append(mh.middlewares, middleware)
}

func (mh *MiddlewareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mh.mu.RLock()
	handler := mh.handler
	for i := len(mh.middlewares) - 1; i >= 0; i-- {
		handler = mh.middlewares[i](handler)
	}
	mh.mu.RUnlock()

	handler.ServeHTTP(w, r)
}

func (mh *MiddlewareHandler) Count() int {
	mh.mu.RLock()
	defer mh.mu.RUnlock()
	return len(mh.middlewares)
}

type RequestLogger struct {
	format    string
	output    io.Writer
	level     string
	mu        sync.RWMutex
}

type RequestLoggerConfig struct {
	Format string
	Output io.Writer
	Level  string
}

func NewRequestLogger(config RequestLoggerConfig) *RequestLogger {
	if config.Format == "" {
		config.Format = "%s %s %d %s"
	}
	if config.Output == nil {
		config.Output = io.Discard
	}
	if config.Level == "" {
		config.Level = "info"
	}
	return &RequestLogger{
		format: config.Format,
		output: config.Output,
		level:  config.Level,
	}
}

func (rl *RequestLogger) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			rw := NewResponseWrapper(w)

			next.ServeHTTP(rw, req)

			duration := time.Since(start)
			rl.mu.RLock()
			defer rl.mu.RUnlock()

			statusCode := rw.StatusCode()
			logLine := fmt.Sprintf(rl.format,
				req.Method,
				req.URL.Path,
				statusCode,
				duration.String(),
			)

			fmt.Fprintln(rl.output, logLine)
		})
	}
}

func (rl *RequestLogger) SetLevel(level string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.level = level
}

type BodyLimitMiddleware struct {
	maxBytes int64
	message  string
	mu       sync.RWMutex
}

func NewBodyLimitMiddleware(maxBytes int64) *BodyLimitMiddleware {
	return &BodyLimitMiddleware{
		maxBytes: maxBytes,
		message:  "Request body too large",
	}
}

func (blm *BodyLimitMiddleware) SetMessage(msg string) {
	blm.mu.Lock()
	defer blm.mu.Unlock()
	blm.message = msg
}

func (blm *BodyLimitMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			blm.mu.RLock()
			maxBytes := blm.maxBytes
			message := blm.message
			blm.mu.RUnlock()

			req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
			if err := req.ParseForm(); err != nil {
				if err.Error() == "http: request body too large" {
					http.Error(w, message, http.StatusRequestEntityTooLarge)
					return
				}
			}

			next.ServeHTTP(w, req)
		})
	}
}

type SecureHeadersMiddleware struct {
	headers map[string]string
	mu      sync.RWMutex
}

func NewSecureHeadersMiddleware() *SecureHeadersMiddleware {
	return &SecureHeadersMiddleware{
		headers: map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
			"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
			"Content-Security-Policy": "default-src 'self'",
		},
	}
}

func (shm *SecureHeadersMiddleware) SetHeader(key, value string) {
	shm.mu.Lock()
	defer shm.mu.Unlock()
	shm.headers[key] = value
}

func (shm *SecureHeadersMiddleware) RemoveHeader(key string) {
	shm.mu.Lock()
	defer shm.mu.Unlock()
	delete(shm.headers, key)
}

func (shm *SecureHeadersMiddleware) Middleware() func(http.Handler) http.Handler {
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

type ChainBuilder struct {
	handler    http.Handler
	middleware []func(http.Handler) http.Handler
	mu         sync.RWMutex
}

func NewChainBuilder(handler http.Handler) *ChainBuilder {
	return &ChainBuilder{
		handler:    handler,
		middleware: make([]func(http.Handler) http.Handler, 0),
	}
}

func (cb *ChainBuilder) Use(mws ...func(http.Handler) http.Handler) *ChainBuilder {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.middleware = append(cb.middleware, mws...)
	return cb
}

func (cb *ChainBuilder) Build() http.Handler {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	handler := cb.handler
	for i := len(cb.middleware) - 1; i >= 0; i-- {
		handler = cb.middleware[i](handler)
	}
	return handler
}

func (cb *ChainBuilder) Count() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.middleware)
}

type GzipMiddleware struct {
	level int
	types []string
	mu    sync.RWMutex
}

func NewGzipMiddleware(types ...string) *GzipMiddleware {
	if len(types) == 0 {
		types = []string{"text/plain", "text/html", "application/json", "text/css", "application/javascript"}
	}
	return &GzipMiddleware{
		level: -1,
		types: types,
	}
}

func (gm *GzipMiddleware) SetLevel(level int) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.level = level
}

func (gm *GzipMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, req)
				return
			}

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")

			gz := &gzipResponseWriter{ResponseWriter: w}
			defer gz.Close()

			next.ServeHTTP(gz, req)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
}

func (grw *gzipResponseWriter) Close() error {
	if closer, ok := grw.ResponseWriter.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (grw *gzipResponseWriter) Flush() {
	if f, ok := grw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
