package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelPanic
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	case LevelPanic:
		return "panic"
	default:
		return "unknown"
	}
}

func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	case "panic":
		return LevelPanic
	default:
		return LevelInfo
	}
}

type Field struct {
	Key   string
	Value interface{}
}

type Entry struct {
	Level   Level
	Message string
	Fields  []Field
	Time    time.Time
	Caller  string
	context context.Context
	mu      sync.RWMutex
}

func (e *Entry) SetField(key string, value interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, f := range e.Fields {
		if f.Key == key {
			e.Fields[i] = Field{Key: key, Value: value}
			return
		}
	}
	e.Fields = append(e.Fields, Field{Key: key, Value: value})
}

func (e *Entry) GetField(key string) (interface{}, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, f := range e.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

func (e *Entry) WithContext(ctx context.Context) *Entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.context = ctx
	return e
}

func (e *Entry) WithFields(fields ...Field) *Entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Fields = append(e.Fields, fields...)
	return e
}

type Formatter interface {
	Format(entry *Entry) ([]byte, error)
}

type JSONFormatter struct {
	PrettyPrint bool
	mu          sync.RWMutex
}

func NewJSONFormatter(prettyPrint bool) *JSONFormatter {
	return &JSONFormatter{PrettyPrint: prettyPrint}
}

func (jf *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	entry.mu.RLock()
	defer entry.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("{")

	sb.WriteString(fmt.Sprintf(`"time":"%s"`, entry.Time.Format(time.RFC3339Nano)))
	sb.WriteString(fmt.Sprintf(`,"level":"%s"`, entry.Level.String()))
	sb.WriteString(fmt.Sprintf(`,"message":%q`, entry.Message))

	if entry.Caller != "" {
		sb.WriteString(fmt.Sprintf(`,"caller":%q`, entry.Caller))
	}

	if len(entry.Fields) > 0 {
		sb.WriteString(`,"fields":{`)
		for i, f := range entry.Fields {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`%q:%v`, f.Key, f.Value))
		}
		sb.WriteString("}")
	}

	sb.WriteString("}")
	return []byte(sb.String() + "\n"), nil
}

type ConsoleFormatter struct {
	ColorsEnabled bool
	Pattern       string
	mu            sync.RWMutex
}

func NewConsoleFormatter(colors bool) *ConsoleFormatter {
	return &ConsoleFormatter{
		ColorsEnabled: colors,
		Pattern:       "%s [%s] %s",
	}
}

func (cf *ConsoleFormatter) Format(entry *Entry) ([]byte, error) {
	entry.mu.RLock()
	defer entry.mu.RUnlock()

	timeStr := entry.Time.Format("15:04:05.000")
	levelStr := strings.ToUpper(entry.Level.String())

	if cf.ColorsEnabled {
		levelStr = colorizeLevel(entry.Level)
	}

	msg := fmt.Sprintf("%s [%-5s] %s", timeStr, levelStr, entry.Message)

	if len(entry.Fields) > 0 {
		fieldStrs := make([]string, 0, len(entry.Fields))
		for _, f := range entry.Fields {
			fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", f.Key, f.Value))
		}
		msg += " | " + strings.Join(fieldStrs, " ")
	}

	return []byte(msg + "\n"), nil
}

func colorizeLevel(level Level) string {
	switch level {
	case LevelDebug:
		return "\033[36mDEBUG\033[0m"
	case LevelInfo:
		return "\033[32mINFO \033[0m"
	case LevelWarn:
		return "\033[33mWARN \033[0m"
	case LevelError:
		return "\033[31mERROR\033[0m"
	case LevelFatal:
		return "\033[35mFATAL\033[0m"
	case LevelPanic:
		return "\033[31mPANIC\033[0m"
	default:
		return "?????"
	}
}

type Logger struct {
	level      Level
	outputs    []io.Writer
	formatter  Formatter
	hooks      []func(*Entry)
	mu         sync.RWMutex
	sampleRate map[string]float64
}

type LoggerConfig struct {
	Level     Level
	Outputs   []io.Writer
	Formatter Formatter
	Hooks     []func(*Entry)
}

func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:     LevelInfo,
		Outputs:   []io.Writer{os.Stdout},
		Formatter: &ConsoleFormatter{ColorsEnabled: true},
	}
}

func New(config ...LoggerConfig) *Logger {
	cfg := DefaultLoggerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &Logger{
		level:      cfg.Level,
		outputs:    cfg.Outputs,
		formatter:  cfg.Formatter,
		hooks:      cfg.Hooks,
		sampleRate: make(map[string]float64),
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

func (l *Logger) SetFormatter(formatter Formatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = formatter
}

func (l *Logger) AddOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, w)
}

