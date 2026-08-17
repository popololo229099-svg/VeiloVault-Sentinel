package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Message struct {
	ID          string
	Topic       string
	Key         string
	Value       []byte
	Headers     map[string]string
	Timestamp   time.Time
	Partition   int32
	Offset      int64
	RetryCount  int
	MaxRetries  int
	Deadline    time.Time
}

type MessageHandler func(ctx context.Context, msg *Message) error

type MessageBus interface {
	Publish(ctx context.Context, msg *Message) error
	Subscribe(topic string, handler MessageHandler) error
	Close() error
}

type InMemoryMessageBus struct {
	subscriptions map[string][]MessageHandler
	buffer        chan *Message
	mu            sync.RWMutex
	closed        bool
}

func NewInMemoryMessageBus(bufferSize int) *InMemoryMessageBus {
	if bufferSize <= 0 {
		bufferSize = 10000
	}
	return &InMemoryMessageBus{
		subscriptions: make(map[string][]MessageHandler),
		buffer:        make(chan *Message, bufferSize),
	}
}

func (b *InMemoryMessageBus) Publish(ctx context.Context, msg *Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return fmt.Errorf("message bus is closed")
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	handlers := b.subscriptions[msg.Topic]
	for _, handler := range handlers {
		go func(h MessageHandler) {
			_ = h(ctx, msg)
		}(handler)
	}
	return nil
}

func (b *InMemoryMessageBus) Subscribe(topic string, handler MessageHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscriptions[topic] = append(b.subscriptions[topic], handler)
	return nil
}

func (b *InMemoryMessageBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	close(b.buffer)
	return nil
}

type DeadLetterQueue struct {
	messages []*Message
	maxSize  int
	mu       sync.RWMutex
}

func NewDeadLetterQueue(maxSize int) *DeadLetterQueue {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &DeadLetterQueue{
		messages: make([]*Message, 0),
		maxSize:  maxSize,
	}
}

func (dlq *DeadLetterQueue) Add(msg *Message) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	if len(dlq.messages) >= dlq.maxSize {
		dlq.messages = dlq.messages[1:]
	}
	msg.RetryCount = 0
	dlq.messages = append(dlq.messages, msg)
}

func (dlq *DeadLetterQueue) Get() (*Message, bool) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	if len(dlq.messages) == 0 {
		return nil, false
	}
	msg := dlq.messages[0]
	dlq.messages = dlq.messages[1:]
	return msg, true
}

func (dlq *DeadLetterQueue) Size() int {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return len(dlq.messages)
}

func (dlq *DeadLetterQueue) Peek() (*Message, bool) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	if len(dlq.messages) == 0 {
		return nil, false
	}
	return dlq.messages[0], true
}

func (dlq *DeadLetterQueue) Drain(n int) []*Message {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	if n > len(dlq.messages) {
		n = len(dlq.messages)
	}
	result := make([]*Message, n)
	copy(result, dlq.messages[:n])
	dlq.messages = dlq.messages[n:]
	return result
}

type RetryPolicy struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      bool
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}
}

func (rp RetryPolicy) Delay(attempt int) time.Duration {
	delay := float64(rp.BaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= rp.Multiplier
	}
	if delay > float64(rp.MaxDelay) {
		delay = float64(rp.MaxDelay)
	}
	d := time.Duration(delay)
	if rp.Jitter {
		d = time.Duration(float64(d) * (0.5 + 0.5*float64(time.Now().UnixNano()%100)/100.0))
	}
	return d
}

type MessageProcessor struct {
	bus       MessageBus
	dlq       *DeadLetterQueue
	retry     RetryPolicy
	handlers  map[string]MessageHandler
	mu        sync.RWMutex
	workerCh  chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewMessageProcessor(bus MessageBus, retry RetryPolicy, workers int) *MessageProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	if workers <= 0 {
		workers = 10
	}
	return &MessageProcessor{
		bus:      bus,
		dlq:      NewDeadLetterQueue(10000),
		retry:    retry,
		handlers: make(map[string]MessageHandler),
		workerCh: make(chan struct{}, workers),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (p *MessageProcessor) RegisterHandler(topic string, handler MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topic] = handler
	_ = p.bus.Subscribe(topic, p.wrapHandler(topic, handler))
}

func (p *MessageProcessor) wrapHandler(topic string, handler MessageHandler) MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		return p.processWithRetry(ctx, msg, handler)
	}
}

