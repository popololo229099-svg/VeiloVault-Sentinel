package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileLogger struct {
	file     *os.File
	config   FileLoggerConfig
	mu       sync.Mutex
	size     int64
	rotateCh chan struct{}
}

type FileLoggerConfig struct {
	FilePath      string
	MaxSize       int64
	MaxAge        time.Duration
	MaxBackups    int
	Compress      bool
	BufferSize    int
	FlushInterval time.Duration
}

func DefaultFileLoggerConfig() FileLoggerConfig {
	return FileLoggerConfig{
		FilePath:      "app.log",
		MaxSize:       100 * 1024 * 1024,
		MaxAge:        30 * 24 * time.Hour,
		MaxBackups:    5,
		Compress:      false,
		BufferSize:    4096,
		FlushInterval: time.Second,
	}
}

func NewFileLogger(config FileLoggerConfig) (*FileLogger, error) {
	if config.FilePath == "" {
		config.FilePath = "app.log"
	}
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024
	}
	if config.BufferSize == 0 {
		config.BufferSize = 4096
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = time.Second
	}

	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	fl := &FileLogger{
		file:     f,
		config:   config,
		rotateCh: make(chan struct{}, 1),
	}

	info, err := f.Stat()
	if err == nil {
		fl.size = info.Size()
	}

	go fl.flushLoop()

	return fl, nil
}

func (fl *FileLogger) Write(p []byte) (int, error) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.size+int64(len(p)) > fl.config.MaxSize {
		if err := fl.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := fl.file.Write(p)
	fl.size += int64(n)
	return n, err
}

func (fl *FileLogger) rotate() error {
	fl.file.Close()

	ext := filepath.Ext(fl.file.Name())
	prefix := fl.file.Name()[:len(fl.file.Name())-len(ext)]

	for i := fl.config.MaxBackups; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d%s", prefix, i, ext)
		dst := fmt.Sprintf("%s.%d%s", prefix, i+1, ext)
		os.Rename(src, dst)
	}

	dst := fmt.Sprintf("%s.1%s", prefix, ext)
	os.Rename(fl.file.Name(), dst)

	f, err := os.OpenFile(fl.file.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	fl.file = f
	fl.size = 0

	fl.cleanOldFiles(prefix, ext)
	return nil
}

func (fl *FileLogger) cleanOldFiles(prefix, ext string) {
	if fl.config.MaxAge == 0 {
		return
	}

	dir := filepath.Dir(prefix)
	cutoff := time.Now().Add(-fl.config.MaxAge)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ext {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func (fl *FileLogger) flushLoop() {
	ticker := time.NewTicker(fl.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fl.mu.Lock()
			if fl.file != nil {
				fl.file.Sync()
			}
			fl.mu.Unlock()
		case <-fl.rotateCh:
			return
		}
	}
}

func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	close(fl.rotateCh)
	if fl.file != nil {
		return fl.file.Close()
	}
	return nil
}

func (fl *FileLogger) Sync() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.file != nil {
		return fl.file.Sync()
	}
	return nil
}

type BufferedLogger struct {
	output io.Writer
	buffer []byte
	config BufferedLoggerConfig
	mu     sync.Mutex
	stopCh chan struct{}
}

type BufferedLoggerConfig struct {
	BufferSize    int
	FlushInterval time.Duration
	FlushOnFull   bool
}

func NewBufferedLogger(output io.Writer, config BufferedLoggerConfig) *BufferedLogger {
	if config.BufferSize == 0 {
		config.BufferSize = 8192
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = time.Second
	}

	bl := &BufferedLogger{
		output: output,
		buffer: make([]byte, 0, config.BufferSize),
		config: config,
		stopCh: make(chan struct{}),
	}

	go bl.flushLoop()
	return bl
}

func (bl *BufferedLogger) Write(p []byte) (int, error) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.buffer = append(bl.buffer, p...)

	if bl.config.FlushOnFull && len(bl.buffer) >= bl.config.BufferSize {
		return bl.flush()
	}

	return len(p), nil
}

func (bl *BufferedLogger) flush() (int, error) {
	n := len(bl.buffer)
	if n == 0 {
		return 0, nil
	}

	_, err := bl.output.Write(bl.buffer)
	bl.buffer = bl.buffer[:0]
	return n, err
}

func (bl *BufferedLogger) flushLoop() {
	ticker := time.NewTicker(bl.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bl.mu.Lock()
			bl.flush()
			bl.mu.Unlock()
		case <-bl.stopCh:
			return
		}
	}
}

func (bl *BufferedLogger) Flush() error {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	_, err := bl.flush()
	return err
}

func (bl *BufferedLogger) Close() error {
	close(bl.stopCh)
	return bl.Flush()
}