func (l *Logger) AddHook(hook func(*Entry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

func (l *Logger) SetSampleRate(key string, rate float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sampleRate[key] = rate
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	l.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

func (l *Logger) Panic(msg string, fields ...Field) {
	l.log(LevelPanic, msg, fields...)
	panic(msg)
}

func (l *Logger) log(level Level, msg string, fields ...Field) {
	l.mu.RLock()
	if level < l.level {
		l.mu.RUnlock()
		return
	}

	formatter := l.formatter
	outputs := make([]io.Writer, len(l.outputs))
	copy(outputs, l.outputs)
	hooks := make([]func(*Entry), len(l.hooks))
	copy(hooks, l.hooks)
	l.mu.RUnlock()

	entry := &Entry{
		Level:   level,
		Message: msg,
		Fields:  fields,
		Time:    time.Now(),
	}

	for _, hook := range hooks {
		hook(entry)
	}

	data, err := formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format log entry: %v\n", err)
		return
	}

	for _, w := range outputs {
		w.Write(data)
	}
}

func (l *Logger) WithFields(fields ...Field) *LoggerEntry {
	return &LoggerEntry{
		logger: l,
		fields: fields,
	}
}

type LoggerEntry struct {
	logger *Logger
	fields []Field
}

func (le *LoggerEntry) Debug(msg string) {
	le.logger.log(LevelDebug, msg, le.fields...)
}

func (le *LoggerEntry) Info(msg string) {
	le.logger.log(LevelInfo, msg, le.fields...)
}

func (le *LoggerEntry) Warn(msg string) {
	le.logger.log(LevelWarn, msg, le.fields...)
}

func (le *LoggerEntry) Error(msg string) {
	le.logger.log(LevelError, msg, le.fields...)
}

type MultiWriter struct {
	writers []io.Writer
	mu      sync.RWMutex
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) Add(w io.Writer) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.writers = append(mw.writers, w)
}

func (mw *MultiWriter) Remove(w io.Writer) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	for i, writer := range mw.writers {
		if writer == w {
			mw.writers = append(mw.writers[:i], mw.writers[i+1:]...)
			return
		}
	}
}

func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	mw.mu.RLock()
	defer mw.mu.RUnlock()

	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

type LogRotater struct {
	dir        string
	prefix     string
	maxSize    int64
	maxFiles   int
	current    *os.File
	size       int64
	mu         sync.Mutex
}

type LogRotationConfig struct {
	Directory string
	Prefix    string
	MaxSize   int64
	MaxFiles  int
}

func NewLogRotater(config LogRotationConfig) (*LogRotater, error) {
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxFiles == 0 {
		config.MaxFiles = 5
	}
	if config.Directory == "" {
		config.Directory = "."
	}

	lr := &LogRotater{
		dir:      config.Directory,
		prefix:   config.Prefix,
		maxSize:  config.MaxSize,
		maxFiles: config.MaxFiles,
	}

	if err := lr.openFile(); err != nil {
		return nil, err
	}

	return lr, nil
}

func (lr *LogRotater) openFile() error {
	filename := filepath.Join(lr.dir, fmt.Sprintf("%s.log", lr.prefix))
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	info, err := f.Stat()
	if err == nil {
		lr.size = info.Size()
	}

	lr.current = f
	return nil
}

func (lr *LogRotater) Write(p []byte) (int, error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.size+int64(len(p)) > lr.maxSize {
		if err := lr.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := lr.current.Write(p)
	lr.size += int64(n)
	return n, err
}

func (lr *LogRotater) rotate() error {
	if lr.current != nil {
		lr.current.Close()
	}

	for i := lr.maxFiles - 1; i >= 1; i-- {
		src := filepath.Join(lr.dir, fmt.Sprintf("%s.%d.log", lr.prefix, i))
		dst := filepath.Join(lr.dir, fmt.Sprintf("%s.%d.log", lr.prefix, i+1))
		os.Rename(src, dst)
	}

	src := filepath.Join(lr.dir, fmt.Sprintf("%s.log", lr.prefix))
	dst := filepath.Join(lr.dir, fmt.Sprintf("%s.1.log", lr.prefix))
	os.Rename(src, dst)

	return lr.openFile()
}

func (lr *LogRotater) Close() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if lr.current != nil {
		return lr.current.Close()
	}
	return nil
}

type ContextLogger struct {
	logger *Logger
	mu     sync.RWMutex
}

func NewContextLogger(logger *Logger) *ContextLogger {
	return &ContextLogger{logger: logger}
}

type logContextKey string

const loggerKey logContextKey = "logger"

func (cl *ContextLogger) FromContext(ctx context.Context) *Logger {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		return l
	}
	return cl.logger
}

func (cl *ContextLogger) WithContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