func (p *MessageProcessor) processWithRetry(ctx context.Context, msg *Message, handler MessageHandler) error {
	var lastErr error
	for attempt := 0; attempt <= p.retry.MaxRetries; attempt++ {
		msg.RetryCount = attempt
		lastErr = handler(ctx, msg)
		if lastErr == nil {
			return nil
		}
		if attempt < p.retry.MaxRetries {
			delay := p.retry.Delay(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	p.dlq.Add(msg)
	return fmt.Errorf("message %s failed after %d retries: %w", msg.ID, p.retry.MaxRetries, lastErr)
}

func (p *MessageProcessor) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *MessageProcessor) DLQSize() int {
	return p.dlq.Size()
}

type MessageInterceptor func(ctx context.Context, msg *Message) (*Message, error)

type InterceptorMessageBus struct {
	inner        MessageBus
	interceptors []MessageInterceptor
	mu           sync.RWMutex
}

func NewInterceptorMessageBus(inner MessageBus) *InterceptorMessageBus {
	return &InterceptorMessageBus{inner: inner}
}

func (b *InterceptorMessageBus) AddInterceptor(interceptor MessageInterceptor) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.interceptors = append(b.interceptors, interceptor)
}

func (b *InterceptorMessageBus) Publish(ctx context.Context, msg *Message) error {
	b.mu.RLock()
	interceptors := make([]MessageInterceptor, len(b.interceptors))
	copy(interceptors, b.interceptors)
	b.mu.RUnlock()

	for _, interceptor := range interceptors {
		processed, err := interceptor(ctx, msg)
		if err != nil {
			return fmt.Errorf("interceptor error: %w", err)
		}
		msg = processed
	}
	return b.inner.Publish(ctx, msg)
}

func (b *InterceptorMessageBus) Subscribe(topic string, handler MessageHandler) error {
	return b.inner.Subscribe(topic, handler)
}

func (b *InterceptorMessageBus) Close() error {
	return b.inner.Close()
}

type MessageFilter func(msg *Message) bool

type FilteredMessageBus struct {
	inner   MessageBus
	filters map[string][]MessageFilter
	mu      sync.RWMutex
}

func NewFilteredMessageBus(inner MessageBus) *FilteredMessageBus {
	return &FilteredMessageBus{
		inner:   inner,
		filters: make(map[string][]MessageFilter),
	}
}

func (f *FilteredMessageBus) AddFilter(topic string, filter MessageFilter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters[topic] = append(f.filters[topic], filter)
}

func (f *FilteredMessageBus) Publish(ctx context.Context, msg *Message) error {
	f.mu.RLock()
	filters := f.filters[msg.Topic]
	f.mu.RUnlock()

	for _, filter := range filters {
		if !filter(msg) {
			return nil
		}
	}
	return f.inner.Publish(ctx, msg)
}

func (f *FilteredMessageBus) Subscribe(topic string, handler MessageHandler) error {
	return f.inner.Subscribe(topic, handler)
}

func (f *FilteredMessageBus) Close() error {
	return f.inner.Close()
}

type MessageBatch struct {
	Messages []*Message
	BatchID  string
	Created  time.Time
}

type BatchProducer struct {
	bus       MessageBus
	batchSize int
	flushTime time.Duration
	pending   []*Message
	mu        sync.Mutex
}

func NewBatchProducer(bus MessageBus, batchSize int, flushTime time.Duration) *BatchProducer {
	return &BatchProducer{
		bus:       bus,
		batchSize: batchSize,
		flushTime: flushTime,
		pending:   make([]*Message, 0),
	}
}

func (bp *BatchProducer) Add(msg *Message) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.pending = append(bp.pending, msg)
	if len(bp.pending) >= bp.batchSize {
		return bp.flush()
	}
	return nil
}

func (bp *BatchProducer) Flush() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.flush()
}

func (bp *BatchProducer) flush() error {
	if len(bp.pending) == 0 {
		return nil
	}
	for _, msg := range bp.pending {
		if err := bp.bus.Publish(context.Background(), msg); err != nil {
			return err
		}
	}
	bp.pending = bp.pending[:0]
	return nil
}

type MessageStore interface {
	Store(ctx context.Context, msg *Message) error
	Get(ctx context.Context, id string) (*Message, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, topic string, since time.Time) ([]*Message, error)
}

type InMemoryMessageStore struct {
	messages map[string]*Message
	mu       sync.RWMutex
}

func NewInMemoryMessageStore() *InMemoryMessageStore {
	return &InMemoryMessageStore{messages: make(map[string]*Message)}
}

func (s *InMemoryMessageStore) Store(ctx context.Context, msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[msg.ID] = msg
	return nil
}