type TeeWriter struct {
	writers []io.Writer
	mu      sync.RWMutex
}

func NewTeeWriter(writers ...io.Writer) *TeeWriter {
	return &TeeWriter{writers: writers}
}

func (tw *TeeWriter) Add(w io.Writer) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.writers = append(tw.writers, w)
}

func (tw *TeeWriter) Write(p []byte) (int, error) {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	var lastErr error
	for _, w := range tw.writers {
		if _, err := w.Write(p); err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return len(p), nil
}

type LevelFilter struct {
	next     io.Writer
	minLevel Level
	maxLevel Level
	mu       sync.RWMutex
}

func NewLevelFilter(next io.Writer, minLevel, maxLevel Level) *LevelFilter {
	return &LevelFilter{
		next:     next,
		minLevel: minLevel,
		maxLevel: maxLevel,
	}
}

func (lf *LevelFilter) Write(p []byte) (int, error) {
	lf.mu.RLock()
	defer lf.mu.RUnlock()
	return lf.next.Write(p)
}

func (lf *LevelFilter) SetRange(min, max Level) {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	lf.minLevel = min
	lf.maxLevel = max
}

type RateLimitedWriter struct {
	next      io.Writer
	limit     time.Duration
	lastWrite time.Time
	mu        sync.Mutex
}

func NewRateLimitedWriter(next io.Writer, limit time.Duration) *RateLimitedWriter {
	return &RateLimitedWriter{
		next:  next,
		limit: limit,
	}
}

func (rlw *RateLimitedWriter) Write(p []byte) (int, error) {
	rlw.mu.Lock()
	defer rlw.mu.Unlock()

	now := time.Now()
	if now.Sub(rlw.lastWrite) < rlw.limit {
		return len(p), nil
	}

	rlw.lastWrite = now
	return rlw.next.Write(p)
}

type DedupeWriter struct {
	next   io.Writer
	seen   map[string]time.Time
	window time.Duration
	mu     sync.Mutex
}

func NewDedupeWriter(next io.Writer, window time.Duration) *DedupeWriter {
	return &DedupeWriter{
		next:   next,
		seen:   make(map[string]time.Time),
		window: window,
	}
}

func (dw *DedupeWriter) Write(p []byte) (int, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	key := string(p)
	now := time.Now()

	if lastSeen, exists := dw.seen[key]; exists {
		if now.Sub(lastSeen) < dw.window {
			return len(p), nil
		}
	}

	dw.seen[key] = now
	return dw.next.Write(p)
}

func (dw *DedupeWriter) Cleanup() {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	now := time.Now()
	for key, lastSeen := range dw.seen {
		if now.Sub(lastSeen) > dw.window {
			delete(dw.seen, key)
		}
	}
}

type PrefixWriter struct {
	next   io.Writer
	prefix []byte
	mu     sync.RWMutex
}

func NewPrefixWriter(next io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		next:   next,
		prefix: []byte(prefix),
	}
}

func (pw *PrefixWriter) Write(p []byte) (int, error) {
	pw.mu.RLock()
	defer pw.mu.RUnlock()

	_, err := pw.next.Write(pw.prefix)
	if err != nil {
		return 0, err
	}
	return pw.next.Write(p)
}

func (pw *PrefixWriter) SetPrefix(prefix string) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.prefix = []byte(prefix)
}

type CountingWriter struct {
	next       io.Writer
	totalBytes int64
	totalCalls int64
	mu         sync.RWMutex
}

func NewCountingWriter(next io.Writer) *CountingWriter {
	return &CountingWriter{next: next}
}

func (cw *CountingWriter) Write(p []byte) (int, error) {
	n, err := cw.next.Write(p)
	cw.mu.Lock()
	cw.totalBytes += int64(n)
	cw.totalCalls++
	cw.mu.Unlock()
	return n, err
}

func (cw *CountingWriter) Stats() (int64, int64) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.totalBytes, cw.totalCalls
}

type FanoutWriter struct {
	writers  []io.Writer
	strategy string
	idx      int
	mu       sync.RWMutex
}

func NewFanoutWriter(writers ...io.Writer) *FanoutWriter {
	return &FanoutWriter{
		writers:  writers,
		strategy: "broadcast",
	}
}

func (fw *FanoutWriter) SetStrategy(strategy string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.strategy = strategy
}

func (fw *FanoutWriter) Write(p []byte) (int, error) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	switch fw.strategy {
	case "round-robin":
		return fw.writeRoundRobin(p)
	default:
		return fw.writeBroadcast(p)
	}
}

func (fw *FanoutWriter) writeRoundRobin(p []byte) (int, error) {
	if len(fw.writers) == 0 {
		return 0, nil
	}
	w := fw.writers[fw.idx%len(fw.writers)]
	fw.idx++
	return w.Write(p)
}