type FieldExtractor struct {
	prefixes []string
	mu       sync.RWMutex
}

func NewFieldExtractor(prefixes ...string) *FieldExtractor {
	return &FieldExtractor{prefixes: prefixes}
}

func (fe *FieldExtractor) Extract(fields []Field) map[string]interface{} {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	result := make(map[string]interface{})
	for _, f := range fields {
		for _, prefix := range fe.prefixes {
			if strings.HasPrefix(f.Key, prefix) {
				result[f.Key] = f.Value
				break
			}
		}
	}
	return result
}

func (fe *FieldExtractor) Filter(fields []Field, excludeKeys ...string) []Field {
	excludeSet := make(map[string]bool)
	for _, key := range excludeKeys {
		excludeSet[key] = true
	}

	result := make([]Field, 0, len(fields))
	for _, f := range fields {
		if !excludeSet[f.Key] {
			result = append(result, f)
		}
	}
	return result
}

type Sampler struct {
	rate     float64
	counters map[string]int64
	mu       sync.RWMutex
}

func NewSampler(rate float64) *Sampler {
	if rate <= 0 || rate > 1 {
		rate = 1.0
	}
	return &Sampler{
		rate:     rate,
		counters: make(map[string]int64),
	}
}

func (s *Sampler) ShouldSample(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[key]++
	if s.rate >= 1.0 {
		return true
	}
	return float64(s.counters[key])*s.rate >= float64(s.counters[key]-1)
}

func (s *Sampler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters = make(map[string]int64)
}

type LogAggregator struct {
	entries []*Entry
	mu      sync.RWMutex
	maxSize int
}

func NewLogAggregator(maxSize int) *LogAggregator {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &LogAggregator{
		entries: make([]*Entry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (la *LogAggregator) Add(entry *Entry) {
	la.mu.Lock()
	defer la.mu.Unlock()

	if len(la.entries) >= la.maxSize {
		la.entries = la.entries[1:]
	}
	la.entries = append(la.entries, entry)
}

func (la *LogAggregator) GetByLevel(level Level) []*Entry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	result := make([]*Entry, 0)
	for _, entry := range la.entries {
		if entry.Level == level {
			result = append(result, entry)
		}
	}
	return result
}

func (la *LogAggregator) Search(keyword string) []*Entry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	result := make([]*Entry, 0)
	for _, entry := range la.entries {
		if strings.Contains(entry.Message, keyword) {
			result = append(result, entry)
		}
	}
	return result
}

func (la *LogAggregator) Recent(n int) []*Entry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	if n > len(la.entries) {
		n = len(la.entries)
	}

	result := make([]*Entry, n)
	copy(result, la.entries[len(la.entries)-n:])
	return result
}

func (la *LogAggregator) Clear() {
	la.mu.Lock()
	defer la.mu.Unlock()
	la.entries = la.entries[:0]
}

func (la *LogAggregator) Count() int {
	la.mu.RLock()
	defer la.mu.RUnlock()
	return len(la.entries)
}

func (la *LogAggregator) SortByTime() []*Entry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	result := make([]*Entry, len(la.entries))
	copy(result, la.entries)

	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.Before(result[j].Time)
	})

	return result
}

type FieldBuilder struct {
	fields []Field
	mu     sync.Mutex
}

func NewFieldBuilder() *FieldBuilder {
	return &FieldBuilder{
		fields: make([]Field, 0),
	}
}

func (fb *FieldBuilder) String(key, value string) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.fields = append(fb.fields, Field{Key: key, Value: value})
	return fb
}

func (fb *FieldBuilder) Int(key string, value int) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.fields = append(fb.fields, Field{Key: key, Value: value})
	return fb
}

func (fb *FieldBuilder) Int64(key string, value int64) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.fields = append(fb.fields, Field{Key: key, Value: value})
	return fb
}

func (fb *FieldBuilder) Float64(key string, value float64) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.fields = append(fb.fields, Field{Key: key, Value: value})
	return fb
}

func (fb *FieldBuilder) Bool(key string, value bool) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.fields = append(fb.fields, Field{Key: key, Value: value})
	return fb
}

func (fb *FieldBuilder) Error(key string, err error) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if err != nil {
		fb.fields = append(fb.fields, Field{Key: key, Value: err.Error()})
	}
	return fb
}

func (fb *FieldBuilder) Any(key string, value interface{}) *FieldBuilder {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.fields = append(fb.fields, Field{Key: key, Value: value})
	return fb
}

func (fb *FieldBuilder) Build() []Field {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	result := make([]Field, len(fb.fields))
	copy(result, fb.fields)
	return result
}

func (fb *FieldBuilder) Count() int {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return len(fb.fields)
}