func (s *InMemoryMessageStore) Get(ctx context.Context, id string) (*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msg, ok := s.messages[id]
	if !ok {
		return nil, fmt.Errorf("message %s not found", id)
	}
	return msg, nil
}

func (s *InMemoryMessageStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, id)
	return nil
}

func (s *InMemoryMessageStore) List(ctx context.Context, topic string, since time.Time) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Message
	for _, msg := range s.messages {
		if msg.Topic == topic && msg.Timestamp.After(since) {
			result = append(result, msg)
		}
	}
	return result, nil
}

type OutboxEntry struct {
	ID           string
	AggregateID  string
	AggregateType string
	EventType    string
	Payload      []byte
	CreatedAt    time.Time
	PublishedAt  *time.Time
	Status       string
}

type OutboxRepository interface {
	Save(ctx context.Context, entry *OutboxEntry) error
	GetUnpublished(ctx context.Context, limit int) ([]*OutboxEntry, error)
	MarkPublished(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type InMemoryOutboxRepository struct {
	entries map[string]*OutboxEntry
	mu      sync.RWMutex
}

func NewInMemoryOutboxRepository() *InMemoryOutboxRepository {
	return &InMemoryOutboxRepository{entries: make(map[string]*OutboxEntry)}
}

func (r *InMemoryOutboxRepository) Save(ctx context.Context, entry *OutboxEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.CreatedAt = time.Now()
	entry.Status = "pending"
	r.entries[entry.ID] = entry
	return nil
}

func (r *InMemoryOutboxRepository) GetUnpublished(ctx context.Context, limit int) ([]*OutboxEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*OutboxEntry
	for _, e := range r.entries {
		if e.Status == "pending" {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *InMemoryOutboxRepository) MarkPublished(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[id]; ok {
		entry.Status = "published"
		now := time.Now()
		entry.PublishedAt = &now
	}
	return nil
}

func (r *InMemoryOutboxRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
	return nil
}

type OutboxPublisher struct {
	repo      OutboxRepository
	bus       MessageBus
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewOutboxPublisher(repo OutboxRepository, bus MessageBus, interval time.Duration) *OutboxPublisher {
	ctx, cancel := context.WithCancel(context.Background())
	return &OutboxPublisher{
		repo:     repo,
		bus:      bus,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (p *OutboxPublisher) Start() {
	p.wg.Add(1)
	go p.poll()
}

func (p *OutboxPublisher) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *OutboxPublisher) poll() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.publishPending()
		}
	}
}

func (p *OutboxPublisher) publishPending() {
	entries, _ := p.repo.GetUnpublished(p.ctx, 100)
	for _, entry := range entries {
		msg := &Message{
			ID:        entry.ID,
			Topic:     entry.EventType,
			Key:       entry.AggregateID,
			Value:     entry.Payload,
			Timestamp: time.Now(),
		}
		if err := p.bus.Publish(p.ctx, msg); err == nil {
			_ = p.repo.MarkPublished(p.ctx, entry.ID)
		}
	}
}

type MessageValidator func(msg *Message) error

type ValidatedMessageBus struct {
	inner      MessageBus
	validators map[string][]MessageValidator
	mu         sync.RWMutex
}

func NewValidatedMessageBus(inner MessageBus) *ValidatedMessageBus {
	return &ValidatedMessageBus{
		inner:      inner,
		validators: make(map[string][]MessageValidator),
	}
}

func (v *ValidatedMessageBus) AddValidator(topic string, validator MessageValidator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.validators[topic] = append(v.validators[topic], validator)
}

func (v *ValidatedMessageBus) Publish(ctx context.Context, msg *Message) error {
	v.mu.RLock()
	validators := v.validators[msg.Topic]
	v.mu.RUnlock()

	for _, validator := range validators {
		if err := validator(msg); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}
	return v.inner.Publish(ctx, msg)
}

func (v *ValidatedMessageBus) Subscribe(topic string, handler MessageHandler) error {
	return v.inner.Subscribe(topic, handler)
}

func (v *ValidatedMessageBus) Close() error {
	return v.inner.Close()
}

type MessageRouter struct {
	routes map[string]MessageHandler
	mu     sync.RWMutex
}

func NewMessageRouter() *MessageRouter {
	return &MessageRouter{routes: make(map[string]MessageHandler)}
}

func (r *MessageRouter) Route(pattern string, handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[pattern] = handler
}

func (r *MessageRouter) Handle(ctx context.Context, msg *Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.routes[msg.Topic]
	if !ok {
		return fmt.Errorf("no handler for topic %s", msg.Topic)
	}
	return handler(ctx, msg)
}

func (r *MessageRouter) HasRoute(topic string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.routes[topic]
	return ok
}

type MessageTransformer func(msg *Message) (*Message, error)

type TransformPipeline struct {
	transformers []MessageTransformer
}

func NewTransformPipeline() *TransformPipeline {
	return &TransformPipeline{transformers: make([]MessageTransformer, 0)}
}

func (tp *TransformPipeline) Add(transformer MessageTransformer) *TransformPipeline {
	tp.transformers = append(tp.transformers, transformer)
	return tp
}

func (tp *TransformPipeline) Transform(msg *Message) (*Message, error) {
	var err error
	for _, t := range tp.transformers {
		msg, err = t(msg)
		if err != nil {
			return nil, err
		}
	}
	return msg, nil
}

type TopicPartitioner struct {
	numPartitions int32
}

func NewTopicPartitioner(numPartitions int32) *TopicPartitioner {
	if numPartitions <= 0 {
		numPartitions = 1
	}
	return &TopicPartitioner{numPartitions: numPartitions}
}

func (tp *TopicPartitioner) Partition(key string) int32 {
	if key == "" {
		return 0
	}
	hash := uint32(0)
	for i := 0; i < len(key); i++ {
		hash = hash*31 + uint32(key[i])
	}
	return int32(hash % uint32(tp.numPartitions))
}

type MessageTracer struct {
	spans map[string]*Span
	mu    sync.RWMutex
}

type Span struct {
	ID        string
	ParentID  string
	TraceID   string
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Tags      map[string]string
}

func NewMessageTracer() *MessageTracer {
	return &MessageTracer{spans: make(map[string]*Span)}
}

func (mt *MessageTracer) StartSpan(id, parentID, traceID, name string) *Span {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	span := &Span{
		ID:        id,
		ParentID:  parentID,
		TraceID:   traceID,
		Name:      name,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
	}
	mt.spans[id] = span
	return span
}

func (mt *MessageTracer) EndSpan(id string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if span, ok := mt.spans[id]; ok {
		span.EndTime = time.Now()
	}
}

func (mt *MessageTracer) GetSpan(id string) (*Span, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	span, ok := mt.spans[id]
	return span, ok
}

type CircuitBreakerMessages struct {
	bus          MessageBus
	state        int
	failCount    int64
	successCount int64
	threshold    int64
	resetTimeout time.Duration
	lastFailure  time.Time
	mu           sync.Mutex
}

func NewCircuitBreakerMessages(bus MessageBus, threshold int64, resetTimeout time.Duration) *CircuitBreakerMessages {
	return &CircuitBreakerMessages{
		bus:          bus,
		state:        0,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreakerMessages) Publish(ctx context.Context, msg *Message) error {
	cb.mu.Lock()
	if cb.state == 1 {
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = 2
		} else {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker is open")
		}
	}
	cb.mu.Unlock()

	err := cb.bus.Publish(ctx, msg)
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err != nil {
		cb.failCount++
		cb.lastFailure = time.Now()
		if cb.failCount >= cb.threshold {
			cb.state = 1
		}
		return err
	}
	if cb.state == 2 {
		cb.successCount++
		if cb.successCount >= cb.threshold {
			cb.state = 0
			cb.failCount = 0
			cb.successCount = 0
		}
	} else {
		cb.failCount = 0
	}
	return nil
}

func (cb *CircuitBreakerMessages) Subscribe(topic string, handler MessageHandler) error {
	return cb.bus.Subscribe(topic, handler)
}

func (cb *CircuitBreakerMessages) Close() error {
	return cb.bus.Close()
}

type PriorityMessageQueue struct {
	queues  map[int][]*Message
	mu      sync.Mutex
}

func NewPriorityMessageQueue() *PriorityMessageQueue {
	return &PriorityMessageQueue{
		queues: make(map[int][]*Message),
	}
}

func (pq *PriorityMessageQueue) Enqueue(msg *Message, priority int) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.queues[priority] = append(pq.queues[priority], msg)
}

func (pq *PriorityMessageQueue) Dequeue() (*Message, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	for priority := 9; priority >= 0; priority-- {
		if msgs, ok := pq.queues[priority]; ok && len(msgs) > 0 {
			msg := msgs[0]
			pq.queues[priority] = msgs[1:]
			return msg, true
		}
	}
	return nil, false
}

func (pq *PriorityMessageQueue) Size() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	total := 0
	for _, msgs := range pq.queues {
		total += len(msgs)
	}
	return total
}