func (fw *FanoutWriter) writeBroadcast(p []byte) (int, error) {
	for _, w := range fw.writers {
		if _, err := w.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (fw *FanoutWriter) AddWriter(w io.Writer) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.writers = append(fw.writers, w)
}

func (fw *FanoutWriter) RemoveWriter(w io.Writer) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for i, writer := range fw.writers {
		if writer == w {
			fw.writers = append(fw.writers[:i], fw.writers[i+1:]...)
			return
		}
	}
}

type AsyncWriter struct {
	next   io.Writer
	ch     chan []byte
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

type AsyncWriterConfig struct {
	BufferSize int
	Workers    int
}

func NewAsyncWriter(next io.Writer, config AsyncWriterConfig) *AsyncWriter {
	if config.BufferSize == 0 {
		config.BufferSize = 1000
	}

	aw := &AsyncWriter{
		next: next,
		ch:   make(chan []byte, config.BufferSize),
		done: make(chan struct{}),
	}

	go aw.processLoop()
	return aw
}

func (aw *AsyncWriter) Write(p []byte) (int, error) {
	aw.mu.Lock()
	if aw.closed {
		aw.mu.Unlock()
		return 0, fmt.Errorf("writer closed")
	}
	aw.mu.Unlock()

	data := make([]byte, len(p))
	copy(data, p)

	select {
	case aw.ch <- data:
		return len(p), nil
	default:
		return len(p), nil
	}
}

func (aw *AsyncWriter) processLoop() {
	for data := range aw.ch {
		aw.next.Write(data)
	}
	close(aw.done)
}

func (aw *AsyncWriter) Close() error {
	aw.mu.Lock()
	if aw.closed {
		aw.mu.Unlock()
		return nil
	}
	aw.closed = true
	aw.mu.Unlock()

	close(aw.ch)
	<-aw.done
	return nil
}

type ConditionalWriter struct {
	next      io.Writer
	condition func([]byte) bool
	mu        sync.RWMutex
}

func NewConditionalWriter(next io.Writer, condition func([]byte) bool) *ConditionalWriter {
	return &ConditionalWriter{
		next:      next,
		condition: condition,
	}
}

func (cw *ConditionalWriter) Write(p []byte) (int, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if cw.condition(p) {
		return cw.next.Write(p)
	}
	return len(p), nil
}

func (cw *ConditionalWriter) SetCondition(fn func([]byte) bool) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.condition = fn
}

type SyncWriter struct {
	next    io.Writer
	buffer  []byte
	flushCh chan struct{}
	done    chan struct{}
	mu      sync.Mutex
}

func NewSyncWriter(next io.Writer) *SyncWriter {
	sw := &SyncWriter{
		next:    next,
		flushCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	go sw.syncLoop()
	return sw
}

func (sw *SyncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	sw.buffer = append(sw.buffer, p...)
	sw.mu.Unlock()

	select {
	case sw.flushCh <- struct{}{}:
	default:
	}

	return len(p), nil
}

func (sw *SyncWriter) syncLoop() {
	defer close(sw.done)
	for range sw.flushCh {
		sw.mu.Lock()
		if len(sw.buffer) > 0 {
			sw.next.Write(sw.buffer)
			sw.buffer = sw.buffer[:0]
		}
		sw.mu.Unlock()
	}
}

func (sw *SyncWriter) Flush() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if len(sw.buffer) > 0 {
		_, err := sw.next.Write(sw.buffer)
		sw.buffer = sw.buffer[:0]
		return err
	}
	return nil
}

func (sw *SyncWriter) Close() error {
	close(sw.flushCh)
	<-sw.done
	return sw.Flush()
}

type LoggerMiddleware struct {
	logger *Logger
	config LoggerMiddlewareConfig
	mu     sync.RWMutex
}

type LoggerMiddlewareConfig struct {
	LogRequests  bool
	LogResponses bool
	LogHeaders   bool
	LogBody      bool
	SkipPaths    []string
	Level        Level
}

func NewLoggerMiddleware(logger *Logger, config LoggerMiddlewareConfig) *LoggerMiddleware {
	if config.Level == 0 {
		config.Level = LevelInfo
	}
	return &LoggerMiddleware{
		logger: logger,
		config: config,
	}
}

func (lm *LoggerMiddleware) ShouldSkip(path string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	for _, skip := range lm.config.SkipPaths {
		if skip == path {
			return true
		}
	}
	return false
}

func (lm *LoggerMiddleware) Log(level Level, msg string, fields ...Field) {
	lm.logger.log(level, msg, fields...)
}

func createLogFile(dir, prefix string) (io.Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	filename := filepath.Join(dir, fmt.Sprintf("%s.log", prefix))
	return os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}
